package plugin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/sqlds/v5"
	"github.com/hydrolix/plugin/pkg/plugin/exchange"
	"github.com/hydrolix/plugin/pkg/plugin/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testIssuer is the issuer jwtWithSubject's tokens name.
const testIssuer = "https://kc.example/realms/console"

// jwtWithSubject builds an unsigned JWT naming a subject at testIssuer. The
// plugin reads the claims without verifying — the console's facade is the
// verifier — so an unsigned token is exactly what this code path sees in a test.
func jwtWithSubject(sub string) string {
	return jwtFrom(testIssuer, sub)
}

func jwtFrom(iss, sub string) string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"iss":"` + iss + `","sub":"` + sub + `"}`))
	return "eyJhbGciOiJSUzI1NiJ9." + payload + ".signature"
}

// identityAt is the subject the plugin keys on for a sub at testIssuer.
func identityAt(sub string) string { return testIssuer + "|" + sub }

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
	assert.Equal(t, identityAt("kc-sub-alice"), args["sub"],
		"the issuer-qualified subject keys the connection cache")
	assert.NotContains(t, args, "oauthToken",
		"the forwarded token must not be in connectionArgs: it is the pool key, and Grafana refreshes it")
	for _, v := range args {
		assert.NotContains(t, v, token, "no connection arg may carry the forwarded token")
	}

	p, ok := exchange.PrincipalFrom(ctx)
	require.True(t, ok, "the principal must ride the query context")
	assert.Equal(t, identityAt("kc-sub-alice"), p.Subject)
	assert.Equal(t, token, p.SubjectToken)
	assert.Equal(t, "localhost", p.Audience, "the host is the audience")
}

func TestExchangeMode_TheAudienceIsAlwaysTheHost(t *testing.T) {
	// An audience that could differ from the host is an exfiltration path: an
	// editor points the host at their own server and the audience at a real
	// cluster, and every viewer's real-cluster token is sent to that server. A
	// stored `exchangeAudience` from an earlier build must therefore be ignored.
	jsonData := []byte(`{"host":"attacker.example","port":443,"protocol":"http",` +
		`"credentialsType":"` + exchange.CredentialsType + `",` +
		`"exchangeAudience":"cluster.example.hydrolix.net","dialTimeout":"10","queryTimeout":"20"}`)
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
	assert.Equal(t, "attacker.example", p.Audience,
		"the token is minted for the host it will be sent to, never for another cluster")
}

func TestExchangeMode_AClientsOwnConnectionArgsNeverChooseTheConnection(t *testing.T) {
	// sqlds keys the connection on the query JSON's connectionArgs, and that JSON
	// comes from the browser. A caller with no forwarded token — basic auth,
	// a service account, alerting, a public dashboard — must not be able to name
	// someone else's subject and land on their connection and cluster token.
	h := NewHydrolix()
	victim := `{"rawSql":"SELECT 1","connectionArgs":{"sub":"` + identityAt("kc-sub-victim") + `"}}`

	t.Run("with no forwarded token", func(t *testing.T) {
		req := makeQueryDataReq(t, exchange.CredentialsType, nil, victim)
		ctx, out := h.MutateQueryData(context.Background(), req)
		assert.Nil(t, connArgsOf(t, out), "the client's connectionArgs must be removed")
		_, ok := exchange.PrincipalFrom(ctx)
		assert.False(t, ok)
	})
	t.Run("with a forwarded token of their own", func(t *testing.T) {
		req := makeQueryDataReq(t, exchange.CredentialsType,
			map[string]string{backend.OAuthIdentityTokenHeaderName: "Bearer " + jwtWithSubject("kc-sub-mallory")},
			victim)
		_, out := h.MutateQueryData(context.Background(), req)
		assert.Equal(t, identityAt("kc-sub-mallory"), connArgsOf(t, out)["sub"])
	})
	t.Run("in a mode that builds none", func(t *testing.T) {
		req := makeQueryDataReq(t, "serviceAccount", nil, victim)
		_, out := h.MutateQueryData(context.Background(), req)
		assert.Nil(t, connArgsOf(t, out))
	})
}

func TestExchangeMode_TheSameSubFromTwoIssuersIsTwoPeople(t *testing.T) {
	// `sub` is unique per issuer only, and Grafana can run several OAuth
	// providers. Keyed on `sub` alone, these two would share a connection and a
	// cached cluster token.
	h := NewHydrolix()
	subjectFor := func(iss string) string {
		req := makeQueryDataReq(t, exchange.CredentialsType,
			map[string]string{backend.OAuthIdentityTokenHeaderName: "Bearer " + jwtFrom(iss, "1234")},
			`{"rawSql":"SELECT 1"}`)
		ctx, out := h.MutateQueryData(context.Background(), req)
		p, ok := exchange.PrincipalFrom(ctx)
		require.True(t, ok)
		assert.Equal(t, p.Subject, connArgsOf(t, out)["sub"])
		return p.Subject
	}
	assert.NotEqual(t, subjectFor("https://kc.example/realms/console"), subjectFor("https://accounts.google.com"))
}

func TestExchangeMode_ATokenNamingNoIssuerYieldsNoPrincipal(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"kc-sub-alice"}`))
	req := makeQueryDataReq(t, exchange.CredentialsType,
		map[string]string{backend.OAuthIdentityTokenHeaderName: "Bearer h." + payload + ".s"},
		`{"rawSql":"SELECT 1"}`)

	ctx, out := NewHydrolix().MutateQueryData(context.Background(), req)

	assert.NotContains(t, connArgsOf(t, out), "sub")
	_, ok := exchange.PrincipalFrom(ctx)
	assert.False(t, ok, "a subject with no issuer is not an identity")
}

