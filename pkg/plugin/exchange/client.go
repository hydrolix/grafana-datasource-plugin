package exchange

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// The three outcomes a caller can act on differently. They are separate because
// the remedies differ and a panel should say which one happened: a refusal is
// about this person's access, unavailability is about the console, and a
// misconfiguration is about the operator.
var (
	// ErrRefused: the console answered, and the answer was no.
	ErrRefused = errors.New("exchange: the console refused this account access to this cluster")
	// ErrUnavailable: the console did not answer, or answered that it could not
	// mint just now — a role removal converging, Keycloak unreachable,
	// provisioning deferred.
	ErrUnavailable = errors.New("exchange: the console could not be reached")
	// ErrMisconfigured: this cluster has no delegate credential here.
	ErrMisconfigured = errors.New("exchange: no delegate credential is configured for this cluster")
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

// NewHTTPExchanger builds an exchanger with a bounded client. The timeout is
// deliberately shorter than a panel's patience: a slow console should read as
// unavailable rather than hold a query open.
func NewHTTPExchanger(cfg Config) *HTTPExchanger {
	return &HTTPExchanger{Config: cfg, Client: &http.Client{Timeout: 10 * time.Second}}
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
		return "", 0, fmt.Errorf("%w: %s", ErrMisconfigured, audience)
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
		return "", 0, fmt.Errorf("%w: building the request failed", ErrUnavailable)
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
		return "", 0, fmt.Errorf("%w: the exchange answered 200 without a usable token", ErrUnavailable)
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
		return fmt.Errorf("%w: %s", ErrUnavailable, describe(e))
	case status >= 500, status == http.StatusTooManyRequests:
		return fmt.Errorf("%w: the exchange answered %d", ErrUnavailable, status)
	case status == http.StatusUnauthorized:
		// The delegate credential itself was refused: an operator problem, not
		// this person's access.
		return fmt.Errorf("%w: the exchange refused the delegate credential", ErrMisconfigured)
	case e.Error == "invalid_client":
		return fmt.Errorf("%w: the exchange refused the delegate credential", ErrMisconfigured)
	case e.Error != "":
		return fmt.Errorf("%w: %s", ErrRefused, describe(e))
	default:
		return fmt.Errorf("%w: the exchange answered %d", ErrRefused, status)
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
