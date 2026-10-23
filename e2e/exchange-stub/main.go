// Command exchange-stub stands in for the two parties the exchanging
// credentials mode talks to and which the dev stack does not have: the
// console's RFC 8693 token-exchange facade, and a Hydrolix cluster that takes
// a bearer token. It is an e2e fixture, not a reference implementation: it
// mints opaque tokens, verifies nothing about the sign-in token beyond
// reading its subject, and keeps everything in memory.
//
// Plain HTTP (STUB_HTTP_ADDR, default :9080): the test's control surface only.
//
//	GET  /e2e/events         everything it saw, in order, as JSON
//	POST /e2e/reset          forget events, minted tokens and the refusal
//	POST /e2e/revoke         every minted token now answers 401 at the cluster
//	POST /e2e/refuse?on=1|0  while on, the exchange refuses every subject with
//	                         409 invalid_target / access_denied
//
// TLS (STUB_TLS_ADDR, default :9443): what the plugin talks to. The plugin
// refuses both over anything but verified TLS, so the dev stack trusts this
// listener's certificate (see certificate).
//
//	POST /token              the exchange: delegate basic auth and the standard
//	                         form in, an opaque cluster token out
//	anything else            the cluster: a request carrying a bearer this stub
//	                         minted is forwarded to ClickHouse with the fixture
//	                         user's basic auth; anything else is a 401 in
//	                         ClickHouse's own words. A bearer that is one of
//	                         the SIGN-IN tokens seen as a subject_token is
//	                         recorded as auth "sign-in": that is the invariant
//	                         the mode exists to keep, and a test asserts it
//	                         never appears.
package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const grantTokenExchange = "urn:ietf:params:oauth:grant-type:token-exchange"

// event is one thing the stub saw. Tests read the list and assert on it.
type event struct {
	// Kind is "exchange" or "cluster".
	Kind string `json:"kind"`
	// Subject is the sign-in token's `sub`, when the stub could tell.
	Subject string `json:"subject,omitempty"`
	// Auth classifies a cluster request's credential: minted, revoked,
	// expired, sign-in, none or unknown.
	Auth   string `json:"auth,omitempty"`
	Status int    `json:"status"`
	// Reason is the OAuth error code of a failed exchange.
	Reason string `json:"reason,omitempty"`
	Path   string `json:"path,omitempty"`
}

type mint struct {
	subject string
	expires time.Time
	revoked bool
}

type stub struct {
	clientID, clientSecret, audience string
	ttl                              time.Duration
	upstreamUser, upstreamPassword   string
	proxy                            *httputil.ReverseProxy

	mu     sync.Mutex
	events []event
	minted map[string]*mint
	signIn map[string]string // sign-in token -> subject
	refuse bool
}

func main() {
	upstream, err := url.Parse(env("STUB_UPSTREAM", "http://clickhouse-server:8123"))
	if err != nil {
		log.Fatalf("STUB_UPSTREAM: %v", err)
	}
	ttl, err := time.ParseDuration(env("STUB_TOKEN_TTL", "300s"))
	if err != nil {
		log.Fatalf("STUB_TOKEN_TTL: %v", err)
	}
	s := &stub{
		clientID:         env("STUB_CLIENT_ID", "grafana-e2e"),
		clientSecret:     env("STUB_CLIENT_SECRET", "e2e-delegate-secret"),
		audience:         env("STUB_AUDIENCE", "exchange-stub"),
		ttl:              ttl,
		upstreamUser:     env("STUB_UPSTREAM_USER", "testuser"),
		upstreamPassword: env("STUB_UPSTREAM_PASSWORD", "testpass"),
		events:           []event{},
		minted:           map[string]*mint{},
		signIn:           map[string]string{},
	}
	s.proxy = s.clusterProxy(upstream)

	plain := http.NewServeMux()
	plain.HandleFunc("/e2e/events", s.listEvents)
	plain.HandleFunc("/e2e/reset", s.reset)
	plain.HandleFunc("/e2e/revoke", s.revoke)
	plain.HandleFunc("/e2e/refuse", s.setRefuse)

	hosts := strings.Split(env("STUB_TLS_HOSTS", "exchange-stub,localhost,127.0.0.1"), ",")
	cert, err := certificate(os.Getenv("STUB_CERT_DIR"), hosts)
	if err != nil {
		log.Fatalf("certificate: %v", err)
	}
	secure := http.NewServeMux()
	secure.HandleFunc("/token", s.token)
	secure.HandleFunc("/", s.cluster)
	cluster := &http.Server{
		Addr:        env("STUB_TLS_ADDR", ":9443"),
		Handler:     secure,
		TLSConfig:   &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12},
		ReadTimeout: 5 * time.Second,
	}
	go func() {
		log.Printf("exchange and cluster (TLS) on %s, forwarding to %s", cluster.Addr, upstream)
		log.Fatal(cluster.ListenAndServeTLS("", ""))
	}()
	addr := env("STUB_HTTP_ADDR", ":9080")
	log.Printf("e2e control on %s, audience %q, client %q, token ttl %s", addr, s.audience, s.clientID, s.ttl)
	log.Fatal(http.ListenAndServe(addr, plain))
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// --- the exchange ----------------------------------------------------------

