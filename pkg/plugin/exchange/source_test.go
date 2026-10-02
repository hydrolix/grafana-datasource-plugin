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
			got[i], errs[i] = s.Token(context.Background(), aud, sub, stok)
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
	if _, err := s.Token(context.Background(), aud, sub, stok); err != nil {
		t.Fatal(err)
	}
	// Well inside the lead window (300 s ⇒ lead 60 s ⇒ stale at 240 s).
	ck.advance(100 * time.Second)
	for i := 0; i < 5; i++ {
		if _, err := s.Token(context.Background(), aud, sub, stok); err != nil {
			t.Fatal(err)
		}
	}
	if ex.count() != 1 {
		t.Fatalf("expected the cache to serve, got %d exchanges", ex.count())
	}
}

func TestRefreshAheadOfExpiry(t *testing.T) {
	s, ex, ck := newFixture(300 * time.Second) // lead = max(60s, 20% of 300s) = 60s
	if _, err := s.Token(context.Background(), aud, sub, stok); err != nil {
		t.Fatal(err)
	}
	ck.advance(239 * time.Second) // not yet stale
	if _, _ = s.Token(context.Background(), aud, sub, stok); ex.count() != 1 {
		t.Fatalf("refreshed too early: %d exchanges", ex.count())
	}
	ck.advance(2 * time.Second) // now inside the lead window
	tok, err := s.Token(context.Background(), aud, sub, stok)
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
	if _, err := s.Token(context.Background(), aud, sub, stok); err != nil {
		t.Fatal(err)
	}
	ck.advance(50 * time.Second)
	if _, _ = s.Token(context.Background(), aud, sub, stok); ex.count() != 1 {
		t.Fatalf("pre-refreshed a short token: %d exchanges", ex.count())
	}
	ck.advance(15 * time.Second) // now inside the 30 s skew ⇒ unusable
	if _, err := s.Token(context.Background(), aud, sub, stok); err != nil {
		t.Fatal(err)
	}
	if ex.count() != 2 {
		t.Fatalf("expected a lazy re-mint once inside the skew, got %d", ex.count())
	}
}

func TestAFailedRefreshKeepsServingTheTokenInHand(t *testing.T) {
	s, ex, ck := newFixture(300 * time.Second)
	first, err := s.Token(context.Background(), aud, sub, stok)
	if err != nil {
		t.Fatal(err)
	}
	ck.advance(241 * time.Second) // stale, still usable
	ex.err = ErrUnavailable

	tok, err := s.Token(context.Background(), aud, sub, stok)
	if err != nil {
		t.Fatalf("a failed refresh must be invisible while the token is usable: %v", err)
	}
	if tok != first {
		t.Fatalf("expected the token in hand, got %q", tok)
	}
}

func TestTheRetryLadderBacksOff(t *testing.T) {
	s, ex, ck := newFixture(3600 * time.Second) // lead 300 s
	if _, err := s.Token(context.Background(), aud, sub, stok); err != nil {
		t.Fatal(err)
	}
	ck.advance(3400 * time.Second) // stale (300 s lead), usable (30 s skew)
	ex.err = ErrUnavailable

	if _, _ = s.Token(context.Background(), aud, sub, stok); ex.count() != 2 {
		t.Fatalf("expected one refresh attempt, got %d", ex.count())
	}
	// Inside the first 5 s rung: no further attempt.
	ck.advance(3 * time.Second)
	if _, _ = s.Token(context.Background(), aud, sub, stok); ex.count() != 2 {
		t.Fatalf("attempted inside the backoff rung: %d", ex.count())
	}
	// Past it: one more attempt, and the ladder lengthens.
	ck.advance(3 * time.Second)
	if _, _ = s.Token(context.Background(), aud, sub, stok); ex.count() != 3 {
		t.Fatalf("expected a second attempt past the rung, got %d", ex.count())
	}
	ck.advance(6 * time.Second) // inside the 10 s rung
	if _, _ = s.Token(context.Background(), aud, sub, stok); ex.count() != 3 {
		t.Fatalf("the ladder did not lengthen: %d", ex.count())
	}
}