func TestExchangeMode_AnUnreadableTokenYieldsNoPrincipal(t *testing.T) {
	// No subject means nothing to key on and nothing to exchange. The query
	// then fails at the JWT hook rather than reaching a cluster unidentified.
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

func TestExchangeMode_PlainHTTPIsRefused(t *testing.T) {
	// The fork consults the JWT hook only on a TLS connection, so a plain-http
	// datasource in this mode would send no credential at all. Say so.
	settings := models.PluginSettings{
		Host: "localhost", Port: 8123, Protocol: "http", Secure: false,
		CredentialsType: exchange.CredentialsType,
		DialTimeout:     "10", QueryTimeout: "20",
	}
	jsonData, err := json.Marshal(settings)
	require.NoError(t, err)

	h := NewHydrolix()
	h.exchangeSource = exchange.NewSource(nil, nil, nil)
	h.exchangePrincipals = exchange.NewPrincipals(0, nil)

	_, err = h.Connect(context.Background(), backend.DataSourceInstanceSettings{
		JSONData:                jsonData,
		DecryptedSecureJSONData: map[string]string{},
	}, nil)

	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "tls")
}

func TestExchangeMode_UnverifiedTLSIsRefused(t *testing.T) {
	// TLS with verification skipped hands the bearer to whoever answers at
	// the host, which is no better than plain http for a bearer.
	settings := models.PluginSettings{
		Host: "cluster.example.hydrolix.net", Port: 443, Protocol: "http",
		Secure: true, SkipTlsVerify: true,
		CredentialsType: exchange.CredentialsType,
		DialTimeout:     "10", QueryTimeout: "20",
	}
	jsonData, err := json.Marshal(settings)
	require.NoError(t, err)

	h := NewHydrolix()
	h.exchangeSource = exchange.NewSource(nil, nil, nil)
	h.exchangePrincipals = exchange.NewPrincipals(0, nil)

	_, err = h.Connect(context.Background(), backend.DataSourceInstanceSettings{
		JSONData:                jsonData,
		DecryptedSecureJSONData: map[string]string{},
	}, nil)

	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "verified tls certificate")
	assert.True(t, backend.IsDownstreamError(err), "the remedy is the datasource's settings")
}

