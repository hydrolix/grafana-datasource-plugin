package plugin

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/proto"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	hdxbuild "github.com/hydrolix/plugin/pkg/build"
	"github.com/hydrolix/plugin/pkg/converters"
	"github.com/hydrolix/sqlds/v5"
	"github.com/hydrolix/sqlds/v5/models"
	"github.com/pkg/errors"
)

// Hydrolix defines how to connect to a Hydrolix datasource
type Hydrolix struct {
	querySettingsContextHandler func(context.Context, map[string]any) context.Context
}

var (
	_ sqlds.Driver                   = (*Hydrolix)(nil)
	_ sqlds.QueryMutator             = (*Hydrolix)(nil)
	_ sqlds.QueryDataMutator         = (*Hydrolix)(nil)
	_ sqlds.QueryErrorMutator        = (*Hydrolix)(nil)
	_ sqlds.InterpolatedQueryMutator = (*Hydrolix)(nil)

	OrgIdHeaderKey = "X-Grafana-Org-Id"
)

// NewHydrolix creates plugin instance with default parameters
func NewHydrolix() *Hydrolix {
	return &Hydrolix{querySettingsContextHandler: clickhouseContextHandler}
}

// getClientInfoProducts reads build information of grafana and plugin
func getClientInfoProducts(ctx context.Context) (products []struct{ Name, Version string }) {
	version := backend.UserAgentFromContext(ctx).GrafanaVersion()

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

	opts.TransportFunc = func(t *http.Transport) (http.RoundTripper, error) {
		t.DisableCompression = false
		return t, nil
	}

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
			oAuthToken, ok := getOAuthToken(args)
			if ok {
				token = oAuthToken
			} else {
				return nil, fmt.Errorf("cannot get auth header")
			}
		} else {
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
		if settings.CredentialsType != "forwardOAuth" {
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

// Settings reads Json Datasource Plugin's configuration
func (h *Hydrolix) Settings(ctx context.Context, config backend.DataSourceInstanceSettings) sqlds.DriverSettings {
	settings, err := models.NewPluginSettings(ctx, config)
	if err != nil {
		return sqlds.DriverSettings{}
	}

	timeoutSec, _ := strconv.Atoi(settings.QueryTimeout)

	return sqlds.DriverSettings{
		Timeout: time.Second * time.Duration(timeoutSec),
		FillMode: &data.FillMissing{
			Mode: data.FillModeNull,
		},
		ForwardHeaders: settings.CredentialsType == "forwardOAuth",
	}
}

// adminCommentSetting is the ClickHouse setting used to carry Grafana
// attribution metadata to Hydrolix query heads.
const adminCommentSetting = "hdx_query_admin_comment"

// adminCommentManagedStart and adminCommentManagedEnd bracket the managed
// Grafana fragment inside hdx_query_admin_comment so that a subsequent
// MutateQueryData invocation on the same request can strip the prior
// fragment instead of appending a duplicate copy.
const (
	adminCommentManagedStart = "grafana_meta_start"
	adminCommentManagedEnd   = "grafana_meta_end"
)

// splitAdminCommentSegments splits an hdx_query_admin_comment value on
// unescaped ";" separators while preserving backslash escape sequences inside
// user-supplied fragments. A backslash followed by any byte is emitted
// verbatim (so `\;` stays as `\;` in the resulting segment, not as a split
// point). An unescaped ";" terminates the current segment; an optional single
// trailing space (the canonical "; " form) is consumed.
func splitAdminCommentSegments(s string) []string {
	if s == "" {
		return nil
	}
	var segments []string
	var current strings.Builder
	for i := 0; i < len(s); {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			current.WriteByte(c)
			current.WriteByte(s[i+1])
			i += 2
			continue
		}
		if c == ';' {
			segments = append(segments, current.String())
			current.Reset()
			i++
			if i < len(s) && s[i] == ' ' {
				i++
			}
			continue
		}
		current.WriteByte(c)
		i++
	}
	segments = append(segments, current.String())
	return segments
}

// stripManagedAdminCommentFragment removes any block bracketed by the managed
// markers from an existing hdx_query_admin_comment value, preserving
// user-supplied fragments. The parser is escape-aware: `\;` and `\\` inside a
// user fragment are kept as part of that fragment rather than treated as
// separators. Repeated managed blocks (e.g. from an earlier double-mutation)
// are all removed.
func stripManagedAdminCommentFragment(existing string) string {
	if !strings.Contains(existing, adminCommentManagedStart) {
		return existing
	}
	segments := splitAdminCommentSegments(existing)
	out := make([]string, 0, len(segments))
	skipping := false
	for _, p := range segments {
		switch p {
		case adminCommentManagedStart:
			skipping = true
		case adminCommentManagedEnd:
			skipping = false
		default:
			if !skipping {
				out = append(out, p)
			}
		}
	}
	return strings.Join(out, "; ")
}

// extractManagedAdminCommentFragment returns just the marker-bracketed block
// from a hdx_query_admin_comment value (without surrounding user fragments),
// or "" if no managed block is present. Used by MutateQuery to stash the
// per-query managed fragment in context for MutateInterpolatedQuery.
func extractManagedAdminCommentFragment(value string) string {
	if !strings.Contains(value, adminCommentManagedStart) {
		return ""
	}
	segments := splitAdminCommentSegments(value)
	out := make([]string, 0, len(segments))
	inside := false
	for _, p := range segments {
		switch p {
		case adminCommentManagedStart:
			inside = true
			out = append(out, p)
		case adminCommentManagedEnd:
			out = append(out, p)
			return strings.Join(out, "; ")
		default:
			if inside {
				out = append(out, p)
			}
		}
	}
	return ""
}

// managedAdminCommentCtxKey is the context key under which MutateQuery stores
// the per-query managed Grafana attribution fragment so MutateInterpolatedQuery
// can read it back after macro expansion.
type managedAdminCommentCtxKey struct{}

// grafanaQueryMeta holds attribution fields forwarded by the frontend from
// DataQueryRequest. These values are best-effort and not identity-bearing.
type grafanaQueryMeta struct {
	PanelID        json.RawMessage `json:"panelId"`
	PanelName      string          `json:"panelName"`
	PanelPluginID  string          `json:"panelPluginId"`
	DashboardUID   string          `json:"dashboardUID"`
	DashboardTitle string          `json:"dashboardTitle"`
	App            string          `json:"app"`
	RequestID      string          `json:"requestId"`
}

// normalizeAdminCommentValue sanitises a value so it can be safely placed in
// the semicolon-separated hdx_query_admin_comment fragment. Empty values
// become "unknown".
func normalizeAdminCommentValue(v string) string {
	v = strings.TrimSpace(v)
	v = strings.NewReplacer("\n", " ", "\r", " ", ";", " ").Replace(v)
	if v == "" {
		return "unknown"
	}
	return v
}

// panelIDString converts a JSON-encoded panel id (number or string) to its
// printable form, falling back to "unknown" when absent or malformed.
// Validation goes through json.Unmarshal so anything that isn't a valid JSON
// scalar (e.g. an unbalanced quote, an object, an array) yields "unknown"
// rather than leaking raw bytes into the admin comment.
func panelIDString(raw json.RawMessage) string {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return "unknown"
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return "unknown"
	}
	switch x := v.(type) {
	case nil:
		return "unknown"
	case string:
		return normalizeAdminCommentValue(x)
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		return "unknown"
	}
}

