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
	"net/url"
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

// Configuration is Grafana's own plugin configuration: a
// `[plugin.hydrolix-hydrolix-datasource]` section in its configuration file,
// whose keys Grafana renders into the plugin's environment with a `GF_PLUGIN_`
// prefix. That prefix is the only one Grafana's plugin validator accepts for an
// environment read, and the only one that arrives at all: Grafana builds a
// MINIMAL environment for a backend plugin process rather than passing its own.
// On Grafana 13.0.3 Enterprise the plugin sees `GF_EDITION`, `GF_VERSION`,
// `GF_ENTERPRISE_*`, `PLUGIN_*`, `grafana_plugin_type` and the rendered
// `[plugin.<id>]` keys, and nothing else — so a variable an operator sets on the
// Grafana process, in a container or a systemd unit, never reaches here
// (measured 2026-10-12). 11.6.0 OSS still passes its environment through, which
// is how a prototype can work on an older build and fail on a deployed one.
//
// It is not datasource settings by design: the credentials are per cluster and
// per Grafana instance, and a secret stored on a datasource travels with the
// datasource when its organization is re-bound to another company.
const (
	EnvURL         = "GF_PLUGIN_EXCHANGE_URL"
	EnvCredentials = "GF_PLUGIN_EXCHANGE_CREDENTIALS"
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
	rawURL = strings.TrimSpace(rawURL)
	if !hasURL || rawURL == "" {
		return Config{}, fmt.Errorf("exchange: %s is empty or unset", EnvURL)
	}
	// The exchange sends the sign-in token in its form and the delegate secret
	// in its basic auth, so a plaintext URL leaks both on every exchange. The
	// URL is not quoted in the error: it can carry userinfo.
	if u, err := url.Parse(rawURL); err != nil || u.Scheme != "https" || u.Host == "" {
		return Config{}, fmt.Errorf("exchange: %s must be an https URL: the exchange sends the sign-in token and the delegate secret", EnvURL)
	}
	if !hasCreds || strings.TrimSpace(rawCreds) == "" {
		return Config{}, fmt.Errorf("exchange: %s is empty or unset", EnvCredentials)
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
	if c.Credentials == nil {
		return Credential{}, false
	}
	cred, ok := c.Credentials[audience]
	return cred, ok
}