func TestSkipTLSVerifyStillWorksOutsideTheExchangingMode(t *testing.T) {
	// The refusal is the exchanging mode's alone: other modes carry a
	// credential the operator chose to send, and keep the switch.
	settings := models.PluginSettings{
		Host: "cluster.example.hydrolix.net", Port: 443, Protocol: "http",
		Secure: true, SkipTlsVerify: true,
		CredentialsType: "forwardOAuth",
		DialTimeout:     "10", QueryTimeout: "20",
	}
	jsonData, err := json.Marshal(settings)
	require.NoError(t, err)

	db, err := NewHydrolix().Connect(context.Background(), backend.DataSourceInstanceSettings{
		JSONData:                jsonData,
		DecryptedSecureJSONData: map[string]string{},
	}, nil)
	require.NoError(t, err)
	_ = db.Close()
}

func TestExchangeMode_AClustersUnauthorizedIsRetriedOnceAndOnlyInThisMode(t *testing.T) {
	// The retry is sqlds's: the driver names the error to retry on, the
	// binding's watch has already dropped the refused token, and the retried
	// query's JWT call is a fresh mint. The other modes keep no retry.
	settingsOf := func(t *testing.T, s models.PluginSettings) sqlds.DriverSettings {
		t.Helper()
		jsonData, err := json.Marshal(s)
		require.NoError(t, err)
		return NewHydrolix().Settings(context.Background(), backend.DataSourceInstanceSettings{
			JSONData: jsonData, DecryptedSecureJSONData: map[string]string{},
		})
	}

	ds := settingsOf(t, exchangingSettings())
	assert.Equal(t, []string{exchange.UnauthorizedMarker}, ds.RetryOn)
	assert.Equal(t, 1, ds.Retries, "one re-mint, not a loop")

	for _, mode := range []string{"forwardOAuth", "serviceAccount", "userAccount"} {
		s := exchangingSettings()
		s.CredentialsType = mode
		ds := settingsOf(t, s)
		assert.Empty(t, ds.RetryOn, mode)
		assert.Zero(t, ds.Retries, mode)
	}
}

func TestExchangeMode_AnUnconfiguredGrafanaStillGetsAnInstanceSoHealthCanSayWhatIsMissing(t *testing.T) {
	// sqlds connects once when the instance is created, before CheckHealth can
	// run. If Connect failed on a missing exchange configuration, Save & test
	// would show that raw connect error and the health message naming what is
	// missing could never be reached. So Connect succeeds, the instance
	// exists, health says what is missing, and the hook answers every request
	// with the operator's problem.
	settings := models.PluginSettings{
		Host: "cluster.example.hydrolix.net", Port: 443, Protocol: "http", Secure: true,
		CredentialsType: exchange.CredentialsType,
		DialTimeout:     "10", QueryTimeout: "20",
	}
	jsonData, err := json.Marshal(settings)
	require.NoError(t, err)
	instance := backend.DataSourceInstanceSettings{
		UID: "unconfigured", JSONData: jsonData, DecryptedSecureJSONData: map[string]string{},
	}

	// NewHydrolix reads the environment, which carries no exchange config here.
	h := NewHydrolix()
	require.Nil(t, h.exchangeSource, "the fixture assumes an unconfigured process")

	t.Run("the instance factory succeeds", func(t *testing.T) {
		ds := NewHdxSqlDatasource(h, instance)
		_, err := ds.NewDatasource(context.Background(), instance)
		require.NoError(t, err, "instance creation must not fail on the missing configuration")

		res, err := ds.CheckHealth(context.Background(), &backend.CheckHealthRequest{})
		require.NoError(t, err)
		assert.Equal(t, backend.HealthStatusError, res.Status)
		assert.Contains(t, res.Message, "not configured",
			"an operator must read what is missing, not an auth failure")
	})
	t.Run("the hook answers with the operator's problem", func(t *testing.T) {
		_, err := h.unconfiguredJWT(context.Background())
		require.Error(t, err)
		assert.ErrorIs(t, err, exchange.ErrMisconfigured)
		assert.ErrorIs(t, err, exchange.ErrNotConfigured)
		assert.Contains(t, err.Error(), "not configured")
	})
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
	}, []byte(`{"sub":"`+identityAt("kc-sub-alice")+`"}`))
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

