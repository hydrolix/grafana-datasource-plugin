//go:build mage
// +build mage

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"
	// mage:import
	"github.com/grafana/grafana-plugin-sdk-go/build"
)

// Default configures the default target.
var Default = build.BuildAll

// Plugin version from package.json
var pluginVersion, _ = getProperty("./package.json", "version")

// Plugin ID from plugin.json
var pluginID, _ = getProperty("./src/plugin.json", "id")

// Set before build callback in order to set up the build info
var _ = build.SetBeforeBuildCallback(func(cfg build.Config) (build.Config, error) {
	var info = fmt.Sprintf(`{"time": %d , "pluginID": "%s", "version":"%s"}`, time.Now().Unix(), pluginID, pluginVersion)
	cfg.CustomVars = map[string]string{
		"github.com/grafana/grafana-plugin-sdk-go/build.buildInfoJSON": info,
	}
	return cfg, nil
})

// getProperty retrieves root property from json file
func getProperty(file string, prop string) (string, error) {
	jsonFile, err := os.Open(file)
	if err != nil {
		fmt.Println(err)
	}
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)
	var result map[string]interface{}
	json.Unmarshal([]byte(byteValue), &result)

	return result[prop].(string), nil
}
