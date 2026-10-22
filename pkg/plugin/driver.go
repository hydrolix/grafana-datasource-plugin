package plugin

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hydrolix/plugin/pkg/identity"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/proto"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/backend/useragent"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/grafana/sqlds/v5"
	hdxbuild "github.com/hydrolix/plugin/pkg/build"
	"github.com/hydrolix/plugin/pkg/converters"
	"github.com/hydrolix/plugin/pkg/plugin/exchange"
	"github.com/hydrolix/plugin/pkg/plugin/models"
	"github.com/pkg/errors"
)

// Hydrolix defines how to connect to a Hydrolix datasource
type Hydrolix struct {
	querySettingsContextHandler func(context.Context, map[string]any) context.Context

	// exchangeSource mints cluster tokens for the exchanging credentials mode,
	// or is nil where this instance is not configured for it. One per
	// datasource instance: NewHydrolix runs in the instance factory, so each
	// datasource exchanges and caches on its own.
	exchangeSource *exchange.Source
	// exchangeConfigErr says why exchangeSource is nil, so a datasource set to
	// the mode can fail with the reason rather than with an auth error.
	exchangeConfigErr error
	// exchangePrincipals holds the last forwarded token per subject, for the
	// requests whose context does not reach the JWT hook — the driver's own
	// handshake when a connection is established.
	exchangePrincipals *exchange.Principals
}

var (
	_ sqlds.Driver            = (*Hydrolix)(nil)
	_ sqlds.QueryMutator      = (*Hydrolix)(nil)
	_ sqlds.QueryDataMutator  = (*Hydrolix)(nil)
	_ sqlds.QueryErrorMutator = (*Hydrolix)(nil)

	OrgIdHeaderKey = "X-Grafana-Org-Id"
)

// NewHydrolix creates plugin instance with default parameters
func NewHydrolix() *Hydrolix {
	h := &Hydrolix{querySettingsContextHandler: clickhouseContextHandler}
	// Read from the environment, which is how Grafana exposes its
	// `[plugin.<id>]` configuration to a plugin process. Absent configuration is
	// not an error: it only means this instance cannot serve the exchanging
	// mode, which nothing else depends on.
	if cfg, err := exchange.ConfigFromEnv(os.LookupEnv); err == nil {
		h.exchangeSource = exchange.NewSource(exchange.NewHTTPExchanger(cfg), nil, log.DefaultLogger)
		h.exchangePrincipals = exchange.NewPrincipals(0, nil)
	} else {
		h.exchangeConfigErr = err
	}
	return h
}

// forwardsUserIdentity reports whether a credentials mode carries the signed-in
// user's identity rather than a credential stored on the datasource.
//
// It exists because this is the second place that had to know, and the first was
// a literal string comparison: a mode added later matched none of them and every
// connection failed its handshake before a query could run.
func forwardsUserIdentity(credentialsType string) bool {
	return credentialsType == "forwardOAuth" || credentialsType == exchange.CredentialsType
}

// exchangeAudienceOf answers the audience a datasource exchanges for: its host,
// which is the audience on every cluster the console registers.
//
// It is never a separate setting. An audience that could differ from the host
// would let anyone who can edit the datasource point the host at a server they
// control and the audience at a real cluster, and every viewer's real-cluster
// token would then be sent to that server.
func exchangeAudienceOf(settings models.PluginSettings) string {
	return settings.Host
}

// getClientInfoProducts reads build information of grafana and plugin
func getClientInfoProducts(ctx context.Context) (products []struct{ Name, Version string }) {
	version := useragent.FromContext(ctx).GrafanaVersion()

	if version != "" {
		products = append(products, struct{ Name, Version string }{
			Name:    "grafana",
			Version: version,
		})
	}

	info := hdxbuild.BuildInfo{}.GetBuildInfo()
	products = append(products, struct{ Name, Version string }{
		Name:    info.PluginID,
		Version: info.Version,
	})

	return products
}

