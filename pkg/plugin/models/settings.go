package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// QuerySettings validation errors. Messages start lowercase per Go
// convention (staticcheck ST1005); previous wording capitalized the
// first word.
var (
	ErrorMessageInvalidJSON         = errors.New("invalid settings json")
	ErrorMessageInvalidHost         = errors.New("server address is missing")
	ErrorMessageInvalidPort         = errors.New("server port is missing")
	ErrorMessageInvalidProtocol     = errors.New("protocol should be either native or http")
	ErrorMessageInvalidQueryTimeout = errors.New("invalid query timeout")
	ErrorMessageInvalidDialTimeout  = errors.New("invalid connect timeout")
	ErrorMessageInvalidSettingType  = errors.New("unsupported value type")
	ErrorMessageInvalidSettingValue = errors.New("invalid value")
)

// PluginSettings structure represent data source configuration options
type PluginSettings struct {
	Host            string         `json:"host"`
	UserName        string         `json:"username"`
	Port            uint16         `json:"port"`
	Protocol        string         `json:"protocol"`
	Password        string         `json:"-"`
	Token           string         `json:"-"`
	CredentialsType string         `json:"credentialsType"`
	Secure          bool           `json:"secure"`
	Path            string         `json:"path,omitempty"`
	SkipTlsVerify   bool           `json:"skipTlsVerify,omitempty"`
	DialTimeout     string         `json:"dialTimeout,omitempty"`
	QueryTimeout    string         `json:"queryTimeout,omitempty"`
	DefaultDatabase string         `json:"defaultDatabase,omitempty"`
	QuerySettings   []QuerySetting `json:"querySettings,omitempty"`
	Other           map[string]any `json:"-"`
}

type QuerySetting struct {
	Setting string `json:"setting"`
	Value   string `json:"value"`
}

// UnmarshalJSON reads a per-query entry the way NewPluginSettings reads a
// stored one, so a number or boolean written by hand into a dashboard or an
// alert rule is kept as text instead of failing the decode.
func (qs *QuerySetting) UnmarshalJSON(data []byte) error {
	var entry any
	if err := json.Unmarshal(data, &entry); err != nil {
		return err
	}
	parsed, err := parseQuerySetting("querySettings entry", entry)
	if err != nil {
		return err
	}
	*qs = parsed
	return nil
}

// IsValid validates configuration data correctness
func (settings *PluginSettings) IsValid() error {
	if settings.Host == "" {
		return backend.DownstreamError(ErrorMessageInvalidHost)
	}
	if settings.Port == 0 {
		return backend.DownstreamError(ErrorMessageInvalidPort)
	}

	if !slices.Contains([]string{"http", "native"}, settings.Protocol) {
		return backend.DownstreamError(ErrorMessageInvalidProtocol)
	}

	if _, err := strconv.Atoi(settings.DialTimeout); err != nil {
		return backend.DownstreamError(ErrorMessageInvalidDialTimeout)
	}

	if _, err := strconv.Atoi(settings.QueryTimeout); err != nil {
		return backend.DownstreamError(ErrorMessageInvalidQueryTimeout)
	}

	return nil
}

// SetDefaults applies default values to not defined options
func (settings *PluginSettings) SetDefaults() {
	if strings.TrimSpace(settings.DialTimeout) == "" {
		settings.DialTimeout = "10"
	}
	if strings.TrimSpace(settings.QueryTimeout) == "" {
		settings.QueryTimeout = "60"
	}
}

