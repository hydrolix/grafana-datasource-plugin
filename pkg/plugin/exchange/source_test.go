package exchange

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	aud  = "cluster.example.hydrolix.net"
	sub  = "kc-sub-alice-0123456789"
	stok = "the.subject.token"
)

// alice is the one person most tests here run as, at the one cluster.
var alice = Principal{Slot: Slot{Audience: aud, Subject: sub}, SubjectToken: stok}

// someone is a person other than alice at the test cluster, with her token.
func someone(subject string) Principal {
	return Principal{Slot: Slot{Audience: aud, Subject: subject}, SubjectToken: stok}
}

// presenting is alice with a different forwarded token in hand.
func presenting(token string) Principal {
	return Principal{Slot: alice.Slot, SubjectToken: token}
}

// fakeExchanger counts calls and answers whatever the test sets.
type fakeExchanger struct {
	mu       sync.Mutex
	calls    int32
	lifetime time.Duration
	err      error
	// block, when non-nil, holds every exchange until it is closed, so a test
	// can keep one in flight while it does something else.
	block chan struct{}
	seq   int
}

func (f *fakeExchanger) Exchange(ctx context.Context, audience, subjectToken string) (string, time.Duration, error) {
	atomic.AddInt32(&f.calls, 1)
	if f.block != nil {
		<-f.block
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return "", 0, f.err
	}
	f.seq++
	return fmt.Sprintf("cluster-token-%d", f.seq), f.lifetime, nil
}

func (f *fakeExchanger) count() int { return int(atomic.LoadInt32(&f.calls)) }

// clock is a hand-wound clock: these semantics are all about time, and a test
// that sleeps is a test that flakes.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func newClock() *clock { return &clock{t: time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)} }
func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}
func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func newFixture(lifetime time.Duration) (*Source, *fakeExchanger, *clock) {
	ex := &fakeExchanger{lifetime: lifetime}
	ck := newClock()
	return NewSource(ex, ck.now, nil), ex, ck
}

// waitForExchange blocks until the fake has been entered, so a test can act
// while an exchange is in flight without sleeping for a guessed interval.
func waitForExchange(t *testing.T, ex *fakeExchanger) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for ex.count() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("no exchange began within two seconds")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestConcurrentCallersCauseOneExchange(t *testing.T) {
	// ADR 042: a dashboard of thirty panels makes one exchange, not thirty.
	s, ex, _ := newFixture(300 * time.Second)
	ex.block = make(chan struct{})

	var wg sync.WaitGroup
	got := make([]string, 30)
	errs := make([]error, 30)
	for i := range got {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			got[i], errs[i] = s.Token(context.Background(), alice)
		}(i)
	}
	// Wait for the one exchange to begin, then release it.
	waitForExchange(t, ex)
	close(ex.block)
	wg.Wait()

	if ex.count() != 1 {
		t.Fatalf("expected 1 exchange for 30 concurrent callers, got %d", ex.count())
	}
	for i := range got {
		if errs[i] != nil {
			t.Fatalf("caller %d failed: %v", i, errs[i])
		}
		if got[i] != "cluster-token-1" {
			t.Fatalf("caller %d got %q", i, got[i])
		}
	}
}

func TestCachedTokenIsServedWithoutExchanging(t *testing.T) {
	s, ex, ck := newFixture(300 * time.Second)
	if _, err := s.Token(context.Background(), alice); err != nil {
		t.Fatal(err)
	}
	// Well inside the lead window (300 s ⇒ lead 60 s ⇒ stale at 240 s).
	ck.advance(100 * time.Second)
	for i := 0; i < 5; i++ {
		if _, err := s.Token(context.Background(), alice); err != nil {
			t.Fatal(err)
		}
	}
	if ex.count() != 1 {
		t.Fatalf("expected the cache to serve, got %d exchanges", ex.count())
	}
}

