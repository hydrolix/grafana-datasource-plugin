package models

import (
	"encoding/json"
	"fmt"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

type PluginSettings struct {
	Host             string                `json:"host"`
	UserName         string                `json:"username"`
	Port             uint16                `json:"port"`
	UseDefaultPort   bool                  `json:"useDefaultPort"`
	SecureConnection bool                  `json:"secureConnection"`
	SkipTlsVerify    bool                  `json:"skipTlsVerify"`
	Protocol         string                `json:"protocol"`
	Secrets          *SecretPluginSettings `json:"-"`
}

type SecretPluginSettings struct {
	Password string `json:"password"`
}

func LoadPluginSettings(source backend.DataSourceInstanceSettings) (*PluginSettings, error) {
	settings := PluginSettings{}
	err := json.Unmarshal(source.JSONData, &settings)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal PluginSettings json: %w", err)
	}

	settings.Secrets = loadSecretPluginSettings(source.DecryptedSecureJSONData)

	return &settings, nil
}

func loadSecretPluginSettings(source map[string]string) *SecretPluginSettings {
	return &SecretPluginSettings{
		Password: source["password"],
	}
}
