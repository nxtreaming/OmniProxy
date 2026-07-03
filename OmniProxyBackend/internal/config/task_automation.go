package config

import "strings"

func normalizeTaskAutomationLaunchMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(strings.ReplaceAll(mode, "_", "-"))) {
	case TaskAutomationLaunchModeLinuxDO, "linux-do", "linux.do", "linux", "browser":
		return TaskAutomationLaunchModeLinuxDO
	case TaskAutomationLaunchModeMedia, "video", "app", "":
		return TaskAutomationLaunchModeMedia
	default:
		return TaskAutomationLaunchModeMedia
	}
}

func normalizeTaskAutomationBrowser(browser string) string {
	switch strings.ToLower(strings.TrimSpace(strings.ReplaceAll(browser, "_", "-"))) {
	case TaskAutomationBrowserEdge, "msedge", "microsoft-edge":
		return TaskAutomationBrowserEdge
	case TaskAutomationBrowserChrome, "google-chrome":
		return TaskAutomationBrowserChrome
	case TaskAutomationBrowserFirefox, "mozilla-firefox":
		return TaskAutomationBrowserFirefox
	case TaskAutomationBrowserDefault, "":
		return TaskAutomationBrowserDefault
	default:
		return TaskAutomationBrowserDefault
	}
}

func normalizeOutboundProxyURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.Contains(value, "://") {
		return value
	}
	if isPort(value) {
		return "http://127.0.0.1:" + value
	}
	if strings.HasPrefix(value, ":") && isPort(strings.TrimPrefix(value, ":")) {
		return "http://127.0.0.1" + value
	}
	if label, port, ok := strings.Cut(value, ":"); ok && isPort(port) {
		switch strings.ToLower(strings.TrimSpace(label)) {
		case "", "mixed", "http", "https":
			return "http://127.0.0.1:" + port
		case "socks", "socks5", "socks5h":
			return "socks5://127.0.0.1:" + port
		}
	}
	return "http://" + value
}

func normalizeTaskAutomationClients(clients []string) []string {
	if len(clients) == 0 {
		return []string{}
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(clients))
	for _, item := range clients {
		value := normalizeTaskAutomationClient(item)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func normalizeTaskAutomationClient(client string) string {
	switch strings.ToLower(strings.TrimSpace(strings.ReplaceAll(client, "_", "-"))) {
	case "codex", "openai-codex":
		return "codex"
	case "claude", "claude-code", "claudecode":
		return "claude"
	case "claude-desktop", "claude-code-desktop":
		return "claude-desktop"
	case "opencode", "open-code":
		return "opencode"
	case "deepseek-tui", "deepseek":
		return "deepseek-tui"
	case "gemini", "gemini-cli":
		return "gemini"
	case "pi", "pi-coding-agent":
		return "pi"
	default:
		return ""
	}
}
