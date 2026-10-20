package exchange

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// Timings ported from the console's browser session manager. The comments say
// why each exists, because the numbers are cheap to copy and expensive to
// rediscover.
const (
	// A token inside this window of its deadline is treated as already expired,
	// so no request in flight carries one that dies on the way.
	expirySkew = 30 * time.Second

	// Refresh ahead of expiry, proportionally: a fraction of the token's
	// lifetime, clamped. A 300 s token refreshes with 60 s left; an hour-long
	// token with 5 minutes left. Refreshing while the current token is still
	// usable is what lets a failed refresh be survivable — the old token keeps
	// serving until it truly expires.
	renewalLeadFraction = 0.2
	renewalLeadMin      = 60 * time.Second
	renewalLeadMax      = 300 * time.Second

	// A token shorter than this is never pre-refreshed: there is no useful room
	// between the lead and the skew, so it stays on the lazy path.
	renewalMinLifetime = 120 * time.Second

	// A failed refresh backs off 5 s, 10 s, 20 s, 40 s, then 60 s. Each rung is
	// only taken if it lands before the current token's real expiry; past that
	// the lazy path takes over and the caller sees the error.
	retryBase = 5 * time.Second
	retryCap  = 60 * time.Second

	// How long a refusal is remembered. The browser has no equivalent because a
	// browser is one user: server-side, a refused person reloading a thirty-panel
	// dashboard would otherwise ask the console thirty times, and a converging
	// role removal would become a load spike against one delegate's quota.
	refusalMemory = 10 * time.Second
)

// Logger is the narrow logging surface this package needs. Implementations must
// never be handed a token, a query or a secret; see the call sites, which pass
// only an audience, a truncated subject and an outcome.
type Logger interface {
	Debug(msg string, args ...any)
	Warn(msg string, args ...any)
}

type nopLogger struct{}

func (nopLogger) Debug(string, ...any) {}
func (nopLogger) Warn(string, ...any)  {}

type entry struct {
	token    string
	deadline time.Time // client clock, stamped from the request's start
	lead     time.Duration

	// subjectHash names the forwarded token this entry was minted from: the
	// one the console verified. The entry is answered only to that token,
	// so a hit never stands in for a verification the console did not do.
	// A refreshed sign-in token is a new string, misses, and is exchanged.
	subjectHash [sha256.Size]byte

	// attempts and nextTry drive the retry ladder for a failed refresh.
	attempts int
	nextTry  time.Time

	// refusedUntil suppresses asking again after the console said no.
	refusedUntil time.Time
}

func (e *entry) usable(now time.Time) bool {
	return e.token != "" && now.Before(e.deadline.Add(-expirySkew))
}

// mintedFrom reports whether this entry was minted from that forwarded token.
func (e *entry) mintedFrom(subjectHash [sha256.Size]byte) bool {
	return e.subjectHash == subjectHash
}

// hashOf fingerprints a forwarded token. The fingerprint is kept rather than
// the token: an entry needs to recognise its token, not hold a second copy.
func hashOf(subjectToken string) [sha256.Size]byte {
	return sha256.Sum256([]byte(subjectToken))
}

func (e *entry) stale(now time.Time) bool {
	return e.lead > 0 && !now.Before(e.deadline.Add(-e.lead))
}

// Source hands out cluster tokens for (audience, subject) pairs, exchanging as
// needed and never more than once at a time per pair.
//
// It is keyed on the subject claim, not on the forwarded token: Grafana
// refreshes the sign-in token, so the token string changes every few minutes
// while the person does not, and where the key reaches the connection pool,
// keying on the token would churn that too.
//
// The subject claim is read without verifying the token, so the key alone
// must never decide who is served. Each entry remembers a hash of the
// forwarded token that minted it, and is answered only to that token: on a
// hit, the console verified this very string; on a miss, the console is
// asked. A refreshed sign-in token therefore costs one exchange per refresh,
// coalesced across a dashboard's panels, which is the price of never serving
// a cluster token to a string nobody checked.
type Source struct {
	ex  Exchanger
	now func() time.Time
	log Logger

	mu      sync.Mutex
	entries map[string]*entry
	gens    map[string]uint64

	flight singleflight.Group
}

// NewSource builds a Source. `now` and `log` may be nil.
func NewSource(ex Exchanger, now func() time.Time, log Logger) *Source {
	if now == nil {
		now = time.Now
	}
	if log == nil {
		log = nopLogger{}
	}
	return &Source{
		ex:      ex,
		now:     now,
		log:     log,
		entries: map[string]*entry{},
		gens:    map[string]uint64{},
	}
}

