package exchange

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
)

// CredentialsType is the datasource credentials mode this package serves. It
// behaves like `forwardOAuth` except that the forwarded token is exchanged for a
// cluster token before it is used.
const CredentialsType = "forwardOAuthExchange"

// Principal is who a query runs as, carried on the query's context from the
// point Grafana hands over the forwarded token to the point the outbound HTTP
// request is built.
//
// It travels on the context rather than in `connectionArgs` because
// connectionArgs are the connection cache's key: the subject belongs there (it
// is stable for a person), the subject token does not (Grafana refreshes it
// every few minutes, and keying on it would rebuild the pool each time).
type Principal struct {
	// Audience is the cluster's configured token audience — what the exchange
	// asks for, and what the cluster checks.
	Audience string
	// Subject is the `sub` claim: stable per person, distinct between people,
	// and not a secret.
	Subject string
	// SubjectToken is the token Grafana forwarded. It is exchanged, never sent
	// to a cluster.
	SubjectToken string
}

type principalKey struct{}

// WithPrincipal puts the principal on the context.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFrom reads the principal back. A query that reaches the transport
// without one is a query this package must refuse: there is nothing to exchange,
// and the alternative — sending the forwarded token to the cluster — is the one
// thing this whole lane exists to avoid.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	if !ok || p.Subject == "" || p.SubjectToken == "" || p.Audience == "" {
		return Principal{}, false
	}
	return p, true
}

// SubjectOf reads the `sub` claim from a JWT without verifying its signature.
//
// Unverified is correct here: this value only picks a cache slot and a
// connection-pool key. The console's exchange facade is the verifier — it pins
// the issuer, the audience and the signature before it mints anything — so a
// forged `sub` buys an attacker their own empty cache entry and a refusal.
func SubjectOf(jwt string) string {
	parts := strings.Split(jwt, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	return claims.Sub
}