// Connect opens a sql.DB connection using datasource settings
func (h *Hydrolix) Connect(ctx context.Context, config backend.DataSourceInstanceSettings, args json.RawMessage) (*sql.DB, error) {
	settings, err := models.NewPluginSettings(ctx, config)
	if err != nil {
		return nil, err
	}

	dt, _ := strconv.Atoi(settings.DialTimeout)
	qt, _ := strconv.Atoi(settings.QueryTimeout)

	protocol := clickhouse.Native
	if settings.Protocol == "http" {
		protocol = clickhouse.HTTP
	}

	compression := clickhouse.CompressionLZ4
	if protocol == clickhouse.HTTP {
		compression = clickhouse.CompressionNone
	}

	var tlsConfig *tls.Config
	if settings.Secure {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: settings.SkipTlsVerify,
		}
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(dt)*time.Second)
	defer cancel()

	opts := &clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", settings.Host, settings.Port)},

		ClientInfo: clickhouse.ClientInfo{
			Products: getClientInfoProducts(ctx),
		},
		Compression: &clickhouse.Compression{
			Method: compression,
		},
		Protocol:    protocol,
		HttpUrlPath: settings.Path,
		DialTimeout: time.Duration(dt) * time.Second,
		ReadTimeout: time.Duration(qt) * time.Second,
		TLS:         tlsConfig,

		BlockBufferSize: 2,
	}

	opts.TransportFunc = transportFor(nil)

	if settings.CredentialsType == "userAccount" || settings.CredentialsType == "" {
		opts.Auth = clickhouse.Auth{
			Database: settings.DefaultDatabase,
			Username: settings.UserName,
			Password: settings.Password,
		}

		if protocol == clickhouse.HTTP {
			// basic auth
			if settings.UserName != "" && settings.Password != "" {
				opts.HttpHeaders = map[string]string{
					"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", settings.UserName, settings.Password))),
				}
			}
			// native format
			opts.Settings = map[string]any{
				"hdx_query_output_format": "Native",
				//"hdx_query_streaming_result": "true",
			}
		}
	} else {
		token := ""
		if settings.CredentialsType == "forwardOAuth" {
			// Two callers reach here:
			//   1) sqlds.NewConnector's bootstrap call with args == nil. No
			//      token yet; build a no-auth *sql.DB. sql.OpenDB does not
			//      contact the server, and the forwardOAuth branch below
			//      skips PingContext, so this entry is safe to sit idle in
			//      the cache. Per-user DBs land on first per-query call once
			//      MutateQueryData has populated connectionArgs.oauthToken.
			//   2) Per-query call with args != nil. The token must be
			//      present; missing is a real error.
			if args == nil {
				token = ""
			} else if oAuthToken, ok := getOAuthToken(args); ok {
				token = oAuthToken
			} else {
				return nil, backend.DownstreamError(fmt.Errorf("forwardOAuth: missing OAuth token in connection args"))
			}
		} else if settings.CredentialsType != exchange.CredentialsType {
			// The exchanging mode stores no credential, and must not pick one
			// up by accident: a token left over from an earlier mode would
			// become an Authorization header below, and the fork applies
			// HttpHeaders after the JWT hook, so it would win over the
			// exchanged token.
			token = settings.Token
		}

		if protocol == clickhouse.HTTP {
			opts.Auth = clickhouse.Auth{
				Database: settings.DefaultDatabase,
			}
			httpHeaders := make(map[string]string, 2)

			orgId, ok := getOrgId(args)
			if ok {
				httpHeaders[OrgIdHeaderKey] = orgId
			}
			if token != "" {
				httpHeaders[backend.OAuthIdentityTokenHeaderName] = "Bearer " + token
			}

			if len(httpHeaders) > 0 {
				opts.HttpHeaders = httpHeaders
			}
			// native format
			opts.Settings = map[string]any{
				"hdx_query_output_format": "Native",
				//"hdx_query_streaming_result": "true",
			}
		} else {
			opts.Auth = clickhouse.Auth{
				Database: settings.DefaultDatabase,
				Username: "__api_token__",
				Password: token,
			}
		}
	}

	if settings.CredentialsType == exchange.CredentialsType {
		if protocol != clickhouse.HTTP {
			// Over HTTP the fork asks the JWT hook on every request. Over
			// native it asks once, at the handshake, and only when the
			// username is its JWT marker — a handshake no Turbine has been
			// seen to accept. Refusing is better than a second, unverified
			// code path: the console's own datasources are all HTTP.
			return nil, backend.DownstreamError(fmt.Errorf(
				"%s credentials need the http protocol: the native protocol binds its credential to the connection", exchange.CredentialsType))
		}
		if tlsConfig == nil {
			// The fork attaches a JWT only on a TLS connection
			// (applyOptionsToRequest, conn_http.go): over plain http the hook
			// is never consulted and the request would go out with no
			// credential at all. A bearer in the clear is not worth supporting.
			return nil, backend.DownstreamError(fmt.Errorf(
				"%s credentials need TLS: the cluster token is a bearer and is attached only over https", exchange.CredentialsType))
		}
		if settings.SkipTlsVerify {
			// TLS that checks nobody is a bearer handed to whoever answers at
			// the host. A cluster's certificate chains to a public root, so
			// there is nothing for skipping to work around.
			return nil, backend.DownstreamError(fmt.Errorf(
				"%s credentials need a verified TLS certificate: turn off Skip TLS verify, or the cluster token goes to whoever answers at the host", exchange.CredentialsType))
		}
		if h.exchangeSource == nil {
			// Not a connect failure. sqlds connects once when the instance is
			// created, before CheckHealth can run, so failing here would turn
			// Save & test into a raw connect error and hide the message that
			// names what is missing. The instance exists; every request on it
			// is answered by the hook with the operator's problem instead.
			opts.GetJWT = h.unconfiguredJWT
		} else {
			// The credential is supplied per request through the fork's
			// GetJWT hook, from the token source, rather than frozen into
			// this connection's headers. The hook puts the header on the
			// initial request, so http.Client's own rule for Authorization
			// on redirects applies to it.
			//
			// The connection is bound to the subject that keys it, so a
			// request whose context does not reach here — the driver's
			// handshake, issued when database/sql establishes the connection
			// — is still identified.
			subject, _ := readConnArg(args, "sub")
			binding := exchange.Bind(h.exchangeSource, h.exchangePrincipals,
				exchange.Slot{Audience: exchangeAudienceOf(settings), Subject: subject})
			opts.GetJWT = binding.JWT
			// The watch only observes: a cluster's 401 drops the refused
			// token, and sqlds retries the query once (see Settings) with a
			// fresh mint.
			opts.TransportFunc = transportFor(binding.Watch)
		}
	}

	db := clickhouse.OpenDB(opts)

	// TODO: add config UI for connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(time.Duration(2) * time.Minute)
	db.SetConnMaxLifetime(time.Duration(2) * time.Minute)

	select {
	case <-ctx.Done():
		if db != nil {
			_ = db.Close()
		}
		return nil, fmt.Errorf("connect to database was cancelled: %w", ctx.Err())
	default:
		// A connection whose credential belongs to the signed-in user cannot be
		// verified here: this runs when the connection is established, with no
		// user in hand, so the handshake would have nobody to be. Both
		// forwarding modes therefore skip it, and the first real query is what
		// proves the connection.
		if !forwardsUserIdentity(settings.CredentialsType) {
			err := db.PingContext(ctx)
			if err != nil {
				var ex *clickhouse.Exception
				if errors.As(err, &ex) {
					log.DefaultLogger.Error(
						"clickhouse exception",
						"code", ex.Code,
						"message", ex.Message,
						"stack", ex.StackTrace,
					)
				}
				if db != nil {
					_ = db.Close()
				}
				return nil, err
			}
		}
	}
	log.DefaultLogger.Debug("connect datasource", "name", config.Name)
	return db, nil
}

