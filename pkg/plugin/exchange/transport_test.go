package exchange

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// recordingBase records what reached the wire and answers a scripted status.
type recordingBase struct {
	mu       sync.Mutex
	bearers  []string
	statuses []int
	calls    int
}

func (r *recordingBase) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bearers = append(r.bearers, req.Header.Get("Authorization"))
	status := http.StatusOK
	if r.calls < len(r.statuses) {
		status = r.statuses[r.calls]
	}
	r.calls++
	return &http.Response{
		StatusCode: status,
		Body:       http.NoBody,
		Header:     http.Header{},
		Request:    req,
	}, nil
}

func (r *recordingBase) seen() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.bearers...)
}

func principalCtx() context.Context {
	return WithPrincipal(context.Background(), Principal{
		Audience: aud, Subject: sub, SubjectToken: stok,
	})
}

func newRequest(t *testing.T, ctx context.Context) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://cluster.example/query",
		strings.NewReader("SELECT 1"))
	if err != nil {
		t.Fatal(err)
	}
	return req
}

func TestTheExchangedTokenIsAttachedAndTheForwardedOneIsNot(t *testing.T) {
	s, _, _ := newFixture(300 * time.Second)
	base := &recordingBase{}
	tr := NewTransport(base, s)

	resp, err := tr.RoundTrip(newRequest(t, principalCtx()))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	seen := base.seen()
	if len(seen) != 1 || seen[0] != "Bearer cluster-token-1" {
		t.Fatalf("authorization sent: %v", seen)
	}
	// The invariant: the console-realm token must never reach a cluster.
	for _, h := range seen {
		if strings.Contains(h, stok) {
			t.Fatalf("the forwarded token reached the cluster: %q", h)
		}
	}
}

func TestARequestWithNoPrincipalIsRefusedAndNeverReachesTheCluster(t *testing.T) {
	s, _, _ := newFixture(300 * time.Second)
	base := &recordingBase{}
	tr := NewTransport(base, s)

	_, err := tr.RoundTrip(newRequest(t, context.Background()))
	if !errors.Is(err, ErrRefused) {
		t.Fatalf("expected a refusal, got %v", err)
	}
	if len(base.seen()) != 0 {
		t.Fatalf("a request with no identity reached the cluster: %v", base.seen())
	}
}

func TestAFailedExchangeNeverFallsBackToTheForwardedToken(t *testing.T) {
	// The whole lane rests on this: if the exchange fails, the query fails.
	ex := &fakeExchanger{lifetime: 300 * time.Second, err: ErrUnavailable}
	s := NewSource(ex, nil, nil)
	base := &recordingBase{}
	tr := NewTransport(base, s)

	_, err := tr.RoundTrip(newRequest(t, principalCtx()))
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected unavailable, got %v", err)
	}
	if len(base.seen()) != 0 {
		t.Fatalf("a request went out despite a failed exchange: %v", base.seen())
	}
}

func TestAClusterUnauthorizedReMintsOnceAndRetries(t *testing.T) {
	s, ex, _ := newFixture(300 * time.Second)
	base := &recordingBase{statuses: []int{http.StatusUnauthorized, http.StatusOK}}
	tr := NewTransport(base, s)

	resp, err := tr.RoundTrip(newRequest(t, principalCtx()))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected the retry to succeed, got %d", resp.StatusCode)
	}
	if ex.count() != 2 {
		t.Fatalf("expected one re-mint after the 401, got %d exchanges", ex.count())
	}
	seen := base.seen()
	if len(seen) != 2 || seen[0] == seen[1] {
		t.Fatalf("expected a retry with a different token, got %v", seen)
	}
}

func TestASecondUnauthorizedIsTheAnswer(t *testing.T) {
	// One re-mint, not a loop.
	s, ex, _ := newFixture(300 * time.Second)
	base := &recordingBase{statuses: []int{http.StatusUnauthorized, http.StatusUnauthorized}}
	tr := NewTransport(base, s)

	resp, err := tr.RoundTrip(newRequest(t, principalCtx()))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected the cluster's 401 to reach the caller, got %d", resp.StatusCode)
	}
	if ex.count() != 2 {
		t.Fatalf("expected exactly one re-mint, got %d exchanges", ex.count())
	}
	if len(base.seen()) != 2 {
		t.Fatalf("expected exactly two attempts, got %d", len(base.seen()))
	}
}

func TestAnUnauthorizedWhoseReMintFailsReturnsTheClustersAnswer(t *testing.T) {
	ex := &fakeExchanger{lifetime: 300 * time.Second}
	s := NewSource(ex, nil, nil)
	base := &recordingBase{statuses: []int{http.StatusUnauthorized}}
	tr := NewTransport(base, s)

	// First exchange succeeds, then the console stops answering.
	if _, err := s.Token(context.Background(), aud, sub, stok); err != nil {
		t.Fatal(err)
	}
	ex.mu.Lock()
	ex.err = ErrUnavailable
	ex.mu.Unlock()

	resp, err := tr.RoundTrip(newRequest(t, principalCtx()))
	if err != nil {
		t.Fatalf("expected the cluster's answer rather than an error: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestTheRequestGivenToRoundTripIsNotMutated(t *testing.T) {
	s, _, _ := newFixture(300 * time.Second)
	tr := NewTransport(&recordingBase{}, s)
	req := newRequest(t, principalCtx())
	req.Header.Set("Authorization", "Bearer something-the-caller-set")

	if _, err := tr.RoundTrip(req); err != nil {
		t.Fatal(err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer something-the-caller-set" {
		t.Fatalf("the caller's request was mutated: %q", got)
	}
}

func TestSubjectOfReadsTheClaimWithoutVerifying(t *testing.T) {
	// A real token's middle segment, unsigned: the facade is the verifier.
	payload := `{"sub":"kc-sub-alice","aud":"grafana-api"}`
	jwt := "header." + base64url(payload) + ".signature"
	if got := SubjectOf(jwt); got != "kc-sub-alice" {
		t.Fatalf("subject %q", got)
	}
	for _, bad := range []string{"", "not-a-jwt", "a.b", "a.!!!.c"} {
		if got := SubjectOf(bad); got != "" {
			t.Fatalf("%q yielded a subject %q", bad, got)
		}
	}
}

func base64url(s string) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	out := make([]byte, 0, len(s)*2)
	for i := 0; i < len(s); i += 3 {
		var chunk [3]byte
		n := copy(chunk[:], s[i:])
		out = append(out, alphabet[chunk[0]>>2])
		out = append(out, alphabet[(chunk[0]&0x03)<<4|chunk[1]>>4])
		if n > 1 {
			out = append(out, alphabet[(chunk[1]&0x0f)<<2|chunk[2]>>6])
		}
		if n > 2 {
			out = append(out, alphabet[chunk[2]&0x3f])
		}
	}
	return string(out)
}

func TestTheTransportWorksAgainstARealServer(t *testing.T) {
	// End to end through net/http rather than a fake round tripper, so the
	// header really is on the wire and the context really does survive.
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
		fmt.Fprintln(w, "42")
	}))
	defer srv.Close()

	s, _, _ := newFixture(300 * time.Second)
	client := &http.Client{Transport: NewTransport(http.DefaultTransport, s)}
	req, err := http.NewRequestWithContext(principalCtx(), http.MethodPost, srv.URL, strings.NewReader("SELECT 1"))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if got != "Bearer cluster-token-1" {
		t.Fatalf("the wire carried %q", got)
	}
}
