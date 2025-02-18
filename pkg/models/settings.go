package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/proxy"
)

var (
	ErrorMessageInvalidJSON     = errors.New("invalid settings json")
	ErrorMessageInvalidHost     = errors.New("Server address is missing")
	ErrorMessageInvalidPort     = errors.New("invalid database port")
	ErrorMessageInvalidUserName = errors.New("username is required")
	ErrorMessageInvalidPassword = errors.New("password is required")
	ErrorMessageInvalidProtocol = errors.New("protocol is should be either native or http")
)

type PluginSettings struct {
	Host             string         `json:"host"`
	UserName         string         `json:"username"`
	Port             uint16         `json:"port"`
	DefaultDatabase  string         `json:"defaultDatabase"`
	Protocol         string         `json:"protocol"`
	Password         string         `json:"-"`
	DialTimeout      string         `json:"dialTimeout,omitempty"`
	QueryTimeout     string         `json:"queryTimeout,omitempty"`
	ProxyOptions     *proxy.Options `json:"-"`
	SecureConnection bool           `json:"secureConnection"`
	SkipTlsVerify    bool           `json:"skipTlsVerify"`
}

func (settings *PluginSettings) isValid() (err error) {
	if settings.Host == "" {
		return backend.DownstreamError(ErrorMessageInvalidHost)
	}
	if settings.Port == 0 {
		return backend.DownstreamError(ErrorMessageInvalidPort)
	}
	return nil
}

func LoadPluginSettings(ctx context.Context, source backend.DataSourceInstanceSettings) (*PluginSettings, error) {
	settings := &PluginSettings{}

	var jsonData map[string]interface{}
	if err := json.Unmarshal(source.JSONData, &jsonData); err != nil {
		return settings, fmt.Errorf("%s: %w", err.Error(), ErrorMessageInvalidJSON)
	}

	if jsonData["host"] != nil {
		settings.Host = jsonData["host"].(string)
	}

	if jsonData["port"] != nil {
		settings.Port = uint16(jsonData["port"].(float64))
	}

	if jsonData["protocol"] != nil {
		settings.Protocol = jsonData["protocol"].(string)
	}

	if jsonData["username"] != nil {
		settings.UserName = jsonData["username"].(string)
	}

	if jsonData["defaultDatabase"] != nil {
		settings.DefaultDatabase = jsonData["defaultDatabase"].(string)
	}

	if jsonData["dialTimeout"] == nil {
		settings.DialTimeout = "30s"
	} else {
		if dt, ok := jsonData["dialTimeout"].(string); ok {
			if strings.TrimSpace(dt) == "" {
				dt = "30s"
			}

			if _, err := time.ParseDuration(dt); err == nil {
				// duration is valid
				settings.DialTimeout = dt
			} else if _, err := time.ParseDuration(dt + "s"); err == nil {
				// default plain number to seconds
				settings.DialTimeout = dt + "s"
			} else {
				return settings, fmt.Errorf("%s: %w", err.Error(), ErrorMessageInvalidJSON)
			}
		} else if val, ok := jsonData["dialTimeout"].(float64); ok {
			settings.DialTimeout = fmt.Sprintf("%ds", int64(val))
		}
	}

	if jsonData["queryTimeout"] == nil {
		settings.QueryTimeout = "60s"
	} else {
		if qt, ok := jsonData["queryTimeout"].(string); ok {
			if strings.TrimSpace(qt) == "" {
				qt = "60s"
			}

			if _, err := time.ParseDuration(qt); err == nil {
				// duration is valid
				settings.QueryTimeout = qt
			} else if _, err := time.ParseDuration(qt + "s"); err == nil {
				// default plain number to seconds
				settings.QueryTimeout = qt + "s"
			} else {
				return settings, fmt.Errorf("%s: %w", err.Error(), ErrorMessageInvalidJSON)
			}
		} else if val, ok := jsonData["queryTimeout"].(float64); ok {
			settings.QueryTimeout = fmt.Sprintf("%ds", int64(val))
		}
	}

	if jsonData["secureConnection"] != nil {
		settings.SecureConnection = jsonData["secureConnection"].(bool)
	}

	if jsonData["skipTlsVerify"] != nil {
		settings.SkipTlsVerify = jsonData["skipTlsVerify"].(bool)
	}
	

	if password, ok := source.DecryptedSecureJSONData["password"]; ok {
		settings.Password = password
	}

	proxyOpts, err := source.ProxyOptionsFromContext(ctx)

	if err == nil && proxyOpts != nil {
		// the sdk expects the timeout to not be a string
		timeout, err := strconv.ParseFloat(settings.DialTimeout, 64)
		if err == nil {
			proxyOpts.Timeouts.Timeout = time.Duration(timeout) * time.Second
		}

		settings.ProxyOptions = proxyOpts
	}

	return settings, settings.isValid()

}