// Converters defines list of data type converters
func (h *Hydrolix) Converters() []sqlutil.Converter {
	return converters.Converters
}

// Macros returns list of macro functions convert the macros of raw query
func (h *Hydrolix) Macros() sqlutil.Macros {
	return sqlutil.Macros{}
}

// Settings reads Json Datasource Plugin's configuration. ForwardHeaders is
// pinned false here: the OAuth-keying flow (C4) injects connectionArgs via
// Driver.MutateQueryData, and ForwardHeaders=true would otherwise pollute
// the cache key by writing the full HTTP header map into ConnectionArgs.
func (h *Hydrolix) Settings(ctx context.Context, config backend.DataSourceInstanceSettings) sqlds.DriverSettings {
	settings, err := models.NewPluginSettings(ctx, config)
	if err != nil {
		return sqlds.DriverSettings{}
	}

	timeoutSec, _ := strconv.Atoi(settings.QueryTimeout)

	ds := sqlds.DriverSettings{
		Timeout: time.Second * time.Duration(timeoutSec),
		FillMode: &data.FillMissing{
			Mode: data.FillModeNull,
		},
		ForwardHeaders: false,
	}
	if settings.CredentialsType == exchange.CredentialsType {
		// A cluster's 401 means the token in hand is no longer accepted,
		// whatever the source believes. The binding's watch has dropped it by
		// the time the error reaches sqlds, so one retry — sqlds reconnects
		// and runs the query again — goes out with a fresh mint. A second 401
		// is the answer.
		//
		// A non-zero Retries also sends sqlds's own health check through its
		// reconnect-and-ping path. That path never runs in this mode:
		// HdxSqlDatasource.CheckHealth answers the forwarding modes itself,
		// because the bootstrap connection carries no signed-in user.
		ds.RetryOn = []string{exchange.UnauthorizedMarker}
		ds.Retries = 1
	}
	return ds
}