// NewPluginSettings initializes PluginSettings with data provided by Grafana
func NewPluginSettings(_ context.Context, source backend.DataSourceInstanceSettings) (settings PluginSettings, e error) {
	var jsonData map[string]interface{}
	if err := json.Unmarshal(source.JSONData, &jsonData); err != nil {
		return settings, fmt.Errorf("%s: %w", err.Error(), ErrorMessageInvalidJSON)
	}

	stringFields := []struct {
		key string
		dst *string
	}{
		{"host", &settings.Host},
		{"protocol", &settings.Protocol},
		{"credentialsType", &settings.CredentialsType},
		{"path", &settings.Path},
		{"username", &settings.UserName},
		{"defaultDatabase", &settings.DefaultDatabase},
		{"dialTimeout", &settings.DialTimeout},
		{"queryTimeout", &settings.QueryTimeout},
	}
	for _, f := range stringFields {
		if jsonData[f.key] == nil {
			continue
		}
		v, err := parseString(f.key, jsonData[f.key])
		if err != nil {
			return settings, err
		}
		*f.dst = v
	}

	if jsonData["port"] != nil {
		port, err := parseUint("port", jsonData["port"])
		if err != nil {
			return settings, err
		}
		settings.Port = port
	}

	if jsonData["secure"] != nil {
		secure, err := parseBool("secure", jsonData["secure"])
		if err != nil {
			return settings, err
		}
		settings.Secure = secure
	}

	if jsonData["skipTlsVerify"] != nil {
		skipTlsVerify, err := parseBool("skipTlsVerify", jsonData["skipTlsVerify"])
		if err != nil {
			return settings, err
		}
		settings.SkipTlsVerify = skipTlsVerify
	}

	if jsonData["querySettings"] != nil {
		settings.QuerySettings = []QuerySetting{}
		entries, ok := jsonData["querySettings"].([]any)
		if !ok {
			return settings, backend.DownstreamError(fmt.Errorf("querySettings: %w (want an array)", ErrorMessageInvalidSettingType))
		}
		for i, entry := range entries {
			qs, err := parseQuerySetting(fmt.Sprintf("querySettings[%d]", i), entry)
			if err != nil {
				return settings, err
			}
			// The frontend drops nameless entries too, so both send the same set.
			if qs.Setting != "" {
				settings.QuerySettings = append(settings.QuerySettings, qs)
			}
		}
	}

	if password, ok := source.DecryptedSecureJSONData["password"]; ok {
		settings.Password = password
	}

	if token, ok := source.DecryptedSecureJSONData["token"]; ok {
		settings.Token = token
	}

	settings.SetDefaults()

	return settings, settings.IsValid()
}

// parseString reads a string field, also accepting the numbers and booleans
// that an unquoted value in provisioning YAML, Pulumi or the API produces.
func parseString(field string, in any) (string, error) {
	switch v := in.(type) {
	case string:
		return v, nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(v), nil
	default:
		return "", backend.DownstreamError(fmt.Errorf("%s: %w (want a string, number or boolean)", field, ErrorMessageInvalidSettingType))
	}
}

// parseQuerySetting reads one querySettings entry. A missing setting name
// yields "", a missing value yields "" (what the config editor stores for an
// empty field).
func parseQuerySetting(field string, in any) (QuerySetting, error) {
	entry, ok := in.(map[string]any)
	if !ok {
		return QuerySetting{}, backend.DownstreamError(fmt.Errorf("%s: %w (want an object with setting and value)", field, ErrorMessageInvalidSettingType))
	}
	var qs QuerySetting
	if entry["setting"] != nil {
		setting, err := parseString(field+".setting", entry["setting"])
		if err != nil {
			return QuerySetting{}, err
		}
		qs.Setting = setting
	}
	if entry["value"] != nil {
		value, err := parseString(field+".value", entry["value"])
		if err != nil {
			return QuerySetting{}, err
		}
		qs.Value = value
	}
	return qs, nil
}

// parseBool reads a boolean field, also accepting "true" / "false" text and
// the numbers 1 and 0.
func parseBool(field string, in any) (bool, error) {
	var text string
	switch v := in.(type) {
	case bool:
		return v, nil
	case string:
		text = v
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		text = fmt.Sprintf("%d", v)
	case float32:
		text = strconv.FormatFloat(float64(v), 'f', -1, 32)
	case float64:
		text = strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return false, backend.DownstreamError(fmt.Errorf("%s: %w (want a boolean)", field, ErrorMessageInvalidSettingType))
	}
	b, err := strconv.ParseBool(text)
	if err != nil {
		return false, backend.DownstreamError(fmt.Errorf("%s: %w (want true or false)", field, ErrorMessageInvalidSettingValue))
	}
	return b, nil
}

// parseUint reads a port-sized field, also accepting a whole number as text.
func parseUint(field string, in any) (uint16, error) {
	var text string
	switch v := in.(type) {
	case string:
		text = v
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		text = fmt.Sprintf("%d", v)
	case float32:
		text = strconv.FormatFloat(float64(v), 'f', -1, 32)
	case float64:
		text = strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return 0, backend.DownstreamError(fmt.Errorf("%s: %w (want a number)", field, ErrorMessageInvalidSettingType))
	}
	n, err := strconv.ParseUint(text, 10, 16)
	if err != nil {
		return 0, backend.DownstreamError(fmt.Errorf("%s: %w (want a whole number from 0 to 65535)", field, ErrorMessageInvalidSettingValue))
	}
	return uint16(n), nil
}