func TestARefusalIsRememberedBriefly(t *testing.T) {
	// A refused person reloading a thirty-panel dashboard must not ask thirty
	// times, and a converging removal must not become a load spike.
	s, ex, ck := newFixture(300 * time.Second)
	ex.err = fmt.Errorf("%w: access_denied", ErrRefused)

	for i := 0; i < 10; i++ {
		_, err := s.Token(context.Background(), aud, sub, stok)
		if !errors.Is(err, ErrRefused) {
			t.Fatalf("call %d: expected a refusal, got %v", i, err)
		}
	}
	if ex.count() != 1 {
		t.Fatalf("expected the refusal to be remembered, got %d exchanges", ex.count())
	}
	ck.advance(refusalMemory + time.Second)
	if _, err := s.Token(context.Background(), aud, sub, stok); !errors.Is(err, ErrRefused) {
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
	if _, err := s.Token(context.Background(), aud, sub, stok); err != nil {
		t.Fatal(err)
	}
	ck.advance(241 * time.Second) // stale but usable
	ex.err = fmt.Errorf("%w: access_denied", ErrRefused)

	// The refresh fails with a refusal: this call still answers from the token
	// in hand, because it was usable when the call began...
	if _, err := s.Token(context.Background(), aud, sub, stok); err != nil {
		t.Fatalf("unexpected error on the refreshing call: %v", err)
	}
	// ...but the token is gone, so the next call is refused rather than served.
	if _, err := s.Token(context.Background(), aud, sub, stok); !errors.Is(err, ErrRefused) {
		t.Fatalf("expected the next call to be refused, got %v", err)
	}
}

func TestInvalidateForcesAFreshExchange(t *testing.T) {
	s, ex, _ := newFixture(300 * time.Second)
	if _, err := s.Token(context.Background(), aud, sub, stok); err != nil {
		t.Fatal(err)
	}
	s.Invalidate(aud, sub)
	tok, err := s.Token(context.Background(), aud, sub, stok)
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
		_, _ = s.Token(context.Background(), aud, sub, stok)
	}()
	waitForExchange(t, ex)
	s.Invalidate(aud, sub) // while the exchange is in flight
	close(ex.block)
	<-done

	if s.cachedToken(aud, sub) != "" {
		t.Fatal("an exchange that started before the invalidation populated the cache")
	}
}

func TestTwoSubjectsDoNotShareAToken(t *testing.T) {
	s, ex, _ := newFixture(300 * time.Second)
	a, err := s.Token(context.Background(), aud, "sub-a", stok)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Token(context.Background(), aud, "sub-b", stok)
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
	if _, err := s.Token(context.Background(), "cluster-one", sub, stok); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Token(context.Background(), "cluster-two", sub, stok); err != nil {
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
		_, _ = s.Token(context.Background(), aud, sub, stok)
	}()
	waitForExchange(t, ex)
	ck.advance(10 * time.Second) // the exchange took ten seconds
	close(ex.block)
	<-done

	// Deadline = start + 300 s, not completion + 300 s.
	if got, want := s.deadlineFor(aud, sub), ck.now().Add(290*time.Second); !got.Equal(want) {
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

	if _, err := s.Token(context.Background(), aud, sub, stok); err != nil {
		t.Fatal(err)
	}
	ck.advance(241 * time.Second)
	ex.err = ErrUnavailable
	_, _ = s.Token(context.Background(), aud, sub, stok)
	ex.err = fmt.Errorf("%w: access_denied", ErrRefused)
	s.Invalidate(aud, sub)
	_, _ = s.Token(context.Background(), aud, sub, stok)
	_, _ = s.Token(context.Background(), aud, sub, stok)

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