func TestRefreshAheadOfExpiry(t *testing.T) {
	s, ex, ck := newFixture(300 * time.Second) // lead = max(60s, 20% of 300s) = 60s
	if _, err := s.Token(context.Background(), alice); err != nil {
		t.Fatal(err)
	}
	ck.advance(239 * time.Second) // not yet stale
	if _, _ = s.Token(context.Background(), alice); ex.count() != 1 {
		t.Fatalf("refreshed too early: %d exchanges", ex.count())
	}
	ck.advance(2 * time.Second) // now inside the lead window
	tok, err := s.Token(context.Background(), alice)
	if err != nil {
		t.Fatal(err)
	}
	if ex.count() != 2 {
		t.Fatalf("expected a refresh inside the lead window, got %d exchanges", ex.count())
	}
	if tok != "cluster-token-2" {
		t.Fatalf("expected the refreshed token, got %q", tok)
	}
}

func TestShortLivedTokenIsNeverPreRefreshed(t *testing.T) {
	// Under the minimum lifetime there is no useful room between lead and skew,
	// so the token stays on the lazy path.
	s, ex, ck := newFixture(90 * time.Second)
	if _, err := s.Token(context.Background(), alice); err != nil {
		t.Fatal(err)
	}
	ck.advance(50 * time.Second)
	if _, _ = s.Token(context.Background(), alice); ex.count() != 1 {
		t.Fatalf("pre-refreshed a short token: %d exchanges", ex.count())
	}
	ck.advance(15 * time.Second) // now inside the 30 s skew ⇒ unusable
	if _, err := s.Token(context.Background(), alice); err != nil {
		t.Fatal(err)
	}
	if ex.count() != 2 {
		t.Fatalf("expected a lazy re-mint once inside the skew, got %d", ex.count())
	}
}

func TestAFailedRefreshKeepsServingTheTokenInHand(t *testing.T) {
	s, ex, ck := newFixture(300 * time.Second)
	first, err := s.Token(context.Background(), alice)
	if err != nil {
		t.Fatal(err)
	}
	ck.advance(241 * time.Second) // stale, still usable
	ex.err = ErrUnavailable

	tok, err := s.Token(context.Background(), alice)
	if err != nil {
		t.Fatalf("a failed refresh must be invisible while the token is usable: %v", err)
	}
	if tok != first {
		t.Fatalf("expected the token in hand, got %q", tok)
	}
}

func TestTheRetryLadderBacksOff(t *testing.T) {
	s, ex, ck := newFixture(3600 * time.Second) // lead 300 s
	if _, err := s.Token(context.Background(), alice); err != nil {
		t.Fatal(err)
	}
	ck.advance(3400 * time.Second) // stale (300 s lead), usable (30 s skew)
	ex.err = ErrUnavailable

	if _, _ = s.Token(context.Background(), alice); ex.count() != 2 {
		t.Fatalf("expected one refresh attempt, got %d", ex.count())
	}
	// Inside the first 5 s rung: no further attempt.
	ck.advance(3 * time.Second)
	if _, _ = s.Token(context.Background(), alice); ex.count() != 2 {
		t.Fatalf("attempted inside the backoff rung: %d", ex.count())
	}
	// Past it: one more attempt, and the ladder lengthens.
	ck.advance(3 * time.Second)
	if _, _ = s.Token(context.Background(), alice); ex.count() != 3 {
		t.Fatalf("expected a second attempt past the rung, got %d", ex.count())
	}
	ck.advance(6 * time.Second) // inside the 10 s rung
	if _, _ = s.Token(context.Background(), alice); ex.count() != 3 {
		t.Fatalf("the ladder did not lengthen: %d", ex.count())
	}
}

func TestARefusalIsRememberedBriefly(t *testing.T) {
	// A refused person reloading a thirty-panel dashboard must not ask thirty
	// times, and a converging removal must not become a load spike.
	s, ex, ck := newFixture(300 * time.Second)
	ex.err = fmt.Errorf("%w: access_denied", ErrRefused)

	for i := 0; i < 10; i++ {
		_, err := s.Token(context.Background(), alice)
		if !errors.Is(err, ErrRefused) {
			t.Fatalf("call %d: expected a refusal, got %v", i, err)
		}
	}
	if ex.count() != 1 {
		t.Fatalf("expected the refusal to be remembered, got %d exchanges", ex.count())
	}
	ck.advance(refusalMemory + time.Second)
	if _, err := s.Token(context.Background(), alice); !errors.Is(err, ErrRefused) {
		t.Fatalf("expected a refusal after the memory lapsed, got %v", err)
	}
	if ex.count() != 2 {
		t.Fatalf("expected one more ask after the memory lapsed, got %d", ex.count())
	}
}