// MutateQueryData merges datasource's query options with the target query's
// query options, and injects per-request connectionArgs (oauthToken, orgId)
// derived from inbound HTTP headers so sqlds keys the connection cache per
// user / per org. See openspec/changes/plugin-oauth-keyed-pooling.
func (h *Hydrolix) MutateQueryData(ctx context.Context, req *backend.QueryDataRequest) (context.Context, *backend.QueryDataRequest) {
	pluginSettings, err := models.NewPluginSettings(ctx, *req.PluginContext.DataSourceInstanceSettings)

	if err != nil {
		log.DefaultLogger.Error("failed to parse plugin settings", "err", err)
		return ctx, req
	}
	if pluginSettings.QuerySettings == nil {
		pluginSettings.QuerySettings = []models.QuerySetting{}
	}

	headers := req.GetHTTPHeaders()
	// The forwarded identity onto the context, for everything downstream that
	// sqlds's interpolator signature cannot reach. Helpers in
	// `pkg/identity/identity.go`; the consumers are `cacheScope` and
	// `executeQuery` in `pkg/plugin/metadata.go`.
	ctx = identity.WithForwardedToken(ctx, identity.TokenOf(headers))
	connArgs := map[string]string{}
	if pluginSettings.CredentialsType == "forwardOAuth" {
		if token := strings.TrimPrefix(headers.Get(backend.OAuthIdentityTokenHeaderName), "Bearer "); token != "" {
			connArgs["oauthToken"] = token
		}
	}
	if pluginSettings.CredentialsType == exchange.CredentialsType {
		// The SUBJECT keys the connection cache, not the token: Grafana
		// refreshes the token every few minutes while the person stays the
		// same, so keying on the token would rebuild the pool each time. The
		// token itself rides the context, where it is not part of any key.
		//
		// The subject is issuer-qualified (exchange.IdentityOf): `sub` is
		// unique per issuer only, and Grafana can run several OAuth providers.
		if token := strings.TrimPrefix(headers.Get(backend.OAuthIdentityTokenHeaderName), "Bearer "); token != "" {
			if subject := exchange.IdentityOf(token); subject != "" {
				connArgs["sub"] = subject
				principal := exchange.Principal{
					Slot:         exchange.Slot{Audience: exchangeAudienceOf(pluginSettings), Subject: subject},
					SubjectToken: token,
				}
				ctx = exchange.WithPrincipal(ctx, principal)
				if h.exchangePrincipals != nil {
					h.exchangePrincipals.Remember(principal)
				}
			}
		}
	}
	if org := headers.Get(OrgIdHeaderKey); org != "" {
		connArgs["orgId"] = org
	}

	for i, q := range req.Queries {
		var dataQuery struct {
			RawSql        string                `json:"rawSql"`
			QuerySettings []models.QuerySetting `json:"querySettings,omitempty"`
		}
		_ = json.Unmarshal(q.JSON, &dataQuery)
		mergedSettings := make(map[string]string)
		for _, setting := range pluginSettings.QuerySettings {
			mergedSettings[setting.Setting] = setting.Value
		}
		if dataQuery.QuerySettings != nil {

			for _, setting := range dataQuery.QuerySettings {
				mergedSettings[setting.Setting] = setting.Value
			}
		}

		mergedSettingsArray := make([]models.QuerySetting, len(mergedSettings))
		n := 0
		for k, v := range mergedSettings {
			mergedSettingsArray[n] = models.QuerySetting{
				Setting: k,
				Value:   v,
			}
			n++
		}

		// connectionArgs are the connection cache's key, and the identity a
		// connection is built for. They are always the ones built above, never
		// the query's own: sqlds reads them from the query JSON the browser
		// sent, so a client-supplied value would otherwise choose whose cached
		// connection — and whose cluster token — the query runs on. With none
		// built, the key is removed rather than left to the client.
		patches := map[string]any{"querySettings": mergedSettingsArray, "connectionArgs": nil}
		if len(connArgs) > 0 {
			patches["connectionArgs"] = connArgs
		}
		jmsg, err := jsonSet(q.JSON, patches)
		if err != nil {
			// Fail CLOSED. Leaving the query as the browser sent it would hand
			// sqlds whatever connectionArgs it carried, which is exactly the
			// choice this block exists to take away. The query keeps only the
			// patches: our key, and no SQL, so sqlds refuses it rather than
			// running it on somebody else's connection.
			log.DefaultLogger.Error("failed to serialize query JSON", "err", err, "refId", q.RefID)
			jmsg = json.RawMessage("{}")
			if fallback, err := json.Marshal(withoutNil(patches)); err == nil {
				jmsg = fallback
			}
		}
		req.Queries[i].JSON = jmsg
	}

	return ctx, req
}