// buildGrafanaAdminComment builds the managed Grafana metadata fragment for
// the hdx_query_admin_comment setting. The output uses stable ordering.
//
// includeUserIdentity is the per-datasource PII gate. When false (default),
// user_email / user_login / user_name are always emitted as "unknown" even
// if Grafana provided real values. user_role and org_id are not gated — Role
// is a broad bucket (Admin/Editor/Viewer) and OrgID is a numeric identifier,
// neither of which directly identifies a person.
func buildGrafanaAdminComment(req *backend.QueryDataRequest, q backend.DataQuery, meta grafanaQueryMeta, includeUserIdentity bool) string {
	email := "unknown"
	login := "unknown"
	name := "unknown"
	role := "unknown"
	orgID := "unknown"
	if req != nil {
		if req.PluginContext.OrgID > 0 {
			orgID = strconv.FormatInt(req.PluginContext.OrgID, 10)
		}
		if req.PluginContext.User != nil {
			if includeUserIdentity {
				if req.PluginContext.User.Email != "" {
					email = normalizeAdminCommentValue(req.PluginContext.User.Email)
				}
				if req.PluginContext.User.Login != "" {
					login = normalizeAdminCommentValue(req.PluginContext.User.Login)
				}
				if req.PluginContext.User.Name != "" {
					name = normalizeAdminCommentValue(req.PluginContext.User.Name)
				}
			}
			if req.PluginContext.User.Role != "" {
				role = normalizeAdminCommentValue(req.PluginContext.User.Role)
			}
		}
	}

	parts := []string{
		adminCommentManagedStart,
		"user_email=" + email,
		"user_login=" + login,
		"user_name=" + name,
		"user_role=" + role,
		"org_id=" + orgID,
		"panel_id=" + panelIDString(meta.PanelID),
		"panel_name=" + normalizeAdminCommentValue(meta.PanelName),
		"panel_plugin_id=" + normalizeAdminCommentValue(meta.PanelPluginID),
		"dashboard_uid=" + normalizeAdminCommentValue(meta.DashboardUID),
		"dashboard_title=" + normalizeAdminCommentValue(meta.DashboardTitle),
		"app=" + normalizeAdminCommentValue(meta.App),
		"ref_id=" + normalizeAdminCommentValue(q.RefID),
		"request_id=" + normalizeAdminCommentValue(meta.RequestID),
		adminCommentManagedEnd,
	}
	return strings.Join(parts, "; ")
}