func TestARefusalDropsAUsableToken(t *testing.T) {
	// A refusal is the removal path taking effect; continuing to serve would
	// defeat the gate chain that produced it.
	s, ex, ck := newFixture(300 * time.Second)
	if _, err := s.Token(context.Background(), alice); err != nil {
		t.Fatal(err)
	}
	ck.advance(241 * time.Second) // stale but usable
	ex.err = fmt.Errorf("%w: access_denied", ErrRefused)

	// The refresh fails with a refusal. The token in hand was usable when the
	// call began, and a transient failure would serve it, but a refusal is the
	// console's answer about this person: the refreshing call is refused too,
	// not served from the token the refusal just withdrew...
	if _, err := s.Token(context.Background(), alice); !errors.Is(err, ErrRefused) {
		t.Fatalf("expected the refreshing call to be refused, got %v", err)
	}
	// ...and so is the next one: the token is gone.
	if _, err := s.Token(context.Background(), alice); !errors.Is(err, ErrRefused) {
		t.Fatalf("expected the next call to be refused, got %v", err)
	}
}

func TestATransientRefreshFailureStillServesTheTokenInHand(t *testing.T) {
	// The counterpart: the console being unreachable says nothing about this
	// person, so a usable token keeps serving while the refresh backs off.
	s, ex, ck := newFixture(300 * time.Second)
	first, err := s.Token(context.Background(), alice)
	if err != nil {
		t.Fatal(err)
	}
	ck.advance(241 * time.Second) // stale but usable
	ex.err = fmt.Errorf("%w: dial", ErrUnavailable)
	got, err := s.Token(context.Background(), alice)
	if err != nil || got != first {
		t.Fatalf("expected the token in hand, got %q, %v", got, err)
	}
}

func TestInvalidateForcesAFreshExchange(t *testing.T) {
	s, ex, _ := newFixture(300 * time.Second)
	if _, err := s.Token(context.Background(), alice); err != nil {
		t.Fatal(err)
	}
	s.Invalidate(alice.Slot)
	tok, err := s.Token(context.Background(), alice)
	if err != nil {
		t.Fatal(err)
	}
	if ex.count() != 2 || tok != "cluster-token-2" {
		t.Fatalf("expected a fresh exchange after invalidation: calls=%d token=%q", ex.count(), tok)
	}
}

func TestAnExchangeInFlightCannotOutliveAnInvalidation(t *testing.T) {
	// The generation check: without it, an exchange that began before an
	// invalidation would quietly restore the session it was meant to end.
	s, ex, _ := newFixture(300 * time.Second)
	ex.block = make(chan struct{})

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = s.Token(context.Background(), alice)
	}()
	waitForExchange(t, ex)
	s.Invalidate(alice.Slot) // while the exchange is in flight
	close(ex.block)
	<-done

	if s.cachedToken(alice.Slot) != "" {
		t.Fatal("an exchange that started before the invalidation populated the cache")
	}
}

