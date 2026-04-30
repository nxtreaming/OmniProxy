package config

import "strings"

const (
	RuntimeProfileProduction = "production"
	RuntimeProfileDev        = "dev"
)

type runtimeProfileConfig struct {
	name               string
	dataDirEnvVar      string
	defaultDataDirName string
	configDirName      string
	bootstrapFileName  string
	defaultProxyPort   int
	defaultControlPort int
}

var activeProfile = runtimeProfileConfig{
	name:               RuntimeProfileProduction,
	dataDirEnvVar:      "OMNIPROXY_DATA_DIR",
	defaultDataDirName: ".omniproxy",
	configDirName:      "OmniProxy",
	bootstrapFileName:  ".omniproxy-bootstrap.json",
	defaultProxyPort:   3000,
	defaultControlPort: 3890,
}

func SetRuntimeProfile(name string) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case RuntimeProfileDev:
		activeProfile = runtimeProfileConfig{
			name:               RuntimeProfileDev,
			dataDirEnvVar:      "OMNIPROXY_DEV_DATA_DIR",
			defaultDataDirName: ".omniproxy-dev",
			configDirName:      "OmniProxyDev",
			bootstrapFileName:  ".omniproxy-dev-bootstrap.json",
			defaultProxyPort:   3001,
			defaultControlPort: 3891,
		}
	default:
		activeProfile = runtimeProfileConfig{
			name:               RuntimeProfileProduction,
			dataDirEnvVar:      "OMNIPROXY_DATA_DIR",
			defaultDataDirName: ".omniproxy",
			configDirName:      "OmniProxy",
			bootstrapFileName:  ".omniproxy-bootstrap.json",
			defaultProxyPort:   3000,
			defaultControlPort: 3890,
		}
	}
}

func RuntimeProfile() string {
	return activeProfile.name
}

func DefaultProxyPort() int {
	return activeProfile.defaultProxyPort
}

func DefaultControlPort() int {
	return activeProfile.defaultControlPort
}