// MutateQuery adds user location timezone metadata if it is available. Also, it rounds the Query Time Range to
// specified time interval.
func (h *Hydrolix) MutateQuery(ctx context.Context, req backend.DataQuery) (context.Context, backend.DataQuery) {
	var dataQuery struct {
		Meta struct {
			TimeZone string `json:"timezone"`
		} `json:"meta"`
		Format        int                   `json:"format"`
		Round         string                `json:"round"`
		QuerySettings []models.QuerySetting `json:"querySettings"`
	}

	if err := json.Unmarshal(req.JSON, &dataQuery); err != nil {
		return ctx, req
	}

	if dataQuery.Meta.TimeZone != "" {
		loc, err := time.LoadLocation(dataQuery.Meta.TimeZone)
		if err != nil || loc == nil {
			log.DefaultLogger.Warn("invalid timezone", "tz", dataQuery.Meta.TimeZone)
		} else {
			log.DefaultLogger.Debug("Update query context with location info", "location", loc.String())
			ctx = clickhouse.Context(ctx, clickhouse.WithUserLocation(loc))
		}
	}

	if dataQuery.QuerySettings != nil {
		log.DefaultLogger.Debug("Update query context with settings info", "settings", dataQuery.QuerySettings)
		customSettings := make(map[string]any, len(dataQuery.QuerySettings))
		for _, v := range dataQuery.QuerySettings {
			customSettings[v.Setting] = clickhouse.CustomSetting{Value: fmt.Sprintf("%v", v.Value)}
		}
		ctx = h.querySettingsContextHandler(ctx, customSettings)
	}

	return ctx, req
}

