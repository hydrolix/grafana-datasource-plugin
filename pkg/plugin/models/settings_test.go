package models

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/assert"
)

func TestPluginSettings(t *testing.T) {
	t.Run("parse grafana ds plugin settings", func(t *testing.T) {
		settings := PluginSettings{
			Host:            "localhost",
			Port:            80,
			Protocol:        "native",
			UserName:        "default",
			Password:        "pass",
			Secure:          true,
			Path:            "/query",
			SkipTlsVerify:   true,
			DialTimeout:     "10",
			QueryTimeout:    "20",
			DefaultDatabase: "dbdb",
			Other:           nil,
		}
		jsonData, err := json.Marshal(settings)
		if err != nil {
			t.Fatal(err)
		}

		dsSettings := backend.DataSourceInstanceSettings{
			Name:                    "test-hydrolix-http-datasource",
			JSONData:                jsonData,
			DecryptedSecureJSONData: map[string]string{"password": settings.Password},
		}
		newSettings, err := NewPluginSettings(context.Background(), dsSettings)
		assert.NoError(t, err)
		assert.Equal(t, settings, newSettings)

	})

	t.Run("parse ds plugin settings various types", func(t *testing.T) {
		settings := PluginSettings{
			Host:            "localhost",
			Port:            80,
			Protocol:        "native",
			UserName:        "default",
			Password:        "pass",
			Secure:          true,
			Path:            "/query",
			SkipTlsVerify:   true,
			DialTimeout:     "10",
			QueryTimeout:    "20",
			DefaultDatabase: "dbdb",
			Other:           nil,
		}
		originalSettings, err := json.Marshal(settings)
		if err != nil {
			t.Fatal(err)
		}

		tests := []struct {
			name string
			val  any
			res  any
		}{
			{"secure", "true", true},
			{"secure", "True", true},
			{"secure", "1", true},
			{"secure", 1, true},
			{"secure", uint16(1), true},
			{"secure", int32(1), true},
			{"secure", int64(1), true},
			{"secure", float32(1), true},
			{"secure", float64(1), true},
			{"secure", "false", false},
			{"secure", "False", false},
			{"secure", 0, false},
			{"secure", int32(0), false},
			{"secure", int64(0), false},
			{"secure", float32(0), false},
			{"secure", float64(0), false},
			{"skipTlsVerify", "true", true},
			{"skipTlsVerify", "True", true},
			{"skipTlsVerify", "1", true},
			{"skipTlsVerify", 1, true},
			{"skipTlsVerify", uint16(1), true},
			{"skipTlsVerify", int64(1), true},
			{"skipTlsVerify", float32(1), true},
			{"skipTlsVerify", float64(1), true},
			{"skipTlsVerify", "false", false},
			{"skipTlsVerify", "False", false},
			{"skipTlsVerify", 0, false},
			{"skipTlsVerify", uint16(0), false},
			{"skipTlsVerify", int64(0), false},
			{"skipTlsVerify", int16(0), false},
			{"skipTlsVerify", float64(0), false},
			{"skipTlsVerify", float32(0), false},
			{"port", uint16(80), uint16(80)},
			{"port", int32(80), uint16(80)},
			{"port", int64(80), uint16(80)},
			{"port", float64(80), uint16(80)},
			{"port", float32(80), uint16(80)},
			{"port", "80", uint16(80)},
		}
		for _, test := range tests {
			t.Run(fmt.Sprintf("%s-%v-%t", test.name, test.val, test.val), func(t *testing.T) {
				var rawJson map[string]any
				err = json.Unmarshal(originalSettings, &rawJson)
				rawJson[test.name] = test.val
				jsonSettigns, _ := json.Marshal(rawJson)
				dsSettings := backend.DataSourceInstanceSettings{JSONData: jsonSettigns}

				newSettings, err := NewPluginSettings(context.Background(), dsSettings)
				assert.NoError(t, err)

				fieldName := strings.ToUpper(test.name[:1]) + test.name[1:]
				assert.Equal(t, test.res, getField(newSettings, fieldName))

				switch test.res.(type) {
				case bool:
					v, err := parseBool(test.val)
					assert.NoError(t, err)
					assert.Equal(t, test.res, v)
				default:
					v, err := parseUint(test.val)
					assert.NoError(t, err)
					assert.Equal(t, test.res, v)
				}
			})
		}

	})
	t.Run("parse invalid grafana ds plugin settings", func(t *testing.T) {
		dsSettings := backend.DataSourceInstanceSettings{
			Name:     "test-hydrolix-http-datasource",
			JSONData: []byte("invalid"),
		}
		_, err := NewPluginSettings(context.Background(), dsSettings)
		assert.Error(t, err, "invalid json should return an error")

	})
	t.Run("validate mandatory plugin settings", func(t *testing.T) {
		settings := PluginSettings{
			Host:            "localhost",
			Port:            80,
			Protocol:        "native",
			UserName:        "default",
			Password:        "pass",
			Secure:          true,
			Path:            "/query",
			SkipTlsVerify:   true,
			DialTimeout:     "10",
			QueryTimeout:    "20",
			DefaultDatabase: "dbdb",
			Other:           nil,
		}
		assert.NoError(t, settings.IsValid(), "plugin settings should be valid")

		errSettings := settings
		errSettings.Host = ""
		assert.Error(t, errSettings.IsValid(), ErrorMessageInvalidHost)

		errSettings = settings
		errSettings.Port = 0
		assert.Error(t, errSettings.IsValid(), ErrorMessageInvalidPort)

		errSettings = settings
		errSettings.Protocol = ""
		assert.Error(t, errSettings.IsValid(), ErrorMessageInvalidProtocol)
		errSettings = settings
		errSettings.Protocol = "https"
		assert.Error(t, errSettings.IsValid(), ErrorMessageInvalidProtocol)
		errSettings = settings
		errSettings.Protocol = "native "
		assert.Error(t, errSettings.IsValid(), ErrorMessageInvalidProtocol)

		errSettings = settings
		errSettings.DialTimeout = ""
		assert.Error(t, errSettings.IsValid(), "property should be validated")
		errSettings.SetDefaults()
		assert.NoError(t, errSettings.IsValid(), "plugin settings should be valid")
		errSettings = settings
		errSettings.DialTimeout = "a"
		assert.Error(t, errSettings.IsValid(), ErrorMessageInvalidDialTimeout)

		errSettings = settings
		errSettings.QueryTimeout = ""
		assert.Error(t, errSettings.IsValid(), "property should be validated")
		errSettings.SetDefaults()
		assert.NoError(t, errSettings.IsValid(), "plugin settings should be valid")
		errSettings = settings
		errSettings.QueryTimeout = "b"
		assert.Error(t, errSettings.IsValid(), ErrorMessageInvalidQueryTimeout)

	})
}

