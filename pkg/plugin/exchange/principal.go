package exchange

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"sync"
	"time"
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

// Principals remembers the newest forwarded token for each subject.
//
// The context is the better carrier and is tried first, but it does not always
// arrive: a connection's own handshake — clickhouse-go's "server hello" — runs
// when database/sql establishes the connection, and the context it gets is not
// always the query's. A query that could not be identified would then be
// refused even though Grafana forwarded a perfectly good token, which is what
// the first live run of this change showed.
//
// So the subject, which keys the connection, also keys this registry, and the
// transport can recover the token for the connection it belongs to. Entries are
// the newest token seen for that person and are dropped once they go unused,
// because a token Grafana has stopped refreshing is of no further use.
type Principals struct {
	mu    sync.Mutex
	known map[string]remembered
	idle  time.Duration
	now   func() time.Time
}

type remembered struct {
	principal Principal
	seen      time.Time
}

// NewPrincipals builds a registry. An entry unused for `idle` is dropped; zero
// means one hour.
func NewPrincipals(idle time.Duration, now func() time.Time) *Principals {
	if idle <= 0 {
		idle = time.Hour
	}
	if now == nil {
		now = time.Now
	}
	return &Principals{known: map[string]remembered{}, idle: idle, now: now}
}

// Remember records the newest token for this subject at this cluster.
func (p *Principals) Remember(pr Principal) {
	if pr.Subject == "" || pr.SubjectToken == "" || pr.Audience == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.known[key(pr.Audience, pr.Subject)] = remembered{principal: pr, seen: p.now()}
	p.pruneLocked()
}

// Lookup answers the newest token for this subject at this cluster.
func (p *Principals) Lookup(audience, subject string) (Principal, bool) {
	if audience == "" || subject == "" {
		return Principal{}, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	entry, ok := p.known[key(audience, subject)]
	if !ok || p.now().Sub(entry.seen) > p.idle {
		return Principal{}, false
	}
	return entry.principal, true
}

func (p *Principals) pruneLocked() {
	cutoff := p.now().Add(-p.idle)
	for k, v := range p.known {
		if v.seen.Before(cutoff) {
			delete(p.known, k)
		}
	}
}