func (s *stub) token(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.oauthError(w, http.StatusMethodNotAllowed, "invalid_request", "POST only", "")
		return
	}
	id, secret, ok := r.BasicAuth()
	if !ok || id != s.clientID || secret != s.clientSecret {
		s.oauthError(w, http.StatusUnauthorized, "invalid_client", "", "")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.oauthError(w, http.StatusBadRequest, "invalid_request", "unreadable form", "")
		return
	}
	form := r.PostForm
	subjectToken := form.Get("subject_token")
	subject := subjectOf(subjectToken)
	if subjectToken != "" {
		// Remembered BEFORE any refusal: a refused flow that then forwards
		// the sign-in token to the cluster is exactly what must be caught.
		s.mu.Lock()
		s.signIn[subjectToken] = subject
		s.mu.Unlock()
	}
	switch {
	case form.Get("grant_type") != grantTokenExchange:
		s.oauthError(w, http.StatusBadRequest, "unsupported_grant_type", "", subject)
		return
	case subjectToken == "":
		s.oauthError(w, http.StatusBadRequest, "invalid_request", "subject_token is required", subject)
		return
	case form.Get("audience") != s.audience:
		s.oauthError(w, http.StatusBadRequest, "invalid_target", "unknown audience "+form.Get("audience"), subject)
		return
	}

	s.mu.Lock()
	refuse := s.refuse
	s.mu.Unlock()
	if refuse {
		s.oauthError(w, http.StatusConflict, "invalid_target", "access_denied", subject)
		return
	}

	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		s.oauthError(w, http.StatusInternalServerError, "server_error", "", subject)
		return
	}
	tok := "hdx-e2e-" + hex.EncodeToString(raw[:])
	s.mu.Lock()
	s.minted[tok] = &mint{subject: subject, expires: time.Now().Add(s.ttl)}
	s.mu.Unlock()
	s.record(event{Kind: "exchange", Subject: subject, Status: http.StatusOK})

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":      tok,
		"issued_token_type": "urn:ietf:params:oauth:token-type:access_token",
		"token_type":        "Bearer",
		"expires_in":        int(s.ttl.Seconds()),
	})
}

func (s *stub) oauthError(w http.ResponseWriter, status int, code, description, subject string) {
	s.record(event{Kind: "exchange", Subject: subject, Status: status, Reason: code})
	body := map[string]string{"error": code}
	if description != "" {
		body["error_description"] = description
	}
	writeJSON(w, status, body)
}

// subjectOf reads `sub` out of a JWT without verifying it, which is all the
// facade stand-in needs: the test asserts on who asked, not on whether the
// signature holds.
func subjectOf(jwt string) string {
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
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

// --- the cluster -----------------------------------------------------------

type authKey struct{}

type clusterAuth struct{ auth, subject string }

func (s *stub) cluster(w http.ResponseWriter, r *http.Request) {
	auth, subject, ok := s.classify(r.Header.Get("Authorization"))
	if !ok {
		s.record(event{Kind: "cluster", Auth: auth, Subject: subject, Status: http.StatusUnauthorized, Path: r.URL.Path})
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		w.Header().Set("X-ClickHouse-Exception-Code", "516")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprintf(w, "Code: 516. DB::Exception: Authentication failed: the cluster does not accept this credential (%s). (AUTHENTICATION_FAILED)\n", auth)
		return
	}
	ctx := context.WithValue(r.Context(), authKey{}, clusterAuth{auth: auth, subject: subject})
	s.proxy.ServeHTTP(w, r.WithContext(ctx))
}

func (s *stub) classify(header string) (auth, subject string, ok bool) {
	switch {
	case header == "":
		return "none", "", false
	case !strings.HasPrefix(header, "Bearer "):
		return "unknown", "", false
	}
	tok := strings.TrimPrefix(header, "Bearer ")
	s.mu.Lock()
	defer s.mu.Unlock()
	if sub, seen := s.signIn[tok]; seen {
		return "sign-in", sub, false
	}
	m, minted := s.minted[tok]
	switch {
	case !minted:
		return "unknown", "", false
	case m.revoked:
		return "revoked", m.subject, false
	case time.Now().After(m.expires):
		return "expired", m.subject, false
	}
	return "minted", m.subject, true
}

func (s *stub) clusterProxy(upstream *url.URL) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(upstream)
			pr.Out.Host = upstream.Host
			// A Hydrolix cluster answers queries at /query (the config
			// editor's default HTTP path); ClickHouse answers at /. The
			// query string, which carries the settings, is kept.
			pr.Out.URL.Path = "/"
			pr.Out.URL.RawPath = ""
			// The bearer was for this stub; ClickHouse gets the fixture user.
			pr.Out.Header.Del("Authorization")
			pr.Out.SetBasicAuth(s.upstreamUser, s.upstreamPassword)
		},
		ModifyResponse: func(resp *http.Response) error {
			a, _ := resp.Request.Context().Value(authKey{}).(clusterAuth)
			s.record(event{Kind: "cluster", Auth: a.auth, Subject: a.subject, Status: resp.StatusCode, Path: resp.Request.URL.Path})
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			a, _ := r.Context().Value(authKey{}).(clusterAuth)
			s.record(event{Kind: "cluster", Auth: a.auth, Subject: a.subject, Status: http.StatusBadGateway, Path: r.URL.Path, Reason: "upstream: " + err.Error()})
			http.Error(w, "upstream: "+err.Error(), http.StatusBadGateway)
		},
	}
}

