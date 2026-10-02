package exchange

import (
	"fmt"
	"net/http"
)

// Transport attaches a cluster token to every outbound request, taken from the
// token source at request time rather than frozen into the connection.
//
// This is the seam the design turns on. The connection cache holds a *sql.DB for
// up to an hour, while a cluster token lives minutes, so a credential built when
// the connection was opened is dead for most of that connection's life. Setting
// it per request makes a token refresh invisible to pooling — and mirrors the
// console's browser client, which attaches a token from its session manager to
// each request rather than binding one to a connection.
type Transport struct {
	Base   http.RoundTripper
	Source *Source
}

// NewTransport wraps a base round tripper. A nil base means the default.
func NewTransport(base http.RoundTripper, src *Source) *Transport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &Transport{Base: base, Source: src}
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	p, ok := PrincipalFrom(req.Context())
	if !ok {
		// No principal: refuse rather than send whatever the connection was
		// built with. A query with no identity has no business reaching a
		// cluster on this lane.
		return nil, fmt.Errorf("%w: this request carries no signed-in user", ErrRefused)
	}

	token, err := t.Source.Token(req.Context(), p.Audience, p.Subject, p.SubjectToken)
	if err != nil {
		return nil, err
	}

	resp, err := t.Base.RoundTrip(withBearer(req, token))
	if err != nil {
		return nil, err
	}

	// The cluster is the authority on whether a token is still good, whatever
	// the cache believes. A 401 means re-mint, once; a second 401 is the answer.
	if resp.StatusCode == http.StatusUnauthorized {
		t.Source.Invalidate(p.Audience, p.Subject)
		fresh, mintErr := t.Source.Token(req.Context(), p.Audience, p.Subject, p.SubjectToken)
		if mintErr != nil || fresh == token {
			// Hand back the cluster's own 401 rather than a mint error: the
			// request did complete, and the caller can read its body. Nothing
			// new to present means nothing to retry with.
			return resp, nil
		}
		drain(resp)
		return t.Base.RoundTrip(withBearer(req, fresh))
	}

	return resp, nil
}

// withBearer clones the request with the Authorization header replaced. The
// clone matters: a RoundTripper must not mutate the request it is given, and a
// retry needs a request whose body can be read again.
func withBearer(req *http.Request, token string) *http.Request {
	clone := req.Clone(req.Context())
	clone.Header.Set("Authorization", "Bearer "+token)
	if req.GetBody != nil {
		if body, err := req.GetBody(); err == nil {
			clone.Body = body
		}
	}
	return clone
}

func drain(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_ = resp.Body.Close()
}
