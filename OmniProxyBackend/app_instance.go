package main

import (
	"os"
	"path/filepath"
	"strings"
)

const singleInstanceIDBase = "com.omniproxy.desktop"

func singleInstanceUniqueID() string {
	return singleInstanceIDBase + "." + appRuntimeMode()
}

func appRuntimeMode() string {
	if strings.EqualFold(strings.TrimSpace(appInstanceMode), "dev") {
		return "dev"
	}
	if isDevelopmentVersion(appVersion) {
		return "dev"
	}
	executable, err := os.Executable()
	if err == nil {
		name := strings.ToLower(filepath.Base(executable))
		if strings.Contains(name, "dev") {
			return "dev"
		}
	}
	return "production"
}

func isDevInstance() bool {
	return appRuntimeMode() == "dev"
}

func appDisplayName() string {
	if isDevInstance() {
		return "OmniProxy Dev"
	}
	return "OmniProxy"
}
