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

// Slot is one person at one cluster: what the token cache and the principal
// registry key on, and what a connection is bound to. It carries no token, so
// it can name a person whose token is not in hand — the connection's own
// handshake, which runs before any query's context arrives.
type Slot struct {
	// Audience is the cluster's configured token audience — what the exchange
	// asks for, and what the cluster checks.
	Audience string
	// Subject is who the person is: the token's issuer and its `sub` claim, as
	// IdentityOf joins them. Stable per person, distinct between people, and not
	// a secret.
	Subject string
}

// named is whether the slot names somebody at some cluster. An empty slot is
// nobody's, and answers nothing.
func (s Slot) named() bool { return s.Audience != "" && s.Subject != "" }

// Principal is who a query runs as, carried on the query's context from the
// point Grafana hands over the forwarded token to the point the outbound HTTP
// request is built: a slot, and the token that proves the person is in it.
//
// It travels on the context rather than in `connectionArgs` because
// connectionArgs are the connection cache's key: the subject belongs there (it
// is stable for a person), the subject token does not (Grafana refreshes it
// every few minutes, and keying on it would rebuild the pool each time).
type Principal struct {
	Slot
	// SubjectToken is the token Grafana forwarded. It is exchanged, never sent
	// to a cluster.
	SubjectToken string
}

type principalKey struct{}

// WithPrincipal puts the principal on the context.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFrom reads the principal back. A query that reaches the JWT hook
// without one is a query this package must refuse: there is nothing to exchange,
// and the alternative — sending the forwarded token to the cluster — is the one
// thing this whole lane exists to avoid.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	if !ok || !p.named() || p.SubjectToken == "" {
		return Principal{}, false
	}
	return p, true
}

// claims are the parts of a JWT's payload this package reads.
type claims struct {
	Iss string `json:"iss"`
	Sub string `json:"sub"`
	Exp int64  `json:"exp"`
}

// claimsOf reads a JWT's payload without verifying its signature.
//
// Unverified is correct here: these values only pick a cache slot and a
// connection-pool key, and the slot is answered only to the exact token that
// filled it (see Source). The console's exchange facade is the verifier — it
// pins the issuer, the audience and the signature before it mints anything.
func claimsOf(jwt string) (claims, bool) {
	parts := strings.Split(jwt, ".")
	if len(parts) < 2 {
		return claims{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims{}, false
	}
	var c claims
	if err := json.Unmarshal(payload, &c); err != nil {
		return claims{}, false
	}
	return c, true
}

// SubjectOf reads the `sub` claim from a JWT without verifying its signature.
func SubjectOf(jwt string) string {
	c, _ := claimsOf(jwt)
	return c.Sub
}

// IdentityOf answers who a JWT names: its issuer and its `sub`, joined. Empty
// when either is missing, and a token that names nobody is refused downstream.
//
// `sub` alone is not an identity. It is unique per issuer, and Grafana can run
// several OAuth providers at once, so two people from two providers can share a
// `sub`; keyed on that alone, one person's cluster token would serve the other.
// The separator cannot appear in an issuer, which is a URL, so the first one
// splits the pair unambiguously even where a `sub` contains it.
func IdentityOf(jwt string) string {
	c, _ := claimsOf(jwt)
	if c.Iss == "" || c.Sub == "" {
		return ""
	}
	return c.Iss + "|" + c.Sub
}

// Principals remembers the forwarded token last presented for each subject.
//
// The context is the better carrier and is tried first, but it does not always
// arrive: a connection's own handshake — clickhouse-go's "server hello" — runs
// when database/sql establishes the connection, and the context it gets is not
// always the query's. A query that could not be identified would then be
// refused even though Grafana forwarded a perfectly good token, which is what
// the first live run of this change showed.
//
// So the subject, which keys the connection, also keys this registry, and the
// binding can recover the token for the connection it belongs to. An entry is
// the token that person's most recent request carried — last presented, not
// newest by issuance: in the moment after a Grafana refresh, a request that
// still carries the previous token writes it back, and the handshake
// exchanges that one instead, at the cost of one more exchange. Each entry
// is dropped once the token itself expires: past its `exp` the facade would
// refuse it, so holding it is only retention. The idle limit is the backstop
// for a token with no `exp`.
type Principals struct {
	mu    sync.Mutex
	known map[string]remembered
	idle  time.Duration
	now   func() time.Time
}

type remembered struct {
	principal Principal
	until     time.Time
}

// NewPrincipals builds a registry. An entry is dropped when its token expires,
// or once unused for `idle` if that comes first; zero means one hour.
func NewPrincipals(idle time.Duration, now func() time.Time) *Principals {
	if idle <= 0 {
		idle = time.Hour
	}
	if now == nil {
		now = time.Now
	}
	return &Principals{known: map[string]remembered{}, idle: idle, now: now}
}

// Remember records the token this subject has just presented at this cluster,
// replacing whatever was there.
func (p *Principals) Remember(pr Principal) {
	if !pr.named() || pr.SubjectToken == "" {
		return
	}
	now := p.now()
	until := now.Add(p.idle)
	if c, ok := claimsOf(pr.SubjectToken); ok && c.Exp > 0 {
		if exp := time.Unix(c.Exp, 0); exp.Before(until) {
			until = exp
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.known[key(pr.Slot)] = remembered{principal: pr, until: until}
	p.pruneLocked(now)
}

// Lookup answers the token last presented for this slot.
func (p *Principals) Lookup(s Slot) (Principal, bool) {
	if !s.named() {
		return Principal{}, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	entry, ok := p.known[key(s)]
	if !ok || !p.now().Before(entry.until) {
		return Principal{}, false
	}
	return entry.principal, true
}

func (p *Principals) pruneLocked(now time.Time) {
	for k, v := range p.known {
		if !now.Before(v.until) {
			delete(p.known, k)
		}
	}
}
