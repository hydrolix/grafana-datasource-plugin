package exchange

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
)

// RFC 8693.
const (
	grantType       = "urn:ietf:params:oauth:grant-type:token-exchange"
	tokenTypeAccess = "urn:ietf:params:oauth:token-type:access_token"
)

// Exchanger performs one exchange. The interface exists so the token source can
// be tested without a console.
type Exchanger interface {
	// Exchange answers a cluster token and the lifetime the console gave it.
	Exchange(ctx context.Context, audience, subjectToken string) (token string, lifetime time.Duration, err error)
}

// HTTPExchanger calls the console's RFC 8693 facade.
type HTTPExchanger struct {
	Config Config
	Client *http.Client
}

// exchangeTimeout is deliberately shorter than a panel's patience: a slow
// console should read as unavailable rather than hold a query open.
const exchangeTimeout = 10 * time.Second

// NewHTTPExchanger builds an exchanger on the plugin SDK's HTTP client, so the
// one outbound call this mode depends on carries Grafana's tracing, metrics and
// error-source middleware like every other call a plugin makes, and honours the
// standard proxy environment.
func NewHTTPExchanger(cfg Config) *HTTPExchanger {
	timeouts := httpclient.DefaultTimeoutOptions
	timeouts.Timeout = exchangeTimeout
	client, err := httpclient.New(httpclient.Options{
		Timeouts:    &timeouts,
		Middlewares: httpclient.DefaultMiddlewares(),
	})
	if err != nil {
		// Only proxy configuration can fail here, and none is passed; a bare
		// client keeps the mode working rather than unavailable.
		client = &http.Client{Timeout: exchangeTimeout}
	}
	client.CheckRedirect = answerRedirects
	return &HTTPExchanger{Config: cfg, Client: client}
}

// answerRedirects makes a redirect the exchange's answer rather than a hop.
// Following one would resend the form, and with it the sign-in token, to
// wherever it points: a 307 or 308 keeps the body, and http.Client follows a
// downgrade from https to http as readily as any other. The facade answers at
// its own URL, so classify reports the 3xx as the operator's problem.
func answerRedirects(*http.Request, []*http.Request) error {
	return http.ErrUseLastResponse
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

type errorResponse struct {
	Error       string `json:"error"`
	Description string `json:"error_description"`
}

func (h *HTTPExchanger) Exchange(ctx context.Context, audience, subjectToken string) (string, time.Duration, error) {
	cred, ok := h.Config.Credential(audience)
	if !ok {
		// The detail names neither the audience nor the credential as the
		// missing part: see Misconfigured.
		return "", 0, &Error{Outcome: Misconfigured, Detail: "check the exchange URL and credentials in Grafana's configuration"}
	}

	form := url.Values{
		"grant_type":         {grantType},
		"subject_token":      {subjectToken},
		"subject_token_type": {tokenTypeAccess},
		"audience":           {audience},
	}
	// No `scope`: the facade refuses a request that carries one.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.Config.URL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, &Error{Outcome: Unavailable, Detail: "building the request failed"}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(cred.ClientID, cred.ClientSecret)

	resp, err := h.Client.Do(req)
	if err != nil {
		// The error is not wrapped: a transport error can quote the request URL,
		// and nothing downstream needs more than "unreachable".
		return "", 0, ErrUnavailable
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); _ = resp.Body.Close() }()

	// 1 MiB is far more than any token response, and a bound keeps a confused
	// endpoint from becoming a memory problem.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", 0, ErrUnavailable
	}

	if resp.StatusCode != http.StatusOK {
		return "", 0, classify(resp.StatusCode, body)
	}

	var ok200 tokenResponse
	if err := json.Unmarshal(body, &ok200); err != nil || ok200.AccessToken == "" || ok200.ExpiresIn <= 0 {
		// A 200 that is not a token response is the console's wiring, not this
		// person's access.
		return "", 0, &Error{Outcome: Unavailable, Detail: "the exchange answered 200 without a usable token"}
	}
	return ok200.AccessToken, time.Duration(ok200.ExpiresIn) * time.Second, nil
}

// classify maps the facade's closed error vocabulary onto the two outcomes a
// panel can act on. The facade answers every failure with a readable body and a
// code from RFC 6749 §5.2 plus RFC 8693 §2.2.2, so this never has to parse prose.
//
// `temporarily_unavailable` is the console saying "not now" — it reads as
// unavailable rather than as a refusal of the person, because that is what it
// is, and because a panel that says "access denied" for a converging role
// removal sends someone to the wrong place.
func classify(status int, body []byte) error {
	var e errorResponse
	_ = json.Unmarshal(body, &e)

	switch {
	case e.Error == "temporarily_unavailable", e.Error == "server_error":
		return &Error{Outcome: Unavailable, Detail: describe(e)}
	case status >= 500, status == http.StatusTooManyRequests:
		return failf(Unavailable, "the exchange answered %d", status)
	case status >= 300 && status < 400:
		// Not followed (answerRedirects): the URL names something other than
		// the endpoint itself.
		return failf(Misconfigured, "the exchange URL answered a redirect (%d); set it to the endpoint itself", status)
	case status == http.StatusUnauthorized:
		// The delegate credential itself was refused: an operator problem, not
		// this person's access.
		return &Error{Outcome: Misconfigured, Detail: "the exchange refused the delegate credential"}
	case e.Error == "invalid_client":
		return &Error{Outcome: Misconfigured, Detail: "the exchange refused the delegate credential"}
	case e.Error != "":
		return &Error{Outcome: Refused, Detail: describe(e)}
	default:
		// Nothing in the facade's vocabulary came back. The facade answers
		// every failure with one of its codes, so whatever replied is not the
		// facade: an exchange URL pointing somewhere else, something in front
		// of it, or a host the console itself rejects — a Django
		// `DisallowedHost` 400 reads exactly like this. None of those is a
		// statement about this person's access, and reporting one as a refusal
		// sends an operator to look at someone's permissions when the cause is
		// the URL.
		return failf(Misconfigured, "the exchange URL answered %d with no OAuth error, so it may not be the console's exchange endpoint", status)
	}
}

// describe renders the facade's codes and nothing else. Both fields come from a
// closed vocabulary, so neither can carry token material or a person's data.
func describe(e errorResponse) string {
	if e.Description == "" {
		return e.Error
	}
	return e.Error + "/" + e.Description
}
