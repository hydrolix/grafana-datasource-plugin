package plugin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/hydrolix/plugin/pkg/plugin/exchange"
	"github.com/hydrolix/plugin/pkg/plugin/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// jwtWithSubject builds an unsigned JWT carrying one claim. The plugin reads the
// subject without verifying — the console's facade is the verifier — so an
// unsigned token is exactly what this code path sees in a test.
func jwtWithSubject(sub string) string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"` + sub + `"}`))
	return "eyJhbGciOiJSUzI1NiJ9." + payload + ".signature"
}

func connArgsOf(t *testing.T, req *backend.QueryDataRequest) map[string]string {
	t.Helper()
	var parsed struct {
		ConnectionArgs map[string]string `json:"connectionArgs"`
	}
	require.NoError(t, json.Unmarshal(req.Queries[0].JSON, &parsed))
	return parsed.ConnectionArgs
}

func TestExchangeMode_SubjectKeysThePoolAndTheTokenRidesTheContext(t *testing.T) {
	token := jwtWithSubject("kc-sub-alice")
	h := NewHydrolix()
	req := makeQueryDataReq(t, exchange.CredentialsType,
		map[string]string{backend.OAuthIdentityTokenHeaderName: "Bearer " + token},
		`{"rawSql":"SELECT 1"}`)

	ctx, out := h.MutateQueryData(context.Background(), req)

	args := connArgsOf(t, out)
	assert.Equal(t, "kc-sub-alice", args["sub"],
		"the subject keys the connection cache")
	assert.NotContains(t, args, "oauthToken",
		"the forwarded token must not be in connectionArgs: it is the pool key, and Grafana refreshes it")
	for _, v := range args {
		assert.NotContains(t, v, token, "no connection arg may carry the forwarded token")
	}

	p, ok := exchange.PrincipalFrom(ctx)
	require.True(t, ok, "the principal must ride the query context")
	assert.Equal(t, "kc-sub-alice", p.Subject)
	assert.Equal(t, token, p.SubjectToken)
	assert.Equal(t, "localhost", p.Audience, "the host is the audience where settings name none")
}

func TestExchangeMode_AnExplicitAudienceWins(t *testing.T) {
	settings := models.PluginSettings{
		Host: "localhost", Port: 80, Protocol: "http",
		CredentialsType:  exchange.CredentialsType,
		ExchangeAudience: "cluster.example.hydrolix.net",
		DialTimeout:      "10", QueryTimeout: "20",
	}
	jsonData, err := json.Marshal(settings)
	require.NoError(t, err)
	req := &backend.QueryDataRequest{
		PluginContext: backend.PluginContext{
			DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{
				JSONData:                jsonData,
				DecryptedSecureJSONData: map[string]string{},
			},
		},
		Queries: []backend.DataQuery{{RefID: "A", JSON: []byte(`{"rawSql":"SELECT 1"}`)}},
	}
	req.SetHTTPHeader(backend.OAuthIdentityTokenHeaderName, "Bearer "+jwtWithSubject("kc-sub-bob"))

	ctx, _ := NewHydrolix().MutateQueryData(context.Background(), req)

	p, ok := exchange.PrincipalFrom(ctx)
	require.True(t, ok)
	assert.Equal(t, "cluster.example.hydrolix.net", p.Audience)
}

func TestExchangeMode_AnUnreadableTokenYieldsNoPrincipal(t *testing.T) {
	// No subject means nothing to key on and nothing to exchange. The query
	// then fails at the transport rather than reaching a cluster unidentified.
	h := NewHydrolix()
	req := makeQueryDataReq(t, exchange.CredentialsType,
		map[string]string{backend.OAuthIdentityTokenHeaderName: "Bearer not-a-jwt"},
		`{"rawSql":"SELECT 1"}`)

	ctx, out := h.MutateQueryData(context.Background(), req)

	assert.NotContains(t, connArgsOf(t, out), "sub")
	_, ok := exchange.PrincipalFrom(ctx)
	assert.False(t, ok, "an unreadable token must not produce a principal")
}

func TestExchangeMode_ForwardOAuthIsUntouched(t *testing.T) {
	// The existing mode keeps its behaviour exactly: this change adds a mode
	// rather than altering one.
	h := NewHydrolix()
	req := makeQueryDataReq(t, "forwardOAuth",
		map[string]string{backend.OAuthIdentityTokenHeaderName: "Bearer abc"},
		`{"rawSql":"SELECT 1"}`)

	ctx, out := h.MutateQueryData(context.Background(), req)

	args := connArgsOf(t, out)
	assert.Equal(t, "abc", args["oauthToken"])
	assert.NotContains(t, args, "sub")
	_, ok := exchange.PrincipalFrom(ctx)
	assert.False(t, ok)
}

func TestExchangeMode_NativeProtocolIsRefused(t *testing.T) {
	// Native binds its credential to the connection as a password, so a
	// refreshed token could never reach an open connection.
	settings := models.PluginSettings{
		Host: "localhost", Port: 9440, Protocol: "native",
		CredentialsType: exchange.CredentialsType,
		DialTimeout:     "10", QueryTimeout: "20",
	}
	jsonData, err := json.Marshal(settings)
	require.NoError(t, err)

	_, err = NewHydrolix().Connect(context.Background(), backend.DataSourceInstanceSettings{
		JSONData:                jsonData,
		DecryptedSecureJSONData: map[string]string{},
	}, nil)

	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "http protocol")
}

func TestExchangeMode_AnUnconfiguredGrafanaSaysSoRatherThanFailingAuth(t *testing.T) {
	settings := models.PluginSettings{
		Host: "localhost", Port: 80, Protocol: "http",
		CredentialsType: exchange.CredentialsType,
		DialTimeout:     "10", QueryTimeout: "20",
	}
	jsonData, err := json.Marshal(settings)
	require.NoError(t, err)

	// NewHydrolix reads the environment, which carries no exchange config here.
	h := NewHydrolix()
	require.Nil(t, h.exchangeSource, "the fixture assumes an unconfigured process")

	_, err = h.Connect(context.Background(), backend.DataSourceInstanceSettings{
		JSONData:                jsonData,
		DecryptedSecureJSONData: map[string]string{},
	}, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not configured",
		"an operator must read what is missing, not an auth failure")
}

func TestExchangeMode_ConnectDoesNotPingTheCluster(t *testing.T) {
	// A connection whose credential belongs to the signed-in user cannot be
	// verified at Connect time: there is no user in hand there. The first live
	// run of this change failed exactly here — every query reported "failed to
	// query server hello" because Connect pinged before anyone had signed in to
	// be. Both forwarding modes must skip it.
	assert.True(t, forwardsUserIdentity("forwardOAuth"))
	assert.True(t, forwardsUserIdentity(exchange.CredentialsType))
	assert.False(t, forwardsUserIdentity("serviceAccount"))
	assert.False(t, forwardsUserIdentity("userAccount"))
	assert.False(t, forwardsUserIdentity(""))
}

func TestExchangeMode_ConnectSucceedsWithoutReachingACluster(t *testing.T) {
	// Connect must return a usable *sql.DB without contacting anything: the host
	// below does not exist, and sql.OpenDB does not dial.
	settings := models.PluginSettings{
		Host: "nowhere.invalid", Port: 443, Protocol: "http", Secure: true,
		CredentialsType: exchange.CredentialsType,
		DialTimeout:     "10", QueryTimeout: "20",
	}
	jsonData, err := json.Marshal(settings)
	require.NoError(t, err)

	h := NewHydrolix()
	// Give the process an exchange configuration, so the mode is available.
	h.exchangeSource = exchange.NewSource(nil, nil, nil)
	h.exchangePrincipals = exchange.NewPrincipals(0, nil)

	db, err := h.Connect(context.Background(), backend.DataSourceInstanceSettings{
		JSONData:                jsonData,
		DecryptedSecureJSONData: map[string]string{},
	}, []byte(`{"sub":"kc-sub-alice"}`))
	require.NoError(t, err, "Connect must not verify a per-user connection")
	require.NotNil(t, db)
	_ = db.Close()
}

func healthOf(t *testing.T, settings models.PluginSettings, configure func(*Hydrolix)) *backend.CheckHealthResult {
	t.Helper()
	jsonData, err := json.Marshal(settings)
	require.NoError(t, err)
	instance := backend.DataSourceInstanceSettings{
		UID: "test-uid", JSONData: jsonData, DecryptedSecureJSONData: map[string]string{},
	}
	h := NewHydrolix()
	if configure != nil {
		configure(h)
	}
	ds := NewHdxSqlDatasource(h, instance)
	res, err := ds.CheckHealth(context.Background(), &backend.CheckHealthRequest{})
	require.NoError(t, err)
	return res
}

func exchangingSettings() models.PluginSettings {
	return models.PluginSettings{
		Host: "cluster.example.hydrolix.net", Port: 443, Protocol: "http", Secure: true,
		CredentialsType: exchange.CredentialsType, DialTimeout: "10", QueryTimeout: "20",
	}
}

func TestHealth_SaysWhatIsMissingWhenTheExchangeIsNotConfigured(t *testing.T) {
	res := healthOf(t, exchangingSettings(), nil)
	assert.Equal(t, backend.HealthStatusError, res.Status)
	assert.Contains(t, res.Message, "not configured")
}

func TestHealth_SaysWhichClusterHasNoCredential(t *testing.T) {
	// Credentials are per cluster: a Grafana serving several can be configured
	// for one and not another, and the message has to name which.
	res := healthOf(t, exchangingSettings(), func(h *Hydrolix) {
		cfg := exchange.Config{
			URL:         "https://console/api/v1/auth/token-exchange",
			Credentials: map[string]exchange.Credential{"other.cluster": {ClientID: "a", ClientSecret: "b"}},
		}
		h.exchangeConfig = cfg
		h.exchangeSource = exchange.NewSource(exchange.NewHTTPExchanger(cfg), nil, nil)
		h.exchangePrincipals = exchange.NewPrincipals(0, nil)
	})
	assert.Equal(t, backend.HealthStatusError, res.Status)
	assert.Contains(t, res.Message, "cluster.example.hydrolix.net")
}

func TestHealth_IsHonestThatItCannotTestAnyonesAccess(t *testing.T) {
	// A green tick must not read as "your access works": nothing about anyone's
	// access was tested, because a connection test carries no user.
	res := healthOf(t, exchangingSettings(), func(h *Hydrolix) {
		cfg := exchange.Config{
			URL: "https://console/api/v1/auth/token-exchange",
			Credentials: map[string]exchange.Credential{
				"cluster.example.hydrolix.net": {ClientID: "grafana-cluster", ClientSecret: "s"},
			},
		}
		h.exchangeConfig = cfg
		h.exchangeSource = exchange.NewSource(exchange.NewHTTPExchanger(cfg), nil, nil)
		h.exchangePrincipals = exchange.NewPrincipals(0, nil)
	})
	assert.Equal(t, backend.HealthStatusOk, res.Status)
	assert.Contains(t, res.Message, "cluster.example.hydrolix.net")
	assert.Contains(t, strings.ToLower(res.Message), "no signed-in user")
}

func TestHealth_PlainForwardingNoLongerReportsFalseFailure(t *testing.T) {
	// Upstream runs this on the bootstrap connection, which has no user, so a
	// working forwardOAuth datasource reported degraded health.
	s := exchangingSettings()
	s.CredentialsType = "forwardOAuth"
	res := healthOf(t, s, nil)
	assert.Equal(t, backend.HealthStatusOk, res.Status)
	assert.Contains(t, res.Message, "signed-in user")
}
