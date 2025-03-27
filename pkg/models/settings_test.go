package models

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/assert"
	"reflect"
	"strings"
	"testing"
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

				fieldName := strings.Title(test.name)
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

func getField(v any, field string) any {
	r := reflect.ValueOf(v)
	f := reflect.Indirect(r).FieldByName(field)
	return f.Interface()
}