// exchangeCeiling bounds a detached flight. The HTTP exchanger carries its own
// 10s client timeout; this is the backstop that keeps a hung Exchanger from
// leaking a flight forever now that no caller's cancellation can end one.
const exchangeCeiling = 30 * time.Second

func key(audience, subject string) string { return audience + "\x00" + subject }

// Token answers a cluster token for this subject at this cluster.
//
// A usable cached token is answered without waiting on anything. A usable but
// ageing one triggers a refresh whose failure is invisible — the old token is
// still good. Only an unusable token makes the caller wait, and only then can
// this return an error.
func (s *Source) Token(ctx context.Context, audience, subject, subjectToken string) (string, error) {
	k := key(audience, subject)
	h := hashOf(subjectToken)
	now := s.now()

	s.mu.Lock()
	e := s.entries[k]
	var (
		cached       string
		isUsable     bool
		isStale      bool
		mayTry       bool
		stillRefused bool
	)
	if e != nil {
		// An entry minted from another forwarded token is not usable for
		// this one, whatever its clock says: not served, and not served as
		// the token in hand when a refresh fails either. The refusal memory
		// and the retry ladder stay, as they restrict rather than grant.
		cached, isUsable, isStale = e.token, e.usable(now) && e.mintedFrom(h), e.stale(now)
		mayTry = !now.Before(e.nextTry)
		stillRefused = now.Before(e.refusedUntil)
	}
	s.mu.Unlock()

	if isUsable && !isStale {
		return cached, nil
	}

	if isUsable && isStale {
		if !mayTry {
			// Backing off from a failed refresh, and the token still works.
			return cached, nil
		}
		token, err := s.exchange(ctx, k, audience, subject, subjectToken)
		if err != nil {
			if errors.Is(err, ErrRefused) {
				// The console has withdrawn this person's access, and the
				// exchange has already dropped the token in hand. Serving it
				// anyway would carry the old access past the refusal for as
				// long as the token lived: a transient failure is the
				// console's problem, a refusal is the answer.
				return "", err
			}
			s.log.Warn("exchange refresh failed, serving the token in hand",
				"audience", audience, "subject", truncate(subject), "outcome", outcome(err))
			return cached, nil
		}
		return token, nil
	}

	// No usable token: the caller waits, and an error reaches it.
	if stillRefused {
		// Answer from the refusal memory rather than asking again.
		s.log.Debug("exchange refused recently, not asking again yet",
			"audience", audience, "subject", truncate(subject))
		return "", ErrRefused
	}
	return s.exchange(ctx, k, audience, subject, subjectToken)
}

// exchange performs one exchange for the key, coalescing concurrent callers.
// Thirty panels opening one dashboard make one exchange, not thirty.
//
// The flight is keyed on the forwarded token as well as the subject, so a
// caller only ever receives a token minted from the string it presented. A
// dashboard load that straddles a Grafana refresh makes two exchanges, one per
// token, rather than handing the second token's panels a result the console
// minted for the first.
func (s *Source) exchange(ctx context.Context, k, audience, subject, subjectToken string) (string, error) {
	h := hashOf(subjectToken)
	ch := s.flight.DoChan(k+"\x00"+string(h[:]), func() (any, error) {
		// Detached from every caller, deliberately. The flight is SHARED: the
		// thirty panels above are one exchange, and running it on whichever
		// caller happened to arrive first lets that one caller cancel work the
		// other twenty-nine are still waiting on. In Grafana a caller going
		// away is routine — a panel refresh, a navigation, a closed dashboard —
		// so the leader's context is the wrong lifetime for shared work.
		// `WithoutCancel` keeps the values (the principal rides there) and
		// drops only the cancellation.
		exCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), exchangeCeiling)
		defer cancel()

		// A caller can find the cache empty, then reach here only after the
		// flight that filled it has finished, which starts a new flight. Look
		// again first, so that late caller gets the token a moment ago's
		// exchange produced instead of making a second one.
		s.mu.Lock()
		gen := s.gens[k]
		if e := s.entries[k]; e != nil && e.usable(s.now()) && !e.stale(s.now()) && e.mintedFrom(h) {
			cached := e.token
			s.mu.Unlock()
			return cached, nil
		}
		s.mu.Unlock()

		// The deadline is stamped from the moment the request STARTED, so the
		// local deadline sits at or before the console's true expiry by one
		// round trip. A server timestamp is never compared to the local clock.
		started := s.now()
		token, lifetime, exErr := s.ex.Exchange(exCtx, audience, subjectToken)
		if exErr != nil {
			s.noteFailure(k, exErr)
			return "", exErr
		}
		s.store(k, gen, token, h, started, lifetime)
		s.log.Debug("exchanged", "audience", audience, "subject", truncate(subject),
			"lifetime_s", int(lifetime.Seconds()))
		return token, nil
	})

	select {
	case res := <-ch:
		if res.Err != nil {
			return "", res.Err
		}
		token, _ := res.Val.(string)
		return token, nil
	case <-ctx.Done():
		// THIS caller stops waiting. The flight carries on for the others, and
		// its result still reaches the cache, so the exchange is not wasted.
		// Nothing is recorded as a failure: the console did not fail, a browser
		// moved on, and putting the key on a backoff ladder for that would let
		// ordinary navigation degrade every later request for this person.
		return "", ctx.Err()
	}
}