// CFB-2612 §4.2/6.6b. The open question was whether ad-hoc filter KEY
// POPULATION carries the signed-in user, since it is reached from the editor
// rather than from a panel. It does, and this pins the mechanism that makes it
// so rather than the conclusion.
//
// Both routes end at the same place:
//
//   - the frontend's `getTagKeys` goes through `metadataProvider`'s query
//     runner, which calls `ds.query(...)` — the ordinary /api/ds/query path;
//   - the `/interpolate` RESOURCE call carries `req.Header` into the query it
//     builds, and a macro's metadata lookup re-enters the backend through
//     `MetadataProvider.executeQuery`, which copies those headers onto a
//     `QueryDataRequest` and calls `ds.QueryData`.
//
// sqlds's QueryData calls MutateQueryData (datasource.go:189), so a metadata
// lookup is mutated exactly as a panel's query is. What this test holds is
// that a request built the way `MetadataProvider.executeQuery` builds one —
// headers set with SetHTTPHeader, no connectionArgs of its own — still comes
// out carrying the subject.
func TestExchangeMode_AMetadataLookupCarriesTheSignedInUserToo(t *testing.T) {
	token := jwtWithSubject("kc-sub-carol")
	h := NewHydrolix()

	// As MetadataProvider.executeQuery builds it: a bare query plus headers.
	req := makeQueryDataReq(t, exchange.CredentialsType, nil, `{"rawSql":"DESCRIBE TABLE x","format":1}`)
	req.SetHTTPHeader(backend.OAuthIdentityTokenHeaderName, "Bearer "+token)

	ctx, out := h.MutateQueryData(context.Background(), req)

	assert.Equal(t, identityAt("kc-sub-carol"), connArgsOf(t, out)["sub"],
		"a metadata lookup keys the pool on the same subject a panel's query does")
	p, ok := exchange.PrincipalFrom(ctx)
	require.True(t, ok, "without a principal the lookup would reach the cluster as nobody")
	assert.Equal(t, token, p.SubjectToken)
}

// The counterpart: a metadata lookup arriving with no forwarded token must not
// quietly become an anonymous cluster call.
func TestExchangeMode_AMetadataLookupWithNoTokenYieldsNoPrincipal(t *testing.T) {
	req := makeQueryDataReq(t, exchange.CredentialsType, nil, `{"rawSql":"DESCRIBE TABLE x","format":1}`)

	ctx, out := NewHydrolix().MutateQueryData(context.Background(), req)

	assert.NotContains(t, connArgsOf(t, out), "sub")
	_, ok := exchange.PrincipalFrom(ctx)
	assert.False(t, ok, "no token means no principal, and the transport refuses the request")
}

