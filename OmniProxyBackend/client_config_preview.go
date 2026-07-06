package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"omniproxy/internal/claudedesktop"
)

type clientConfigPreview struct {
	Client       string   `json:"client"`
	ConfigPath   string   `json:"configPath,omitempty"`
	SettingsPath string   `json:"settingsPath,omitempty"`
	BackupPath   string   `json:"backupPath,omitempty"`
	BaseURL      string   `json:"baseUrl,omitempty"`
	Model        string   `json:"model,omitempty"`
	Models       []string `json:"models,omitempty"`
	ProviderID   string   `json:"providerId,omitempty"`
	Message      string   `json:"message"`
}

func (a *appServer) clientConfigPreviews() ([]clientConfigPreview, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	cfg := a.cfg
	a.mu.Unlock()
	port := cfg.ProxyPort
	codexDir := filepath.Join(home, ".codex")
	claudeDir := filepath.Join(home, ".claude")
	openAIModel := strings.TrimSpace(cfg.GatewayRoutes.OpenAI.Model)
	if openAIModel == "" {
		openAIModel = openAICompatibleDefaultModel
	}
	geminiModel := strings.TrimSpace(cfg.GatewayRoutes.Gemini.Model)
	if geminiModel == "" {
		geminiModel = geminiDefaultModel
	}
	claudeModel := strings.TrimSpace(cfg.GatewayRoutes.Claude.Model)
	if claudeModel == "" {
		claudeModel = claudeDefaultModel
	}

	previews := []clientConfigPreview{
		{
			Client:     "Codex",
			ConfigPath: filepath.Join(codexDir, "config.toml"),
			BackupPath: filepath.Join(codexDir, "config.toml") + ".omniproxy.bak",
			BaseURL:    fmt.Sprintf("http://127.0.0.1:%d/codex/v1", port),
			Models:     append([]string(nil), defaultCodexModels...),
			Message:    "写入 OpenAI Responses 兼容入口，并确保 auth.json 有本地占位 API Key",
		},
		{
			Client:       "Claude Code",
			SettingsPath: filepath.Join(claudeDir, "settings.json"),
			BackupPath:   filepath.Join(claudeDir, "settings.json") + ".omniproxy.bak",
			BaseURL:      fmt.Sprintf("http://127.0.0.1:%d/anthropic-router", port),
			Model:        claudeModel,
			Message:      "写入 Anthropic Messages 兼容入口",
		},
		{
			Client:     "DeepSeek-TUI",
			ConfigPath: filepath.Join(home, ".deepseek", "config.toml"),
			BackupPath: filepath.Join(home, ".deepseek", "config.toml") + ".omniproxy.bak",
			BaseURL:    fmt.Sprintf("http://127.0.0.1:%d/opencode-router/v1", port),
			Model:      openAIModel,
			ProviderID: "omniproxy",
			Message:    "写入 OpenAI 兼容 provider",
		},
		{
			Client:       "Gemini CLI",
			ConfigPath:   filepath.Join(home, ".gemini", ".env"),
			SettingsPath: filepath.Join(home, ".gemini", "settings.json"),
			BackupPath:   filepath.Join(home, ".gemini", ".env") + ".omniproxy.bak",
			BaseURL:      fmt.Sprintf("http://127.0.0.1:%d/gemini", port),
			Model:        geminiModel,
			Message:      "写入 Gemini 原生入口和本地 API Key 占位",
		},
		{
			Client:     "OpenCode",
			ConfigPath: filepath.Join(home, ".config", "opencode", "opencode.json"),
			BackupPath: filepath.Join(home, ".config", "opencode", "opencode.json") + ".omniproxy.bak",
			BaseURL:    fmt.Sprintf("http://127.0.0.1:%d/opencode-router/v1", port),
			ProviderID: opencodeOmniProviderID,
			Message:    "添加 OmniProxy provider，并补充 Gemini/OpenRouter/TokenRouter/Zo/Custom 入口",
		},
		{
			Client:     "Pi Coding Agent",
			ConfigPath: filepath.Join(home, ".pi", "agent", "models.json"),
			BackupPath: filepath.Join(home, ".pi", "agent", "models.json") + ".omniproxy.bak",
			BaseURL:    fmt.Sprintf("http://127.0.0.1:%d/pi-router/v1", port),
			ProviderID: piOmniProviderID,
			Message:    "添加 OmniProxy 模型 provider",
		},
	}
	if desktopPreview, ok := claudeDesktopPreview(port); ok {
		previews = append(previews, desktopPreview)
	}
	return previews, nil
}

func claudeDesktopPreview(port int) (clientConfigPreview, bool) {
	paths, err := claudedesktop.CurrentPaths()
	if err != nil {
		return clientConfigPreview{}, false
	}
	return clientConfigPreview{
		Client:     "Claude Code Desktop",
		ConfigPath: paths.ProfilePath,
		BackupPath: paths.MetaPath,
		BaseURL:    claudedesktop.GatewayBaseURL(port),
		Message:    "创建 OmniProxy 3P profile，并写入模型路由映射",
	}, true
}