// getOAuthToken returns the OAuth bearer token written into connectionArgs
// by MutateQueryData. The value is the bare token (Bearer-prefix stripped at
// write time); callers that need the wire form prepend "Bearer " themselves.
func getOAuthToken(args json.RawMessage) (string, bool) {
	return readConnArg(args, "oauthToken")
}

// getOrgId returns the X-Grafana-Org-Id value written into connectionArgs by
// MutateQueryData.
func getOrgId(args json.RawMessage) (string, bool) {
	return readConnArg(args, "orgId")
}

// readConnArg decodes connectionArgs as a flat map[string]string and returns
// the requested key. Nil input, malformed JSON, missing keys, and empty
// values all yield ("", false).
func readConnArg(args json.RawMessage, key string) (string, bool) {
	if len(args) == 0 {
		return "", false
	}
	var m map[string]string
	if err := json.Unmarshal(args, &m); err != nil {
		return "", false
	}
	v, ok := m[key]
	if !ok || v == "" {
		return "", false
	}
	return v, true
}

// unconfiguredJWT answers every request on an exchanging datasource whose
// Grafana has no usable exchange configuration. It is the operator's problem,
// so it reads as a misconfiguration rather than as this person being refused.
func (h *Hydrolix) unconfiguredJWT(context.Context) (string, error) {
	why := h.exchangeConfigErr
	if why == nil {
		why = exchange.ErrNotConfigured
	}
	return "", &exchange.Error{
		Outcome: exchange.Misconfigured,
		Detail:  fmt.Sprintf("this Grafana is not configured for %s credentials: %v", exchange.CredentialsType, why),
		Cause:   why,
	}
}

// exchangeQueryError presents a token source failure to a panel: its message
// is the exchange's reason alone, and it unwraps to the full chain so nothing
// that tests the chain (sqlds's ErrorQuery, the exchange outcomes) changes.
type exchangeQueryError struct {
	reason *exchange.Error
	cause  error
}

func (e *exchangeQueryError) Error() string { return e.reason.Error() }
func (e *exchangeQueryError) Unwrap() error { return e.cause }

// jsonSet update raw message's root object by applying a value to a key property.
// A nil value removes the key.
func jsonSet(jmsg json.RawMessage, val map[string]any) (json.RawMessage, error) {
	var objmap map[string]interface{}
	err := json.Unmarshal(jmsg, &objmap)
	if err != nil {
		return nil, err
	}
	if objmap == nil {
		// A JSON `null` decodes into a nil map without an error, and the
		// first assignment below would panic rather than fail.
		return nil, errors.New("query JSON is null, not an object")
	}
	for k, v := range val {
		// Drop every key that folds to this one, not just the exact spelling.
		// sqlds decodes the query with encoding/json, which matches field
		// names case-insensitively, so a surviving `ConnectionArgs` sent by
		// the browser would be read as connectionArgs and choose the
		// connection a query runs on.
		for km := range objmap {
			if strings.EqualFold(km, k) {
				delete(objmap, km)
			}
		}
		if v == nil {
			delete(objmap, k)
			continue
		}
		objmap[k] = v
	}
	return json.Marshal(objmap)
}

// withoutNil copies a patch set without the keys whose value means "remove".
// Marshalled directly, those would become JSON nulls, and sqlds reads a null
// connectionArgs as a nil map it then writes into.
func withoutNil(val map[string]any) map[string]any {
	out := make(map[string]any, len(val))
	for k, v := range val {
		if v != nil {
			out[k] = v
		}
	}
	return out
}

// clickhouseContextHandler applies query options to context
func clickhouseContextHandler(ctx context.Context, settings map[string]any) context.Context {
	return clickhouse.Context(ctx, clickhouse.WithSettings(settings))
}