// newSettingsFromJSON parses a minimal valid config with overrides applied,
// the way Grafana hands provisioned or API-created jsonData to the plugin.
func newSettingsFromJSON(t *testing.T, overrides map[string]any) (PluginSettings, error) {
	t.Helper()
	raw := map[string]any{"host": "localhost", "port": 80, "protocol": "native"}
	for k, v := range overrides {
		raw[k] = v
	}
	jsonData, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	return NewPluginSettings(context.Background(), backend.DataSourceInstanceSettings{JSONData: jsonData})
}

func TestPluginSettingsStringFields(t *testing.T) {
	t.Run("numbers and booleans are read as text", func(t *testing.T) {
		tests := []struct {
			key   string
			val   any
			field string
			want  string
		}{
			{"dialTimeout", 10, "DialTimeout", "10"},
			{"queryTimeout", 120, "QueryTimeout", "120"},
			{"username", 12345, "UserName", "12345"},
			{"defaultDatabase", 2024, "DefaultDatabase", "2024"},
			{"path", 1, "Path", "1"},
			{"credentialsType", true, "CredentialsType", "true"},
			{"host", 10, "Host", "10"},
			{"username", 1e21, "UserName", "1000000000000000000000"},
			{"username", 1.5, "UserName", "1.5"},
			{"username", false, "UserName", "false"},
			{"username", "default", "UserName", "default"},
		}
		for _, test := range tests {
			t.Run(fmt.Sprintf("%s-%v", test.key, test.val), func(t *testing.T) {
				settings, err := newSettingsFromJSON(t, map[string]any{test.key: test.val})
				assert.NoError(t, err)
				assert.Equal(t, test.want, getField(settings, test.field))
			})
		}
	})

	t.Run("protocol given as a number reaches protocol validation", func(t *testing.T) {
		_, err := newSettingsFromJSON(t, map[string]any{"protocol": 1})
		assert.ErrorIs(t, err, ErrorMessageInvalidProtocol)
	})

	t.Run("fractional timeouts fail the existing timeout validation", func(t *testing.T) {
		_, err := newSettingsFromJSON(t, map[string]any{"dialTimeout": 10.5})
		assert.ErrorIs(t, err, ErrorMessageInvalidDialTimeout)
		_, err = newSettingsFromJSON(t, map[string]any{"queryTimeout": 60.5})
		assert.ErrorIs(t, err, ErrorMessageInvalidQueryTimeout)
	})

	t.Run("objects and arrays return an error naming the field", func(t *testing.T) {
		for _, key := range []string{"host", "protocol", "credentialsType", "path", "username", "defaultDatabase", "dialTimeout", "queryTimeout"} {
			for _, val := range []any{map[string]any{"secret": "s3cr3t"}, []any{"s3cr3t"}} {
				t.Run(fmt.Sprintf("%s-%T", key, val), func(t *testing.T) {
					_, err := newSettingsFromJSON(t, map[string]any{key: val})
					assert.ErrorIs(t, err, ErrorMessageInvalidSettingType)
					assert.True(t, backend.IsDownstreamError(err))
					assert.Contains(t, err.Error(), key)
					assert.NotContains(t, err.Error(), "s3cr3t")
				})
			}
		}
	})
}