// attributionSettings carries plugin-specific attribution flags read directly
// from the raw DataSourceInstanceSettings.JSONData (the sqlds-provided
// PluginSettings struct doesn't model these Hydrolix-only knobs).
type attributionSettings struct {
	IncludeUserIdentity bool `json:"includeUserIdentityInAttribution"`
}

func parseAttributionSettings(jsonData json.RawMessage) attributionSettings {
	var s attributionSettings
	if len(jsonData) == 0 {
		return s
	}
	_ = json.Unmarshal(jsonData, &s)
	return s
}

// MutateQueryData merges datasource's query options with the target query's query options.
func (h *Hydrolix) MutateQueryData(ctx context.Context, req *backend.QueryDataRequest) (context.Context, *backend.QueryDataRequest) {
	pluginSettings, err := models.NewPluginSettings(ctx, *req.PluginContext.DataSourceInstanceSettings)

	if err != nil {
		log.DefaultLogger.Error("failed to parse plugin settings", "err", err)
		return ctx, req
	}
	if pluginSettings.QuerySettings == nil {
		pluginSettings.QuerySettings = []models.QuerySetting{}
	}
	attribution := parseAttributionSettings(req.PluginContext.DataSourceInstanceSettings.JSONData)

	for i, q := range req.Queries {
		var dataQuery struct {
			RawSql        string                `json:"rawSql"`
			QuerySettings []models.QuerySetting `json:"querySettings,omitempty"`
			Meta          struct {
				Grafana grafanaQueryMeta `json:"grafana"`
			} `json:"meta"`
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

		managed := buildGrafanaAdminComment(req, q, dataQuery.Meta.Grafana, attribution.IncludeUserIdentity)
		existing := stripManagedAdminCommentFragment(mergedSettings[adminCommentSetting])
		if strings.TrimSpace(existing) != "" {
			mergedSettings[adminCommentSetting] = existing + "; " + managed
		} else {
			mergedSettings[adminCommentSetting] = managed
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

		if jmsg, err := jsonSet(q.JSON, map[string]any{"querySettings": mergedSettingsArray}); err == nil {
			req.Queries[i].JSON = jmsg
		} else {
			log.DefaultLogger.Error("failed to serialize querySettings", "err", err, "refId", q.RefID)
			continue
		}
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
			if v.Setting == adminCommentSetting {
				if managed := extractManagedAdminCommentFragment(v.Value); managed != "" {
					ctx = context.WithValue(ctx, managedAdminCommentCtxKey{}, managed)
				}
			}
		}
		ctx = h.querySettingsContextHandler(ctx, customSettings)
	}

	return ctx, req
}

// MutateInterpolatedQuery runs after sqlds expands all macros. If the
// interpolated SQL ends with a SETTINGS clause, the managed Grafana
// attribution fragment (computed in MutateQueryData and stashed during
// MutateQuery) is merged into hdx_query_admin_comment so the SQL-level
// SETTINGS does not silently override the session-level value the driver
// would otherwise apply. SQL that doesn't parse, lacks a SETTINGS clause, or
// has no managed fragment in context is returned unchanged — the
// session-level injection still applies as the safety net.
func (h *Hydrolix) MutateInterpolatedQuery(ctx context.Context, sql string) (context.Context, string) {
	managed, ok := ctx.Value(managedAdminCommentCtxKey{}).(string)
	if !ok || managed == "" {
		return ctx, sql
	}
	rewritten, ok := rewriteAdminCommentInSettings(sql, managed)
	if !ok {
		return ctx, sql
	}
	return ctx, rewritten
}

func getOAuthToken(jmsg json.RawMessage) (string, bool) {
	header, ok := getHeader(backend.OAuthIdentityTokenHeaderName, jmsg)
	if ok && header != "" && strings.HasPrefix(header, "Bearer ") {
		return strings.TrimPrefix(header, "Bearer "), true
	} else {
		return "", false
	}
}

func getOrgId(jmsg json.RawMessage) (string, bool) {
	return getHeader(OrgIdHeaderKey, jmsg)
}

func getHeader(headerName string, jmsg json.RawMessage) (string, bool) {

	var m map[string]map[string][]string
	if jmsg != nil {
		err := json.Unmarshal(jmsg, &m)
		if err == nil && m != nil && m[sqlds.HeaderKey] != nil && m[sqlds.HeaderKey][headerName] != nil && len(m[sqlds.HeaderKey][headerName]) > 0 {
			header := m[sqlds.HeaderKey][headerName][0]
			return header, true
		}
	}
	return "", false
}

// jsonSet update raw message's root object by applying a value to a key property
func jsonSet(jmsg json.RawMessage, val map[string]any) (json.RawMessage, error) {
	var objmap map[string]interface{}
	err := json.Unmarshal(jmsg, &objmap)
	if err != nil {
		return nil, err
	}
	for k, v := range val {
		objmap[k] = v
	}
	return json.Marshal(objmap)
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