// store caches a fresh token unless the session was invalidated while it was in
// flight. Without the generation check, an exchange that began before an
// invalidation would quietly restore the session it was meant to end.
func (s *Source) store(k string, gen uint64, token string, subjectHash [sha256.Size]byte, started time.Time, lifetime time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gens[k] != gen {
		return
	}
	lead := time.Duration(0)
	if lifetime >= renewalMinLifetime {
		lead = time.Duration(float64(lifetime) * renewalLeadFraction)
		if lead < renewalLeadMin {
			lead = renewalLeadMin
		}
		if lead > renewalLeadMax {
			lead = renewalLeadMax
		}
	}
	s.entries[k] = &entry{token: token, deadline: started.Add(lifetime), lead: lead, subjectHash: subjectHash}
}

// noteFailure advances the retry ladder, or remembers a refusal.
func (s *Source) noteFailure(k string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.entries[k]
	if e == nil {
		e = &entry{}
		s.entries[k] = e
	}
	now := s.now()

	if errors.Is(err, ErrRefused) {
		// The console says this person may not query this cluster. Drop the
		// token rather than keep serving it: a refusal is the removal path
		// taking effect, and continuing to serve would be the one thing the
		// console's gate chain is there to prevent.
		e.token = ""
		e.refusedUntil = now.Add(refusalMemory)
		e.attempts = 0
		e.nextTry = time.Time{}
		return
	}

	e.attempts++
	backoff := retryBase << (e.attempts - 1)
	if backoff > retryCap || backoff <= 0 {
		backoff = retryCap
	}
	next := now.Add(backoff)
	// A rung past the token's real expiry is pointless: the lazy path takes
	// over there and the caller gets the error instead of a stale token.
	if !e.deadline.IsZero() && next.After(e.deadline) {
		next = e.deadline
	}
	e.nextTry = next
}

// Invalidate drops this subject's token for this cluster and bumps the
// generation, so an exchange already in flight cannot repopulate it. The caller
// is the cluster: a 401 means the cluster no longer accepts what we hold,
// whatever this cache believes.
func (s *Source) Invalidate(audience, subject string) {
	k := key(audience, subject)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gens[k]++
	delete(s.entries, k)
}

// ConfiguredFor reports whether the exchanger holds a delegate credential for
// the audience. Only the HTTP exchanger knows; any other (a test double, nil)
// answers no rather than panicking on the assertion.
func (s *Source) ConfiguredFor(audience string) bool {
	if ex, ok := s.ex.(*HTTPExchanger); ok && ex != nil {
		return ex.ConfiguredFor(audience)
	}
	return false
}

// outcome names an error for a log line, in a closed vocabulary.
func outcome(err error) string {
	switch {
	case errors.Is(err, ErrRefused):
		return "refused"
	case errors.Is(err, ErrMisconfigured):
		return "misconfigured"
	case errors.Is(err, ErrUnavailable):
		return "unavailable"
	default:
		return "error"
	}
}

// truncate shortens a subject for a log line. A subject is an opaque id rather
// than a secret, but a whole one in a log is more than a reader needs.
func truncate(subject string) string {
	// Log the person's part of an issuer-qualified subject, not the issuer.
	if i := strings.Index(subject, "|"); i >= 0 {
		subject = subject[i+1:]
	}
	const n = 12
	if len(subject) <= n {
		return subject
	}
	return subject[:n]
}
