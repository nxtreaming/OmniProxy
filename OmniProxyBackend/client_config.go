package main

import (
	"errors"
	"fmt"
	"omniproxy/internal/clientconfig"
	"omniproxy/internal/logs"
	"os"
	"path/filepath"
	"strings"
)

const (
	geminiDefaultModel            = "gemini-3-pro-preview"
	openAICompatibleDefaultModel  = "gpt-5.6-terra"
	opencodeOmniProviderID        = "omniproxy"
	opencodeGeminiProviderID      = "omniproxy-gemini"
	opencodeOpenRouterProviderID  = "omniproxy-openrouter"
	opencodeTokenRouterProviderID = "omniproxy-tokenrouter"
	opencodeZoProviderID          = "omniproxy-zo"
	opencodeCustomProviderID      = "omniproxy-custom"
	piOmniProviderID              = "omniproxy"
	piAnthropicProviderID         = "omniproxy-anthropic"
	piGeminiProviderID            = "omniproxy-gemini"
	piOpenRouterProviderID        = "omniproxy-openrouter"
	piZoProviderID                = "omniproxy-zo"
	piCustomProviderID            = "omniproxy-custom"
	localClientAPIKey             = "omniproxy-local"
)

type clientConfigureResult struct {
	ConfigPath   string `json:"configPath,omitempty"`
	SettingsPath string `json:"settingsPath,omitempty"`
	BackupPath   string `json:"backupPath,omitempty"`
	BaseURL      string `json:"baseUrl,omitempty"`
	Model        string `json:"model,omitempty"`
	ProviderID   string `json:"providerId,omitempty"`
	Message      string `json:"message"`
}

func (a *appServer) configureDeepSeekTUI() (clientConfigureResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return clientConfigureResult{}, err
	}

	a.mu.Lock()
	cfg := a.cfg
	a.mu.Unlock()
	port := cfg.ProxyPort
	model := strings.TrimSpace(cfg.GatewayRoutes.OpenAI.Model)
	if model == "" {
		model = openAICompatibleDefaultModel
	}

	deepSeekDir := filepath.Join(home, ".deepseek")
	if err := os.MkdirAll(deepSeekDir, 0o755); err != nil {
		return clientConfigureResult{}, err
	}

	configPath := filepath.Join(deepSeekDir, "config.toml")
	baseURL := fmt.Sprintf("http://127.0.0.1:%d/opencode-router/v1", port)
	if err := clientconfig.WriteDeepSeekTUIConfig(configPath, baseURL, localClientAPIKey, model); err != nil {
		return clientConfigureResult{}, err
	}

	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "deepseek-tui configured for omniproxy"})
	return clientConfigureResult{
		ConfigPath: configPath,
		BackupPath: configPath + ".omniproxy.bak",
		BaseURL:    baseURL,
		Model:      model,
		ProviderID: "omniproxy",
		Message:    "DeepSeek-TUI 已配置为通过 OmniProxy OpenAI 兼容网关，后端平台请在网关路由中选择",
	}, nil
}

func (a *appServer) restoreDeepSeekTUIConfig() (clientConfigureResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return clientConfigureResult{}, err
	}

	configPath := filepath.Join(home, ".deepseek", "config.toml")
	if err := clientconfig.RestoreBackup(configPath, configPath+".omniproxy.bak"); err != nil {
		return clientConfigureResult{}, err
	}

	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "deepseek-tui config restored"})
	return clientConfigureResult{
		ConfigPath: configPath,
		BackupPath: configPath + ".omniproxy.bak",
		Message:    "DeepSeek-TUI 配置已恢复到一键配置前的原始配置",
	}, nil
}

func (a *appServer) configureGemini() (clientConfigureResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return clientConfigureResult{}, err
	}

	a.mu.Lock()
	cfg := a.cfg
	a.mu.Unlock()
	port := cfg.ProxyPort
	model := strings.TrimSpace(cfg.GatewayRoutes.Gemini.Model)
	if model == "" {
		model = geminiDefaultModel
	}

	geminiDir := filepath.Join(home, ".gemini")
	if err := os.MkdirAll(geminiDir, 0o755); err != nil {
		return clientConfigureResult{}, err
	}

	baseURL := fmt.Sprintf("http://127.0.0.1:%d/gemini", port)
	envPath := filepath.Join(geminiDir, ".env")
	settingsPath := filepath.Join(geminiDir, "settings.json")

	if err := clientconfig.WriteGeminiEnv(envPath, baseURL, localClientAPIKey, model); err != nil {
		return clientConfigureResult{}, err
	}
	if err := clientconfig.WriteGeminiSettings(settingsPath, "gemini-api-key"); err != nil {
		return clientConfigureResult{}, err
	}

	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "gemini configured for omniproxy"})
	return clientConfigureResult{
		ConfigPath:   envPath,
		SettingsPath: settingsPath,
		BackupPath:   envPath + ".omniproxy.bak",
		BaseURL:      baseURL,
		Model:        model,
		Message:      "Gemini CLI 已配置为通过 OmniProxy 使用 Gemini 账号池",
	}, nil
}

func (a *appServer) restoreGeminiConfig() (clientConfigureResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return clientConfigureResult{}, err
	}

	envPath := filepath.Join(home, ".gemini", ".env")
	settingsPath := filepath.Join(home, ".gemini", "settings.json")
	if err := clientconfig.RestoreBackup(envPath, envPath+".omniproxy.bak"); err != nil {
		return clientConfigureResult{}, err
	}
	if err := clientconfig.RestoreBackup(settingsPath, settingsPath+".omniproxy.bak"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return clientConfigureResult{}, err
	}

	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "gemini config restored"})
	return clientConfigureResult{
		ConfigPath:   envPath,
		SettingsPath: settingsPath,
		BackupPath:   envPath + ".omniproxy.bak",
		Message:      "Gemini CLI 配置已恢复到一键配置前的原始配置",
	}, nil
}
