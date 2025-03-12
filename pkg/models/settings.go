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
	Host            string         `json:"host"`
	UserName        string         `json:"username"`
	Port            uint16         `json:"port"`
	Protocol        string         `json:"protocol"`
	Password        string         `json:"-"`
	Secure          bool           `json:"secure"`
	Path            string         `json:"path,omitempty"`
	SkipTlsVerify   bool           `json:"skipTlsVerify,omitempty"`
	DialTimeout     string         `json:"dialTimeout,omitempty"`
	QueryTimeout    string         `json:"queryTimeout,omitempty"`
	DefaultDatabase string         `json:"defaultDatabase,omitempty"`
	ProxyOptions    *proxy.Options `json:"-"`
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

func LoadPluginSettings(ctx context.Context, source backend.DataSourceInstanceSettings) (settings PluginSettings, e error) {
	var jsonData map[string]interface{}
	if err := json.Unmarshal(source.JSONData, &jsonData); err != nil {
		return settings, fmt.Errorf("%s: %w", err.Error(), ErrorMessageInvalidJSON)
	}

	if jsonData["host"] != nil {
		settings.Host = jsonData["host"].(string)
	}
	if jsonData["port"] != nil {
		if portAsFloat, ok := jsonData["port"].(float64); ok {
			settings.Port = uint16(portAsFloat)
		} else if portAsString, ok := jsonData["port"].(string); ok {
			port, err := strconv.ParseFloat(portAsString, 64)
			if err != nil {
				settings.Port = uint16(port)
			}
		}
	}
	if jsonData["protocol"] != nil {
		settings.Protocol = jsonData["protocol"].(string)
	}
	if jsonData["secure"] != nil {
		if secure, ok := jsonData["secure"].(string); ok {
			settings.Secure, e = strconv.ParseBool(secure)
			if e != nil {
				return settings, backend.DownstreamError(fmt.Errorf("could not parse secure value: %w", e))
			}
		} else {
			settings.Secure = jsonData["secure"].(bool)
		}
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
		settings.SkipTlsVerify = jsonData["skipTlsVerify"].(bool)
	}

	if password, ok := source.DecryptedSecureJSONData["password"]; ok {
		settings.Password = password
	}

	if strings.TrimSpace(settings.DialTimeout) == "" {
		settings.DialTimeout = "10"
	}
	if strings.TrimSpace(settings.QueryTimeout) == "" {
		settings.QueryTimeout = "60"
	}

	proxyOpts, e := source.ProxyOptionsFromContext(ctx)
	if e == nil && proxyOpts != nil {
		// the sdk expects the timeout to not be a string
		timeout, err := strconv.ParseFloat(settings.DialTimeout, 64)
		if err == nil {
			proxyOpts.Timeouts.Timeout = time.Duration(timeout) * time.Second
		}

		settings.ProxyOptions = proxyOpts
	}

	return settings, settings.isValid()
}

func readDuration(jsonData map[string]interface{}, name string, defaultValue string) (*string, error) {
	if jsonData[name] == nil {
		return &defaultValue, nil
	}

	var value string
	if stringValue, ok := jsonData[name].(string); ok {
		if strings.TrimSpace(stringValue) == "" {
			stringValue = defaultValue
		}
		if _, err := time.ParseDuration(stringValue); err == nil {
			value = stringValue
		} else if _, err := time.ParseDuration(stringValue + "s"); err == nil {
			value = stringValue + "s"
		} else {
			return nil, fmt.Errorf("%s: %w", err.Error(), ErrorMessageInvalidJSON)
		}
	} else if floatValue, ok := jsonData[name].(float64); ok {
		value = fmt.Sprintf("%ds", int64(floatValue))
	}

	return &value, nil
}