func TestExchangeMode_AnExchangeFailureReachesThePanelAsItsReason(t *testing.T) {
	// The driver wraps the hook's answer in plumbing: "failed to query server
	// hello info: sendQuery: failed to get JWT: …", and sqlds adds "error
	// querying the database" on top. The panel gets the exchange's own reason;
	// the chain underneath is kept for everything that tests it.
	h := NewHydrolix()
	driverWrap := func(inner error) error {
		return fmt.Errorf("%w: %w", sqlds.ErrorQuery,
			fmt.Errorf("failed to query server hello: failed to query server hello info: sendQuery: failed to get JWT: %w", inner))
	}

	t.Run("a refusal", func(t *testing.T) {
		res := h.MutateQueryError(driverWrap(fmt.Errorf("%w: access_denied", exchange.ErrRefused)))
		assert.Equal(t, "exchange: the console refused this account access to this cluster: access_denied", res.Error())
		assert.Equal(t, backend.ErrorSourceDownstream, res.ErrorSource())
		assert.ErrorIs(t, res, exchange.ErrRefused)
		assert.ErrorIs(t, res, sqlds.ErrorQuery, "sqlds decides retries on the chain")
	})
	t.Run("an unavailable console", func(t *testing.T) {
		res := h.MutateQueryError(driverWrap(fmt.Errorf("%w: dial tcp: i/o timeout", exchange.ErrUnavailable)))
		assert.True(t, strings.HasPrefix(res.Error(), "exchange: the console could not be reached"), res.Error())
		assert.NotContains(t, res.Error(), "server hello")
	})
	t.Run("an unconfigured Grafana is the plugin's problem", func(t *testing.T) {
		unconfigured := &Hydrolix{exchangeConfigErr: exchange.ErrNotConfigured}
		_, hookErr := unconfigured.unconfiguredJWT(context.Background())
		res := h.MutateQueryError(driverWrap(hookErr))
		assert.Equal(t, backend.ErrorSourcePlugin, res.ErrorSource())
		assert.Contains(t, res.Error(), "not configured")
		assert.NotContains(t, res.Error(), "server hello")
	})
	t.Run("a cluster 401 keeps the retry marker", func(t *testing.T) {
		// Not an exchange failure: the cluster answered. sqlds matches the
		// marker in the message to retry once, so the message is untouched.
		raw := fmt.Errorf("%w: sendQuery: %s response body: \"Code: 516\"", sqlds.ErrorQuery, exchange.UnauthorizedMarker)
		res := h.MutateQueryError(raw)
		assert.Contains(t, res.Error(), exchange.UnauthorizedMarker)
	})
}

func TestExchangeMode_ACaseVariantConnectionArgsKeyNeverSurvives(t *testing.T) {
	// Go's encoding/json matches object keys case-insensitively, so sqlds would
	// read `ConnectionArgs` or `connectionargs` as the connection key just as
	// it reads `connectionArgs`. Every variant the browser sent is removed, and
	// only the one built here remains.
	h := NewHydrolix()
	victim := identityAt("kc-sub-victim")
	query := `{"rawSql":"SELECT 1",` +
		`"ConnectionArgs":{"sub":"` + victim + `"},` +
		`"connectionargs":{"sub":"` + victim + `"},` +
		`"CONNECTIONARGS":{"sub":"` + victim + `"}}`

	keysLike := func(t *testing.T, req *backend.QueryDataRequest, name string) map[string]any {
		t.Helper()
		var obj map[string]any
		require.NoError(t, json.Unmarshal(req.Queries[0].JSON, &obj))
		found := map[string]any{}
		for k, v := range obj {
			if strings.EqualFold(k, name) {
				found[k] = v
			}
		}
		return found
	}

	t.Run("with no forwarded token every variant is gone", func(t *testing.T) {
		req := makeQueryDataReq(t, exchange.CredentialsType, nil, query)
		_, out := h.MutateQueryData(context.Background(), req)
		assert.Empty(t, keysLike(t, out, "connectionArgs"))
	})
	t.Run("with a forwarded token only the built key remains", func(t *testing.T) {
		req := makeQueryDataReq(t, exchange.CredentialsType,
			map[string]string{backend.OAuthIdentityTokenHeaderName: "Bearer " + jwtWithSubject("kc-sub-mallory")},
			query)
		_, out := h.MutateQueryData(context.Background(), req)
		found := keysLike(t, out, "connectionArgs")
		require.Len(t, found, 1)
		args, ok := found["connectionArgs"].(map[string]any)
		require.True(t, ok, "the surviving key is the canonical one: %v", found)
		assert.Equal(t, identityAt("kc-sub-mallory"), args["sub"])
		assert.NotContains(t, string(out.Queries[0].JSON), victim)
	})
}