// MutateResponse converts fields of type FieldTypeNullableJSON to string, except for specific visualizations - traces,
// tables, and logs.
func (h *Hydrolix) MutateResponse(_ context.Context, res data.Frames) (data.Frames, error) {
	for _, frame := range res {
		if shouldConvertFields(frame.Meta.PreferredVisualization) {
			if err := convertNullableJSONFields(frame); err != nil {
				return res, err
			}
		}
	}
	return res, nil
}

// shouldConvertFields determines whether field conversion is needed based on visualization type.
func shouldConvertFields(visType data.VisType) bool {
	return visType != data.VisTypeTrace && visType != data.VisTypeTable && visType != data.VisTypeLogs
}

// convertNullableJSONFields converts all FieldTypeNullableJSON fields in the given frame to string.
func convertNullableJSONFields(frame *data.Frame) error {
	var convertedFields []*data.Field

	for _, field := range frame.Fields {
		if field.Type() == data.FieldTypeNullableJSON {
			newField, err := convertFieldToString(field)
			if err != nil {
				return err
			}
			convertedFields = append(convertedFields, newField)
		} else {
			convertedFields = append(convertedFields, field)
		}
	}

	frame.Fields = convertedFields
	return nil
}

// convertFieldToString creates a new field where JSON values are marshaled into string representations.
func convertFieldToString(field *data.Field) (*data.Field, error) {
	values := make([]*string, field.Len())
	newField := data.NewField(field.Name, field.Labels, values)
	newField.SetConfig(field.Config)

	for i := 0; i < field.Len(); i++ {
		val, _ := field.At(i).(*json.RawMessage)
		if val == nil {
			newField.Set(i, nil)
		} else {
			bytes, err := val.MarshalJSON()
			if err != nil {
				return nil, err
			}
			sVal := string(bytes)
			newField.Set(i, &sVal)
		}
	}

	return newField, nil
}

func (h *Hydrolix) MutateQueryError(err error) backend.ErrorWithSource {
	var reason *exchange.Error
	if errors.As(err, &reason) {
		// The driver wraps the hook's answer in its own plumbing ("failed to
		// query server hello info: sendQuery: failed to get JWT: …"). The
		// panel gets the exchange's reason as the message; the chain stays
		// underneath for errors.Is, sqlds's retry test included.
		wrapped := backend.DownstreamError(&exchangeQueryError{reason: reason, cause: err})
		source := backend.ErrorSourceDownstream
		if reason.Outcome == exchange.Misconfigured {
			wrapped = backend.PluginError(&exchangeQueryError{reason: reason, cause: err})
			source = backend.ErrorSourcePlugin
		}
		return backend.NewErrorWithSource(wrapped, source)
	}

	// A redirect is refused at the transport (refuseRedirects); the fork wraps
	// that in its request plumbing, and the panel gets the reason alone.
	var redirect *RedirectError
	if errors.As(err, &redirect) {
		return backend.NewErrorWithSource(backend.DownstreamError(redirect), backend.ErrorSourceDownstream)
	}

	if uw, ok := err.(interface{ Unwrap() []error }); ok {
		for _, e := range uw.Unwrap() {
			if ex, ok := e.(*proto.Exception); ok {
				return backend.NewErrorWithSource(
					backend.DownstreamError(fmt.Errorf("Code: %d. %s: %s", ex.Code, ex.Name, ex.Message)),
					backend.ErrorSourceDownstream,
				)
			}
		}
	}

	var ex *proto.Exception
	if errors.As(err, &ex) {
		return backend.NewErrorWithSource(
			backend.DownstreamError(fmt.Errorf("Code: %d. %s: %s", ex.Code, ex.Name, ex.Message)),
			backend.ErrorSourceDownstream,
		)
	}

	return backend.NewErrorWithSource(
		backend.DownstreamError(err),
		backend.ErrorSourceDownstream,
	)
}