func TestPluginSettingsQuerySettings(t *testing.T) {
	t.Run("numeric and boolean values are read as text", func(t *testing.T) {
		settings, err := newSettingsFromJSON(t, map[string]any{"querySettings": []any{
			map[string]any{"setting": "hdx_query_max_execution_time", "value": 30},
			map[string]any{"setting": "hdx_query_streaming_result", "value": true},
			map[string]any{"setting": "x", "value": 0.5},
			map[string]any{"setting": "hdx_query_pool_name", "value": "pool"},
		}})
		assert.NoError(t, err)
		assert.Equal(t, []QuerySetting{
			{Setting: "hdx_query_max_execution_time", Value: "30"},
			{Setting: "hdx_query_streaming_result", Value: "true"},
			{Setting: "x", Value: "0.5"},
			{Setting: "hdx_query_pool_name", Value: "pool"},
		}, settings.QuerySettings)
	})

	t.Run("nameless entries are skipped and a missing value is empty", func(t *testing.T) {
		settings, err := newSettingsFromJSON(t, map[string]any{"querySettings": []any{
			map[string]any{"value": "1"},
			map[string]any{"setting": ""},
			map[string]any{"setting": "hdx_query_max_rows"},
		}})
		assert.NoError(t, err)
		assert.Equal(t, []QuerySetting{{Setting: "hdx_query_max_rows", Value: ""}}, settings.QuerySettings)
	})

	t.Run("an entry that is not an object returns an error naming its index", func(t *testing.T) {
		_, err := newSettingsFromJSON(t, map[string]any{"querySettings": []any{
			map[string]any{"setting": "hdx_query_max_rows", "value": "1"},
			"hdx_query_max_rows",
		}})
		assert.ErrorIs(t, err, ErrorMessageInvalidSettingType)
		assert.True(t, backend.IsDownstreamError(err))
		assert.Contains(t, err.Error(), "querySettings[1]")
	})

	t.Run("an object value returns an error naming the entry field", func(t *testing.T) {
		_, err := newSettingsFromJSON(t, map[string]any{"querySettings": []any{
			map[string]any{"setting": "hdx_query_max_rows", "value": map[string]any{"n": 1}},
		}})
		assert.ErrorIs(t, err, ErrorMessageInvalidSettingType)
		assert.Contains(t, err.Error(), "querySettings[0].value")
	})

	t.Run("a querySettings value that is not an array returns an error naming the field", func(t *testing.T) {
		for name, value := range map[string]any{
			"object": map[string]any{"setting": "hdx_query_max_rows", "value": "1"},
			"string": "hdx_query_max_rows=1",
			"number": 1,
			"bool":   true,
		} {
			t.Run(name, func(t *testing.T) {
				_, err := newSettingsFromJSON(t, map[string]any{"querySettings": value})
				assert.ErrorIs(t, err, ErrorMessageInvalidSettingType)
				assert.True(t, backend.IsDownstreamError(err))
				assert.Contains(t, err.Error(), "querySettings:")
				assert.NotContains(t, err.Error(), "hdx_query_max_rows")
			})
		}
	})
}

// TestNewPluginSettingsNeverPanics sets each field the backend reads to every
// JSON type; before HDX-12842 a number in a string field panicked.
func TestNewPluginSettingsNeverPanics(t *testing.T) {
	keys := []string{"host", "port", "protocol", "credentialsType", "secure", "path", "username",
		"defaultDatabase", "dialTimeout", "queryTimeout", "skipTlsVerify", "querySettings"}
	values := []any{nil, "s", 0, 1, 10.5, -1, true, false, map[string]any{"k": "v"}, []any{1, "s", nil},
		[]any{map[string]any{"setting": 1, "value": nil}}, []any{map[string]any{"setting": []any{}, "value": 1}}}
	for _, key := range keys {
		for _, val := range values {
			t.Run(fmt.Sprintf("%s-%v", key, val), func(t *testing.T) {
				assert.NotPanics(t, func() {
					_, _ = newSettingsFromJSON(t, map[string]any{key: val})
				})
			})
		}
	}
}

func getField(v any, field string) any {
	r := reflect.ValueOf(v)
	f := reflect.Indirect(r).FieldByName(field)
	return f.Interface()
}
