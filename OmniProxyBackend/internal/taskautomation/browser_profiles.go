package taskautomation

import (
	"strings"

	"omniproxy/internal/config"
)

type BrowserProfile struct {
	Browser      string `json:"browser"`
	BrowserLabel string `json:"browserLabel"`
	Name         string `json:"name"`
	Label        string `json:"label"`
	Account      string `json:"account,omitempty"`
	UserDataDir  string `json:"userDataDir"`
	Profile      string `json:"profile"`
	Path         string `json:"path"`
	IsDefault    bool   `json:"isDefault"`
}

func normalizeBrowserProfileKey(browser string) string {
	switch strings.ToLower(strings.TrimSpace(strings.ReplaceAll(browser, "_", "-"))) {
	case config.TaskAutomationBrowserEdge, "msedge", "microsoft-edge":
		return config.TaskAutomationBrowserEdge
	case config.TaskAutomationBrowserChrome, "google-chrome":
		return config.TaskAutomationBrowserChrome
	case config.TaskAutomationBrowserFirefox, "mozilla-firefox":
		return config.TaskAutomationBrowserFirefox
	case config.TaskAutomationBrowserDefault, "":
		return config.TaskAutomationBrowserDefault
	default:
		return config.TaskAutomationBrowserDefault
	}
}