// --- e2e control -----------------------------------------------------------

func (s *stub) record(e event) {
	s.mu.Lock()
	s.events = append(s.events, e)
	s.mu.Unlock()
	// Subjects and classifications only: never a token.
	log.Printf("%s subject=%q auth=%q status=%d reason=%q path=%q", e.Kind, e.Subject, e.Auth, e.Status, e.Reason, e.Path)
}

func (s *stub) listEvents(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	out := append([]event{}, s.events...)
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, out)
}

func (s *stub) reset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	s.events = []event{}
	s.minted = map[string]*mint{}
	s.signIn = map[string]string{}
	s.refuse = false
	s.mu.Unlock()
	log.Print("reset")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *stub) revoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	n := 0
	for _, m := range s.minted {
		if !m.revoked {
			m.revoked = true
			n++
		}
	}
	s.mu.Unlock()
	log.Printf("revoked %d token(s)", n)
	writeJSON(w, http.StatusOK, map[string]any{"revoked": n})
}

func (s *stub) setRefuse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	on := r.URL.Query().Get("on")
	s.mu.Lock()
	s.refuse = on == "1" || on == "true"
	state := s.refuse
	s.mu.Unlock()
	log.Printf("refuse=%v", state)
	writeJSON(w, http.StatusOK, map[string]any{"refuse": state})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// certificate answers the TLS listener's certificate.
//
// With STUB_CERT_DIR set it lives there as cert.pem and key.pem, and is reused
// while it covers every host and is not about to expire. The dev stack mounts
// cert.pem into Grafana's trust store, because the plugin refuses an
// unverified certificate for both the exchange and the cluster; and Go reads
// that store once per process, so a certificate remade on every start would
// lose the trust of a plugin process that outlived a restart of the stub.
// Without the variable it is made in memory and nothing can trust it.
func certificate(dir string, hosts []string) (tls.Certificate, error) {
	if dir == "" {
		cert, _, _, err := selfSigned(hosts)
		return cert, err
	}
	certPath, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	if cert, err := tls.LoadX509KeyPair(certPath, keyPath); err == nil && stillServes(cert, hosts) {
		log.Printf("certificate: reusing %s", certPath)
		return cert, nil
	}
	cert, certPEM, keyPEM, err := selfSigned(hosts)
	if err != nil {
		return tls.Certificate{}, err
	}
	// The key first: a cert.pem on disk means the pair is complete.
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return tls.Certificate{}, err
	}
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		return tls.Certificate{}, err
	}
	log.Printf("certificate: made %s", certPath)
	return cert, nil
}

// stillServes reports whether a kept certificate names every host and has
// more than a day left.
func stillServes(cert tls.Certificate, hosts []string) bool {
	if len(cert.Certificate) == 0 {
		return false
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil || time.Now().Add(24*time.Hour).After(leaf.NotAfter) {
		return false
	}
	for _, h := range hosts {
		if h = strings.TrimSpace(h); h != "" && leaf.VerifyHostname(h) != nil {
			return false
		}
	}
	return true
}

// selfSigned makes a certificate for the TLS listener, and its PEM encoding.
// It is its own root: the stack trusts this one certificate, not an issuer.
func selfSigned(hosts []string) (cert tls.Certificate, certPEM, keyPEM []byte, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, nil, nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, nil, nil, err
	}
	tmpl := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: strings.TrimSpace(hosts[0]), Organization: []string{"hydrolix e2e"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	for _, h := range hosts {
		h = strings.TrimSpace(h)
		if ip := net.ParseIP(h); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else if h != "" {
			tmpl.DNSNames = append(tmpl.DNSNames, h)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, nil, nil, err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return tls.Certificate{}, nil, nil, err
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, certPEM, keyPEM, nil
}
