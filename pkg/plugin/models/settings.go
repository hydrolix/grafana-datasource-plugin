package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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
		port, err := parseUint(jsonData["port"])
		if err != nil {
			return settings, err
		}
		settings.Port = port
	}

	if jsonData["secure"] != nil {
		secure, err := parseBool(jsonData["secure"])
		if err != nil {
			return settings, err
		}
		settings.Secure = secure
	}

	if jsonData["skipTlsVerify"] != nil {
		skipTlsVerify, err := parseBool(jsonData["skipTlsVerify"])
		if err != nil {
			return settings, err
		}
		settings.SkipTlsVerify = skipTlsVerify
	}

	if jsonData["querySettings"] != nil {
		settings.QuerySettings = []QuerySetting{}
		entries, _ := jsonData["querySettings"].([]any)
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

// parseBool parses boolean value
func parseBool(in any) (bool, error) {
	switch v := in.(type) {
	case bool:
		return v, nil
	case string:
		return strconv.ParseBool(v)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return strconv.ParseBool(fmt.Sprintf("%d", v))
	case float32, float64:
		v64, _ := strconv.ParseFloat(fmt.Sprintf("%f", v), 64)
		if math.Trunc(v64) == v64 {
			return strconv.ParseBool(strconv.FormatFloat(v64, 'f', -1, 64))
		}
		return false, backend.DownstreamError(fmt.Errorf("could not parse bool value: %s", in))
	default:
		return false, backend.DownstreamError(fmt.Errorf("could not parse bool value: %s", in))
	}
}

// parseUint parses unsigned integer value
func parseUint(in any) (uint16, error) {
	switch v := in.(type) {
	case string:
		port, err := strconv.ParseUint(v, 10, 16)
		return uint16(port), err
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		v64, err := strconv.ParseUint(fmt.Sprintf("%d", v), 10, 16)
		return uint16(v64), err
	case float32, float64:
		v64, _ := strconv.ParseFloat(fmt.Sprintf("%f", v), 64)
		if math.Trunc(v64) == v64 {
			return uint16(v64), nil
		}
		return 0, backend.DownstreamError(fmt.Errorf("could not parse bool value: %s", in))
	default:
		return 0, backend.DownstreamError(fmt.Errorf("could not parse uint value: %s", in))
	}

}