func TestEntriesNobodyUsesAreDropped(t *testing.T) {
	// Entries are keyed on the person, so without a prune the cache grows by
	// one per (cluster, person) pair ever seen and never shrinks. An expired,
	// untouched entry is dropped on the next write; one whose token is still
	// usable is kept whatever its age.
	s, ex, ck := newFixture(300 * time.Second)
	for i := 0; i < 50; i++ {
		if _, err := s.Token(context.Background(), someone(fmt.Sprintf("sub-%d", i))); err != nil {
			t.Fatal(err)
		}
	}
	// A refusal and an invalidation make entries with no token at all; they
	// must age out the same way.
	ex.mu.Lock()
	ex.err = ErrRefused
	ex.mu.Unlock()
	_, _ = s.Token(context.Background(), someone("sub-refused"))
	ex.mu.Lock()
	ex.err = nil
	ex.mu.Unlock()
	s.Invalidate(Slot{Audience: aud, Subject: "sub-invalidated"})
	if n := s.entryCount(); n != 52 {
		t.Fatalf("expected 52 entries before any pruning, got %d", n)
	}

	// Everyone's token has expired, but nobody has written since. The prune
	// runs on a write, not on a clock.
	ck.advance(300*time.Second + 1)
	if n := s.entryCount(); n != 52 {
		t.Fatalf("expected no pruning without a write, got %d entries", n)
	}

	// Not idle yet: a write at this point keeps every entry.
	if _, err := s.Token(context.Background(), someone("sub-late")); err != nil {
		t.Fatal(err)
	}
	if n := s.entryCount(); n != 53 {
		t.Fatalf("expected 53 entries before entryIdle has passed, got %d", n)
	}

	// Past entryIdle since those entries were last touched, the next write
	// drops them. The late subject's entry is younger than entryIdle, so it
	// stays even though its token has expired too.
	ck.advance(entryIdle - 299*time.Second)
	if _, err := s.Token(context.Background(), someone("sub-latest")); err != nil {
		t.Fatal(err)
	}
	if n := s.entryCount(); n != 2 {
		t.Fatalf("expected only the two recent subjects to survive, got %d entries", n)
	}
	if s.cachedToken(Slot{Audience: aud, Subject: "sub-late"}) == "" {
		t.Fatal("an entry touched within entryIdle was pruned")
	}

	// A token that is still usable is kept however long ago it was written.
	long, _, lck := newFixture(3 * time.Hour)
	if _, err := long.Token(context.Background(), someone("sub-long")); err != nil {
		t.Fatal(err)
	}
	lck.advance(2 * time.Hour)
	if _, err := long.Token(context.Background(), someone("sub-other")); err != nil {
		t.Fatal(err)
	}
	if n := long.entryCount(); n != 2 {
		t.Fatalf("expected a usable token to survive the prune, got %d entries", n)
	}
}

func TestAnInvalidationOutlivesItsTokenUntilPruned(t *testing.T) {
	// The generation is what makes an invalidation stick against a flight
	// already running. It lives on the entry, so the entry must survive the
	// invalidation itself, and an exchange that captured the old generation
	// must still be refused after the token is gone.
	s, ex, _ := newFixture(300 * time.Second)
	if _, err := s.Token(context.Background(), alice); err != nil {
		t.Fatal(err)
	}
	s.Invalidate(alice.Slot)
	if s.entryCount() != 1 {
		t.Fatal("an invalidated entry must stay so its generation is remembered")
	}

	ex.block = make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = s.Token(context.Background(), alice)
	}()
	// waitForExchange waits for the first call; this is the second.
	for deadline := time.Now().Add(2 * time.Second); ex.count() < 2; {
		if time.Now().After(deadline) {
			t.Fatal("the second exchange did not begin within two seconds")
		}
		time.Sleep(time.Millisecond)
	}
	s.Invalidate(alice.Slot) // a second invalidation, with the generation now non-zero
	close(ex.block)
	<-done

	if s.cachedToken(alice.Slot) != "" {
		t.Fatal("an exchange that started before the second invalidation populated the cache")
	}
}

func TestTwoSubjectsDoNotShareAToken(t *testing.T) {
	s, ex, _ := newFixture(300 * time.Second)
	a, err := s.Token(context.Background(), someone("sub-a"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Token(context.Background(), someone("sub-b"))
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("two subjects were served the same token")
	}
	if ex.count() != 2 {
		t.Fatalf("expected one exchange per subject, got %d", ex.count())
	}
}

func TestOneSubjectAtTwoClustersGetsTwoTokens(t *testing.T) {
	// A shared Grafana serves many clusters, so the key is (audience, subject).
	s, ex, _ := newFixture(300 * time.Second)
	if _, err := s.Token(context.Background(), Principal{Slot: Slot{Audience: "cluster-one", Subject: sub}, SubjectToken: stok}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Token(context.Background(), Principal{Slot: Slot{Audience: "cluster-two", Subject: sub}, SubjectToken: stok}); err != nil {
		t.Fatal(err)
	}
	if ex.count() != 2 {
		t.Fatalf("expected one exchange per cluster, got %d", ex.count())
	}
}

func TestDeadlineIsStampedFromTheRequestStart(t *testing.T) {
	// The wire gives a duration; the deadline must be stamped from when the
	// request STARTED, so it sits at or before the console's true expiry.
	ex := &fakeExchanger{lifetime: 300 * time.Second}
	ck := newClock()
	s := NewSource(ex, ck.now, nil)
	ex.block = make(chan struct{})

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = s.Token(context.Background(), alice)
	}()
	waitForExchange(t, ex)
	ck.advance(10 * time.Second) // the exchange took ten seconds
	close(ex.block)
	<-done

	// Deadline = start + 300 s, not completion + 300 s.
	if got, want := s.deadlineFor(alice.Slot), ck.now().Add(290*time.Second); !got.Equal(want) {
		t.Fatalf("deadline %v, want %v (stamped from the request start)", got, want)
	}
}

type recordingLogger struct {
	mu    sync.Mutex
	lines []string
}

func (r *recordingLogger) record(msg string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines = append(r.lines, fmt.Sprint(append([]any{msg}, args...)...))
}
func (r *recordingLogger) Debug(msg string, args ...any) { r.record(msg, args...) }
func (r *recordingLogger) Warn(msg string, args ...any)  { r.record(msg, args...) }
func (r *recordingLogger) all() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return strings.Join(r.lines, "\n")
}

