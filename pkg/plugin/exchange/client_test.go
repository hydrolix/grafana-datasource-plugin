package exchange

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func testConfig(serverURL string) Config {
	return Config{
		URL: serverURL,
		Credentials: map[string]Credential{
			aud: {ClientID: "grafana-cluster", ClientSecret: "delegate-secret"},
		},
	}
}

func TestTheExchangeSendsTheStandardForm(t *testing.T) {
	var got url.Values
	var authUser, authPass string
	var authOK bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		got = r.PostForm
		authUser, authPass, authOK = r.BasicAuth()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"ct","token_type":"Bearer","expires_in":300}`))
	}))
	defer srv.Close()

	token, lifetime, err := NewHTTPExchanger(testConfig(srv.URL)).Exchange(context.Background(), aud, stok)
	if err != nil {
		t.Fatal(err)
	}
	if token != "ct" || lifetime != 300*time.Second {
		t.Fatalf("token=%q lifetime=%v", token, lifetime)
	}
	if got.Get("grant_type") != grantType {
		t.Fatalf("grant_type=%q", got.Get("grant_type"))
	}
	if got.Get("subject_token") != stok || got.Get("subject_token_type") != tokenTypeAccess {
		t.Fatalf("subject token fields: %v", got)
	}
	if got.Get("audience") != aud {
		t.Fatalf("audience=%q", got.Get("audience"))
	}
	// The facade refuses a request that carries a scope.
	if _, present := got["scope"]; present {
		t.Fatal("the form carried a scope, which the facade refuses")
	}
	if !authOK || authUser != "grafana-cluster" || authPass != "delegate-secret" {
		t.Fatalf("delegate credential not sent as basic auth (ok=%v user=%q)", authOK, authUser)
	}
}

func TestAnUnknownClusterIsMisconfiguredRatherThanRefused(t *testing.T) {
	// No credential for this audience: an operator has not finished configuring
	// this instance, which is not a statement about anyone's access.
	_, _, err := NewHTTPExchanger(testConfig("http://unused.invalid")).
		Exchange(context.Background(), "some-other-cluster", stok)
	if !errors.Is(err, ErrMisconfigured) {
		t.Fatalf("expected misconfigured, got %v", err)
	}
}

func TestTheFacadeVocabularyMapsOntoTheThreeOutcomes(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"access denied is a refusal", 409, `{"error":"invalid_target","error_description":"access_denied"}`, ErrRefused},
		{"an invalid grant is a refusal", 400, `{"error":"invalid_grant"}`, ErrRefused},
		{"temporarily unavailable is not a refusal of the person", 409,
			`{"error":"temporarily_unavailable","error_description":"role_removal_pending"}`, ErrUnavailable},
		{"a server error is unavailable", 409, `{"error":"server_error"}`, ErrUnavailable},
		{"a 500 is unavailable", 500, `nothing useful`, ErrUnavailable},
		{"a 429 is unavailable", 429, `{"error":"temporarily_unavailable","error_description":"rate_limited"}`, ErrUnavailable},
		{"a rejected delegate credential is the operator's problem", 401, `{"error":"invalid_client"}`, ErrMisconfigured},
		{"a 200 without a token is the console's wiring", 200, `{"token_type":"Bearer"}`, ErrUnavailable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(c.status)
				_, _ = w.Write([]byte(c.body))
			}))
			defer srv.Close()
			_, _, err := NewHTTPExchanger(testConfig(srv.URL)).Exchange(context.Background(), aud, stok)
			if !errors.Is(err, c.want) {
				t.Fatalf("got %v, want %v", err, c.want)
			}
		})
	}
}

func TestAnUnreachableConsoleIsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.Close() // nothing is listening
	_, _, err := NewHTTPExchanger(testConfig(srv.URL)).Exchange(context.Background(), aud, stok)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected unavailable, got %v", err)
	}
}

func TestNoErrorTextCarriesTheSecretOrTheToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(409)
		_, _ = w.Write([]byte(`{"error":"invalid_target","error_description":"access_denied"}`))
	}))
	defer srv.Close()
	_, _, err := NewHTTPExchanger(testConfig(srv.URL)).Exchange(context.Background(), aud, stok)
	msg := err.Error()
	for _, forbidden := range []string{"delegate-secret", stok} {
		if contains(msg, forbidden) {
			t.Fatalf("the error text leaked %q: %s", forbidden, msg)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) &&
		(func() bool {
			for i := 0; i+len(needle) <= len(haystack); i++ {
				if haystack[i:i+len(needle)] == needle {
					return true
				}
			}
			return false
		})()
}

func TestConfigFromEnv(t *testing.T) {
	env := func(m map[string]string) func(string) (string, bool) {
		return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
	}

	t.Run("absent is not configured", func(t *testing.T) {
		if _, err := ConfigFromEnv(env(nil)); !errors.Is(err, ErrNotConfigured) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("half configured is an error, not a fallback", func(t *testing.T) {
		if _, err := ConfigFromEnv(env(map[string]string{EnvURL: "https://console/api"})); err == nil ||
			errors.Is(err, ErrNotConfigured) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("the plain environment names work too", func(t *testing.T) {
		// A backend plugin inherits Grafana's environment, so an operator who
		// can set a variable on the process does not have to touch grafana.ini.
		cfg, err := ConfigFromEnv(env(map[string]string{
			EnvURLAlt:         "https://console/api/v1/auth/token-exchange",
			EnvCredentialsAlt: `{"c1":{"client_id":"grafana-c1","client_secret":"s1"}}`,
		}))
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := cfg.Credential("c1"); !ok {
			t.Fatal("the plain names were not read")
		}
	})
	t.Run("the grafana.ini names win where both are set", func(t *testing.T) {
		cfg, err := ConfigFromEnv(env(map[string]string{
			EnvURL:            "https://configured/api",
			EnvCredentials:    `{"c1":{"client_id":"from-ini","client_secret":"s1"}}`,
			EnvURLAlt:         "https://env/api",
			EnvCredentialsAlt: `{"c1":{"client_id":"from-env","client_secret":"s1"}}`,
		}))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.URL != "https://configured/api" {
			t.Fatalf("url %q", cfg.URL)
		}
		if c, _ := cfg.Credential("c1"); c.ClientID != "from-ini" {
			t.Fatalf("client id %q", c.ClientID)
		}
	})
	t.Run("a credential map is read", func(t *testing.T) {
		cfg, err := ConfigFromEnv(env(map[string]string{
			EnvURL:         "https://console/api/v1/auth/token-exchange",
			EnvCredentials: `{"c1":{"client_id":"grafana-c1","client_secret":"s1"},"c2":{"client_id":"grafana-c2","client_secret":"s2"}}`,
		}))
		if err != nil {
			t.Fatal(err)
		}
		if len(cfg.Credentials) != 2 {
			t.Fatalf("expected two clusters, got %d", len(cfg.Credentials))
		}
		if c, ok := cfg.Credential("c2"); !ok || c.ClientID != "grafana-c2" {
			t.Fatalf("c2 = %+v ok=%v", c, ok)
		}
		if _, ok := cfg.Credential("c3"); ok {
			t.Fatal("an unconfigured cluster answered a credential")
		}
	})
	t.Run("a credential missing its secret is refused", func(t *testing.T) {
		_, err := ConfigFromEnv(env(map[string]string{
			EnvURL:         "https://console/api",
			EnvCredentials: `{"c1":{"client_id":"grafana-c1"}}`,
		}))
		if err == nil {
			t.Fatal("expected an error naming the incomplete credential")
		}
	})
	t.Run("malformed json does not echo its input", func(t *testing.T) {
		_, err := ConfigFromEnv(env(map[string]string{
			EnvURL:         "https://console/api",
			EnvCredentials: `{"c1":{"client_secret":"super-secret-value"`,
		}))
		if err == nil {
			t.Fatal("expected an error")
		}
		if contains(err.Error(), "super-secret-value") {
			t.Fatalf("the error echoed the secret: %v", err)
		}
	})
}

// A body with no OAuth error did not come from the facade (CFB-2612). Found
// live: Django answered the rig's exchange URL with a 400 DisallowedHost, and
// the panel said the console had refused this person access to the cluster —
// which would send an operator to look at permissions rather than at the URL.
func TestAReplyWithNoOAuthErrorIsTheOperatorsProblemNotTheUsers(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"django DisallowedHost", 400, "<h1>Bad Request (400)</h1>"},
		{"an html error page", 404, "<html><body>not found</body></html>"},
		{"an empty body", 400, ""},
		{"json that is not an oauth error", 400, `{"detail":"nope"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := classify(tc.status, []byte(tc.body))
			if errors.Is(err, ErrRefused) {
				t.Fatalf("reported as a refusal of the person: %v", err)
			}
			if !errors.Is(err, ErrMisconfigured) {
				t.Fatalf("want ErrMisconfigured, got %v", err)
			}
		})
	}
}

// The counterpart: a real refusal must still read as one.
func TestAnOAuthErrorIsStillARefusal(t *testing.T) {
	err := classify(400, []byte(`{"error":"invalid_grant","error_description":"access_denied"}`))
	if !errors.Is(err, ErrRefused) {
		t.Fatalf("want ErrRefused, got %v", err)
	}
}
