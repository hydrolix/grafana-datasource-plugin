// Package exchange turns a Grafana sign-in token into a cluster token.
//
// A Hydrolix Console customer signs in to Grafana through the console's
// Keycloak realm, so the token Grafana forwards carries the audience of the
// console's Grafana OAuth client. A cluster refuses it: it accepts a token from
// an external issuer only when the token's `aud` holds the audience that
// cluster configures for that issuer. The console exposes an RFC 8693
// token-exchange endpoint for exactly this, and this package calls it.
//
// The semantics here are a port of the console's own browser client
// (`packages/cluster-client/src/session/ClusterSessionManager.ts` in
// hydrolix/hydrolix-console), whose behaviour was shaped by incidents this
// package has no wish to repeat: deadlines stamped on the client clock from the
// moment a request started, refresh ahead of expiry in proportion to lifetime,
// one exchange in flight per user, a bounded retry ladder, and an invalidation
// that an in-flight exchange cannot undo.
package exchange

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Credential is one cluster's delegate: the console issues it, and its
// allowlist holds that one deployment.
type Credential struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// Config is what an operator puts in Grafana's server configuration, never in
// datasource settings: these are per-cluster operator values, and a forward-mode
// datasource is meant to hold no secret at all.
//
// Credentials are a MAP keyed by cluster audience rather than one pair. A single
// Grafana can serve many clusters — in the console's fleet most legacy
// organizations sit on a handful of shared instances — and each cluster has its
// own delegate. Keying by audience keeps each secret scoped to the one cluster
// it can mint for, which is the property the console's delegate registry exists
// to preserve.
type Config struct {
	// URL is the console's token-exchange endpoint.
	URL string
	// Credentials maps a cluster's audience to its delegate credential.
	Credentials map[string]Credential
}

// Env keys, read from Grafana's `[plugin.hydrolix-hydrolix-datasource]` section,
// which Grafana exposes to the plugin process as environment variables.
const (
	EnvURL         = "HDX_EXCHANGE_URL"
	EnvCredentials = "HDX_EXCHANGE_CREDENTIALS"
)

// ErrNotConfigured means this instance has no exchange configuration at all, so
// the exchanging credentials mode is unavailable here. It is not a failure of a
// request: a datasource configured for the mode should fail its health check
// saying what is missing, rather than failing a panel with an auth error.
var ErrNotConfigured = errors.New("exchange: not configured")

// ConfigFromEnv reads the configuration from a lookup function (os.LookupEnv in
// production, a map in tests). It returns ErrNotConfigured when neither key is
// set, and a descriptive error when one is set and unusable — a half-configured
// instance is an operator mistake worth naming, not a silent fallback.
func ConfigFromEnv(lookup func(string) (string, bool)) (Config, error) {
	rawURL, hasURL := lookup(EnvURL)
	rawCreds, hasCreds := lookup(EnvCredentials)
	if !hasURL && !hasCreds {
		return Config{}, ErrNotConfigured
	}
	if strings.TrimSpace(rawURL) == "" {
		return Config{}, fmt.Errorf("exchange: %s is empty", EnvURL)
	}
	if strings.TrimSpace(rawCreds) == "" {
		return Config{}, fmt.Errorf("exchange: %s is empty", EnvCredentials)
	}
	var creds map[string]Credential
	if err := json.Unmarshal([]byte(rawCreds), &creds); err != nil {
		// The error is not wrapped: a JSON decoding error can quote the input,
		// and the input holds client secrets.
		return Config{}, fmt.Errorf("exchange: %s is not a JSON object of audience to credential", EnvCredentials)
	}
	if len(creds) == 0 {
		return Config{}, fmt.Errorf("exchange: %s names no cluster", EnvCredentials)
	}
	for audience, c := range creds {
		if c.ClientID == "" || c.ClientSecret == "" {
			return Config{}, fmt.Errorf("exchange: the credential for audience %q is missing its client id or secret", audience)
		}
	}
	return Config{URL: rawURL, Credentials: creds}, nil
}

// Credential answers the delegate for a cluster audience.
func (c Config) Credential(audience string) (Credential, bool) {
	cred, ok := c.Credentials[audience]
	return cred, ok
}