func TestNoTokenOrSubjectTokenReachesALogLine(t *testing.T) {
	ex := &fakeExchanger{lifetime: 300 * time.Second}
	ck := newClock()
	log := &recordingLogger{}
	s := NewSource(ex, ck.now, log)

	if _, err := s.Token(context.Background(), alice); err != nil {
		t.Fatal(err)
	}
	ck.advance(241 * time.Second)
	ex.err = ErrUnavailable
	_, _ = s.Token(context.Background(), alice)
	ex.err = fmt.Errorf("%w: access_denied", ErrRefused)
	s.Invalidate(alice.Slot)
	_, _ = s.Token(context.Background(), alice)
	_, _ = s.Token(context.Background(), alice)

	rendered := log.all()
	for _, forbidden := range []string{stok, "cluster-token-1", "cluster-token-2", sub} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("a log line leaked %q:\n%s", forbidden, rendered)
		}
	}
	if !strings.Contains(rendered, truncate(sub)) {
		t.Fatalf("expected a truncated subject in the logs:\n%s", rendered)
	}
}

// ---------------------------------------------------------------- cancellation
//
// Raised in review of #219. Thirty panels opening one dashboard coalesce into a
// single exchange, and `singleflight.Do` runs that exchange on the context of
// whichever caller happened to arrive first. The real exchanger binds its HTTP
// request to that context (`client.go`, http.NewRequestWithContext), so the
// leader walking away — a panel refresh, a navigation, a closed dashboard, all
// routine in Grafana — cancels work the other twenty-nine are still waiting on.

// cancellableExchanger respects its context, as the HTTP exchanger does. The
// fake above ignores ctx, which is why this bug could not surface there.
type cancellableExchanger struct {
	started chan struct{} // closed once an exchange is under way
	release chan struct{} // closed to let the exchange finish
	calls   int32
	once    sync.Once
}

func (c *cancellableExchanger) count() int { return int(atomic.LoadInt32(&c.calls)) }

func (c *cancellableExchanger) Exchange(ctx context.Context, audience, subjectToken string) (string, time.Duration, error) {
	atomic.AddInt32(&c.calls, 1)
	c.once.Do(func() { close(c.started) })
	select {
	case <-c.release:
		return "cluster-token-1", time.Hour, nil
	case <-ctx.Done():
		return "", 0, ctx.Err()
	}
}

func TestOneCallerLeavingDoesNotCancelTheOthers(t *testing.T) {
	ex := &cancellableExchanger{started: make(chan struct{}), release: make(chan struct{})}
	ck := newClock()
	s := NewSource(ex, ck.now, nil)

	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	leaderDone := make(chan error, 1)
	go func() {
		_, err := s.Token(leaderCtx, alice)
		leaderDone <- err
	}()

	<-ex.started // the leader owns the flight

	waiterDone := make(chan struct {
		tok string
		err error
	}, 1)
	go func() {
		tok, err := s.Token(context.Background(), alice)
		waiterDone <- struct {
			tok string
			err error
		}{tok, err}
	}()

	// Let the waiter join the flight rather than start its own.
	time.Sleep(50 * time.Millisecond)
	if ex.count() != 1 {
		t.Fatalf("expected the two callers to share one exchange, saw %d", ex.count())
	}

	cancelLeader()    // the leader's panel goes away
	close(ex.release) // whatever is still in flight may now finish

	if err := <-leaderDone; err == nil {
		t.Log("leader returned without error, which is fine; it is the waiter that matters")
	}

	got := <-waiterDone
	if got.err != nil {
		t.Fatalf("a caller that never cancelled was failed by another caller leaving: %v", got.err)
	}
	if got.tok == "" {
		t.Fatal("the surviving caller got no token")
	}

	// And the walk-away must not be recorded as the console failing: that would
	// put the key on a backoff ladder, so ordinary Grafana navigation would
	// degrade every later request for this person and cluster.
	attempts, nextTry := s.retryStateFor(alice.Slot)
	if attempts != 0 || !nextTry.IsZero() {
		t.Fatalf("a cancellation was recorded as a failure: attempts=%d nextTry=%v", attempts, nextTry)
	}
}

