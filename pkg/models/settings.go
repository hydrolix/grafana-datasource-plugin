// Package models provides Hydrolix plugin's configuration settings
package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

// QuerySettings validation errors
var (
	ErrorMessageInvalidJSON         = errors.New("invalid settings json")
	ErrorMessageInvalidHost         = errors.New("Server address is missing")
	ErrorMessageInvalidPort         = errors.New("Server port is missing")
	ErrorMessageInvalidProtocol     = errors.New("Protocol should be either native or http")
	ErrorMessageInvalidQueryTimeout = errors.New("Invalid Query Timeout")
	ErrorMessageInvalidDialTimeout  = errors.New("Invalid Connect Timeout")
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

// allowedQuerySettings lists Hydrolix allowed settings transferred as URL query parameters
var allowedQuerySettings = []string{
	"hdx_query_max_rows",
	"hdx_query_max_attempts",
	"hdx_query_max_result_bytes",
	"hdx_query_max_result_rows",
	"hdx_query_max_timerange_sec",
	"hdx_query_timerange_required",
	"hdx_query_max_partitions",
	"hdx_query_max_peers",
	"hdx_query_pool_name",
	"hdx_query_max_concurrent_partitions",
	"hdx_http_proxy_enabled",
	"hdx_http_proxy_ttl",
	"hdx_query_admin_comment",
	"hdx_query_max_execution_time",
}

// IsValid validates configuration data correctness
func (settings *PluginSettings) IsValid() error {
	if settings.Host == "" {
		return backend.DownstreamError(ErrorMessageInvalidHost)
	}
	if settings.Port == 0 {
		return backend.DownstreamError(ErrorMessageInvalidPort)
	}
	//if settings.UserName == "" {
	//	return backend.DownstreamError(ErrorMessageInvalidUserName)
	//}
	//if settings.Password == "" {
	//	return backend.DownstreamError(ErrorMessageInvalidPassword)
	//}
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

	if jsonData["host"] != nil {
		settings.Host = jsonData["host"].(string)
	}

	if jsonData["port"] != nil {
		port, err := parseUint(jsonData["port"])
		if err != nil {
			return settings, err
		}
		settings.Port = port
	}

	if jsonData["protocol"] != nil {
		settings.Protocol = jsonData["protocol"].(string)
	}

	if jsonData["credentialsType"] != nil {
		settings.CredentialsType = jsonData["credentialsType"].(string)
	}

	if jsonData["secure"] != nil {
		secure, err := parseBool(jsonData["secure"])
		if err != nil {
			return settings, err
		}
		settings.Secure = secure
	}

	if jsonData["path"] != nil {
		settings.Path = jsonData["path"].(string)
	}

	if jsonData["username"] != nil {
		settings.UserName = jsonData["username"].(string)
	}

	if jsonData["defaultDatabase"] != nil {
		settings.DefaultDatabase = jsonData["defaultDatabase"].(string)
	}

	if jsonData["dialTimeout"] != nil {
		settings.DialTimeout = jsonData["dialTimeout"].(string)
	}

	if jsonData["queryTimeout"] != nil {
		settings.QueryTimeout = jsonData["queryTimeout"].(string)
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
		rv := reflect.ValueOf(jsonData["querySettings"])
		if rv.Kind() == reflect.Slice {
			for i := 0; i < rv.Len(); i++ {
				qs := rv.Index(i).Interface().(map[string]interface{})
				settings.QuerySettings = append(settings.QuerySettings, QuerySetting{Value: qs["value"].(string), Setting: qs["setting"].(string)})
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