// The subject claim keys the cache without being verified, so the key alone
// must not decide who is served. An entry is answered only to the forwarded
// token that minted it: that is the string the console checked.

func TestARefreshedSubjectTokenIsExchangedAfresh(t *testing.T) {
	s, ex, ck := newFixture(3600 * time.Second)
	first, err := s.Token(context.Background(), alice)
	if err != nil {
		t.Fatal(err)
	}
	ck.advance(10 * time.Second) // fresh by every clock
	const refreshed = "the.refreshed.token"
	got, err := s.Token(context.Background(), presenting(refreshed))
	if err != nil {
		t.Fatal(err)
	}
	if ex.count() != 2 || got == first {
		t.Fatalf("a token the console never saw must be exchanged: calls=%d token=%q", ex.count(), got)
	}
	// The refreshed token now owns the slot and is served from it.
	if again, _ := s.Token(context.Background(), presenting(refreshed)); again != got || ex.count() != 2 {
		t.Fatalf("expected the cache to serve the refreshed token: calls=%d token=%q", ex.count(), again)
	}
	// The superseded token is no longer the one the entry was minted from.
	if _, err := s.Token(context.Background(), alice); err != nil || ex.count() != 3 {
		t.Fatalf("expected the old token to be exchanged again: calls=%d err=%v", ex.count(), err)
	}
}

func TestAnUnverifiedSubjectTokenIsNeverServedTheTokenInHand(t *testing.T) {
	// The transient-failure path serves the token in hand. That token was
	// minted for another string: for this one, nothing has been verified, so
	// the failure is the answer.
	s, ex, ck := newFixture(300 * time.Second)
	if _, err := s.Token(context.Background(), alice); err != nil {
		t.Fatal(err)
	}
	ck.advance(241 * time.Second) // stale but usable for stok
	ex.err = fmt.Errorf("%w: dial", ErrUnavailable)
	got, err := s.Token(context.Background(), presenting("the.refreshed.token"))
	if !errors.Is(err, ErrUnavailable) || got != "" {
		t.Fatalf("expected a closed failure for an unverified token, got %q, %v", got, err)
	}
}

func TestTwoSubjectTokensInFlightMakeTwoExchanges(t *testing.T) {
	// A dashboard load straddling a Grafana refresh: panels on each token
	// coalesce among themselves, never across, so nobody receives a token
	// minted from a string they did not present.
	s, ex, _ := newFixture(300 * time.Second)
	ex.block = make(chan struct{})
	var wg sync.WaitGroup
	tokens := make([]string, 4)
	for i := range tokens {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			subjectToken := stok
			if i%2 == 1 {
				subjectToken = "the.refreshed.token"
			}
			tokens[i], _ = s.Token(context.Background(), presenting(subjectToken))
		}(i)
	}
	deadline := time.Now().Add(2 * time.Second)
	for ex.count() < 2 {
		if time.Now().After(deadline) {
			t.Fatalf("expected two flights, saw %d", ex.count())
		}
		time.Sleep(time.Millisecond)
	}
	close(ex.block)
	wg.Wait()
	if ex.count() != 2 {
		t.Fatalf("expected exactly one exchange per token, got %d", ex.count())
	}
	if tokens[0] != tokens[2] || tokens[1] != tokens[3] || tokens[0] == tokens[1] {
		t.Fatalf("callers must share a result only with callers on the same token: %q", tokens)
	}
}
