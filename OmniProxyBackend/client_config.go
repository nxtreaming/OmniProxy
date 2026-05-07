package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"OmniProxyBackend/internal/logs"
)

const (
	geminiDefaultModel           = "gemini-3-pro-preview"
	opencodeOmniProviderID       = "omniproxy"
	opencodeGeminiProviderID     = "omniproxy-gemini"
	opencodeOpenRouterProviderID = "omniproxy-openrouter"
	opencodeCustomProviderID     = "omniproxy-custom"
	piOmniProviderID             = "omniproxy"
	piAnthropicProviderID        = "omniproxy-anthropic"
	piGeminiProviderID           = "omniproxy-gemini"
	piOpenRouterProviderID       = "omniproxy-openrouter"
	piCustomProviderID           = "omniproxy-custom"
	localClientAPIKey            = "omniproxy-local"
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

func (a *appServer) configureGemini() (clientConfigureResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return clientConfigureResult{}, err
	}

	a.mu.Lock()
	port := a.cfg.ProxyPort
	a.mu.Unlock()

	geminiDir := filepath.Join(home, ".gemini")
	if err := os.MkdirAll(geminiDir, 0o755); err != nil {
		return clientConfigureResult{}, err
	}

	baseURL := fmt.Sprintf("http://127.0.0.1:%d/gemini", port)
	envPath := filepath.Join(geminiDir, ".env")
	settingsPath := filepath.Join(geminiDir, "settings.json")

	if err := writeGeminiEnv(envPath, baseURL, localClientAPIKey, geminiDefaultModel); err != nil {
		return clientConfigureResult{}, err
	}
	if err := writeGeminiSettings(settingsPath, "gemini-api-key"); err != nil {
		return clientConfigureResult{}, err
	}

	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "gemini configured for omniproxy"})
	return clientConfigureResult{
		ConfigPath:   envPath,
		SettingsPath: settingsPath,
		BackupPath:   envPath + ".omniproxy.bak",
		BaseURL:      baseURL,
		Model:        geminiDefaultModel,
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
	if err := restoreBackup(envPath, envPath+".omniproxy.bak"); err != nil {
		return clientConfigureResult{}, err
	}
	if err := restoreBackup(settingsPath, settingsPath+".omniproxy.bak"); err != nil && !errors.Is(err, os.ErrNotExist) {
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

func writeGeminiEnv(path string, baseURL string, apiKey string, model string) error {
	env, err := readEnvFile(path)
	if err != nil {
		return err
	}
	if err := backupFile(path, path+".omniproxy.bak", []byte("\n")); err != nil {
		return err
	}

	env["GOOGLE_GEMINI_BASE_URL"] = baseURL
	env["GEMINI_API_KEY"] = apiKey
	env["GEMINI_MODEL"] = model
	return writeEnvFile(path, env)
}

func writeGeminiSettings(path string, selectedType string) error {
	data, err := readJSONObject(path)
	if err != nil {
		return err
	}
	if err := backupFile(path, path+".omniproxy.bak", []byte("{}\n")); err != nil {
		return err
	}

	security, _ := data["security"].(map[string]any)
	if security == nil {
		security = map[string]any{}
	}
	auth, _ := security["auth"].(map[string]any)
	if auth == nil {
		auth = map[string]any{}
	}
	auth["selectedType"] = selectedType
	security["auth"] = auth
	data["security"] = security
	return writeJSONObject(path, data)
}

func readEnvFile(path string) (map[string]string, error) {
	env := map[string]string{}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return env, nil
		}
		return nil, err
	}
	for _, line := range strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		env[key] = strings.TrimSpace(value)
	}
	return env, nil
}

func writeEnvFile(path string, env map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, key+"="+env[key])
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}

func (a *appServer) configureOpenCode() (clientConfigureResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return clientConfigureResult{}, err
	}

	a.mu.Lock()
	port := a.cfg.ProxyPort
	a.mu.Unlock()

	opencodeDir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(opencodeDir, 0o755); err != nil {
		return clientConfigureResult{}, err
	}
	configPath := filepath.Join(opencodeDir, "opencode.json")
	routerBaseURL := fmt.Sprintf("http://127.0.0.1:%d/opencode-router/v1", port)
	geminiBaseURL := fmt.Sprintf("http://127.0.0.1:%d/gemini", port)
	openRouterBaseURL := fmt.Sprintf("http://127.0.0.1:%d/openrouter/v1", port)
	customBaseURL := fmt.Sprintf("http://127.0.0.1:%d/custom/v1", port)
	openRouterModels := a.openCodeOpenRouterModels()

	if err := writeOpenCodeConfig(configPath, routerBaseURL, geminiBaseURL, openRouterBaseURL, customBaseURL, openRouterModels); err != nil {
		return clientConfigureResult{}, err
	}

	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "opencode configured for omniproxy"})
	return clientConfigureResult{
		ConfigPath: configPath,
		BackupPath: configPath + ".omniproxy.bak",
		BaseURL:    routerBaseURL,
		ProviderID: opencodeOmniProviderID,
		Message:    "OpenCode 已添加 OmniProxy、OmniProxy Gemini、OmniProxy OpenRouter 和 OmniProxy 自定义网关 provider",
	}, nil
}

func (a *appServer) restoreOpenCodeConfig() (clientConfigureResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return clientConfigureResult{}, err
	}

	configPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	if err := restoreBackup(configPath, configPath+".omniproxy.bak"); err != nil {
		return clientConfigureResult{}, err
	}

	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "opencode config restored"})
	return clientConfigureResult{
		ConfigPath: configPath,
		BackupPath: configPath + ".omniproxy.bak",
		Message:    "OpenCode 配置已恢复到一键配置前的原始配置",
	}, nil
}

func (a *appServer) configurePi() (clientConfigureResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return clientConfigureResult{}, err
	}

	a.mu.Lock()
	port := a.cfg.ProxyPort
	a.mu.Unlock()

	piDir := filepath.Join(home, ".pi", "agent")
	if err := os.MkdirAll(piDir, 0o755); err != nil {
		return clientConfigureResult{}, err
	}
	configPath := filepath.Join(piDir, "models.json")
	routerBaseURL := fmt.Sprintf("http://127.0.0.1:%d/pi-router/v1", port)
	anthropicBaseURL := fmt.Sprintf("http://127.0.0.1:%d/anthropic-router", port)
	geminiBaseURL := fmt.Sprintf("http://127.0.0.1:%d/gemini/v1beta", port)
	openRouterBaseURL := fmt.Sprintf("http://127.0.0.1:%d/openrouter/v1", port)
	customBaseURL := fmt.Sprintf("http://127.0.0.1:%d/custom/v1", port)
	openRouterModels := a.piOpenRouterModels()

	if err := writePiModelsConfig(configPath, routerBaseURL, anthropicBaseURL, geminiBaseURL, openRouterBaseURL, customBaseURL, openRouterModels); err != nil {
		return clientConfigureResult{}, err
	}

	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "pi configured for omniproxy"})
	return clientConfigureResult{
		ConfigPath: configPath,
		BackupPath: configPath + ".omniproxy.bak",
		BaseURL:    routerBaseURL,
		ProviderID: piOmniProviderID,
		Message:    "Pi Coding Agent 已添加 OmniProxy provider",
	}, nil
}

func (a *appServer) restorePiConfig() (clientConfigureResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return clientConfigureResult{}, err
	}

	configPath := filepath.Join(home, ".pi", "agent", "models.json")
	if err := restoreBackup(configPath, configPath+".omniproxy.bak"); err != nil {
		return clientConfigureResult{}, err
	}

	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "pi config restored"})
	return clientConfigureResult{
		ConfigPath: configPath,
		BackupPath: configPath + ".omniproxy.bak",
		Message:    "Pi Coding Agent 配置已恢复到一键配置前的原始配置",
	}, nil
}

func writeOpenCodeConfig(path string, routerBaseURL string, geminiBaseURL string, openRouterBaseURL string, customBaseURL string, openRouterModels map[string]any) error {
	data, err := readJSONObject(path)
	if err != nil {
		return err
	}
	if err := backupFile(path, path+".omniproxy.bak", []byte("{\n  \"$schema\": \"https://opencode.ai/config.json\"\n}\n")); err != nil {
		return err
	}
	if _, ok := data["$schema"].(string); !ok {
		data["$schema"] = "https://opencode.ai/config.json"
	}

	providers, _ := data["provider"].(map[string]any)
	if providers == nil {
		providers = map[string]any{}
	}
	providers[opencodeOmniProviderID] = openCodeRouterProvider(routerBaseURL)
	providers[opencodeGeminiProviderID] = openCodeGeminiProvider(geminiBaseURL)
	providers[opencodeOpenRouterProviderID] = openCodeOpenRouterProvider(openRouterBaseURL, openRouterModels)
	providers[opencodeCustomProviderID] = openCodeCustomProvider(customBaseURL)
	data["provider"] = providers

	return writeJSONObject(path, data)
}

func writePiModelsConfig(path string, routerBaseURL string, anthropicBaseURL string, geminiBaseURL string, openRouterBaseURL string, customBaseURL string, openRouterModels []map[string]any) error {
	data, err := readJSONObject(path)
	if err != nil {
		return err
	}
	if err := backupFile(path, path+".omniproxy.bak", []byte("{\n  \"providers\": {}\n}\n")); err != nil {
		return err
	}

	providers, _ := data["providers"].(map[string]any)
	if providers == nil {
		providers = map[string]any{}
	}
	providers[piOmniProviderID] = piOpenAIProvider("OmniProxy", routerBaseURL, []map[string]any{
		{"id": "gpt-5.4", "name": "GPT-5.4"},
		{"id": "deepseek-v4-pro", "name": "DeepSeek V4 Pro"},
		{"id": "kimi-for-coding", "name": "Kimi for Coding"},
		{"id": "glm-5.1", "name": "GLM-5.1"},
		{"id": "MiniMax-M2.7", "name": "MiniMax M2.7"},
		piReasoningModel(mimoLongContextModel, "MiMo V2.5 Pro 1M"),
		{"id": "openrouter/auto", "name": "OpenRouter Auto"},
		{"id": "custom-model", "name": "Custom Gateway Model"},
	}, true)
	providers[piAnthropicProviderID] = piAnthropicProvider(anthropicBaseURL)
	providers[piGeminiProviderID] = piGeminiProvider(geminiBaseURL)
	providers[piOpenRouterProviderID] = piOpenAIProvider("OmniProxy OpenRouter", openRouterBaseURL, openRouterModels, false)
	providers[piCustomProviderID] = piOpenAIProvider("OmniProxy Custom Gateway", customBaseURL, []map[string]any{
		{"id": "custom-model", "name": "Custom Gateway Model"},
	}, false)
	data["providers"] = providers

	return writeJSONObject(path, data)
}

func (a *appServer) openCodeOpenRouterModels() map[string]any {
	a.mu.Lock()
	cfg := a.cfg
	a.mu.Unlock()
	if cached, ok := a.cachedOpenRouterModels(cfg.OpenRouterBaseURL, "", false); ok {
		return openCodeModelsFromOpenRouter(cached.Models)
	}
	return defaultOpenCodeOpenRouterModels()
}

func (a *appServer) piOpenRouterModels() []map[string]any {
	a.mu.Lock()
	cfg := a.cfg
	a.mu.Unlock()
	if cached, ok := a.cachedOpenRouterModels(cfg.OpenRouterBaseURL, "", false); ok {
		return piModelsFromOpenRouter(cached.Models)
	}
	return defaultPiOpenRouterModels()
}

func piOpenAIProvider(name string, baseURL string, models []map[string]any, supportsReasoningEffort bool) map[string]any {
	if len(models) == 0 {
		models = defaultPiOpenRouterModels()
	}
	return map[string]any{
		"name":       name,
		"api":        "openai-completions",
		"baseUrl":    baseURL,
		"apiKey":     localClientAPIKey,
		"authHeader": true,
		"compat": map[string]any{
			"supportsDeveloperRole":   false,
			"supportsReasoningEffort": supportsReasoningEffort,
		},
		"models": models,
	}
}

func piReasoningModel(id string, name string) map[string]any {
	return map[string]any{
		"id":        id,
		"name":      name,
		"reasoning": true,
		"compat": map[string]any{
			"supportsReasoningEffort": true,
		},
	}
}

func piAnthropicProvider(baseURL string) map[string]any {
	return map[string]any{
		"name":    "OmniProxy Anthropic Router",
		"api":     "anthropic-messages",
		"baseUrl": baseURL,
		"apiKey":  localClientAPIKey,
		"models": []map[string]any{
			{"id": "claude-sonnet-4-5", "name": "Claude Sonnet 4.5"},
			{"id": "deepseek-v4-pro[1m]", "name": "DeepSeek V4 Pro"},
			{"id": "kimi-for-coding", "name": "Kimi for Coding"},
			{"id": "glm-5.1", "name": "GLM-5.1"},
			{"id": "mimo-v2.5-pro[1m]", "name": "MiMo V2.5 Pro 1M"},
		},
	}
}

func piGeminiProvider(baseURL string) map[string]any {
	return map[string]any{
		"name":    "OmniProxy Gemini",
		"api":     "google-generative-ai",
		"baseUrl": baseURL,
		"apiKey":  localClientAPIKey,
		"models": []map[string]any{
			{"id": "gemini-3-pro-preview", "name": "Gemini 3 Pro Preview"},
			{"id": "gemini-3-flash-preview", "name": "Gemini 3 Flash Preview"},
			{"id": "gemini-2.5-flash-lite", "name": "Gemini 2.5 Flash Lite"},
		},
	}
}

func piModelsFromOpenRouter(models []openRouterModelResponse) []map[string]any {
	out := make([]map[string]any, 0, len(models))
	for _, model := range models {
		id := strings.TrimSpace(model.ID)
		if id == "" {
			continue
		}
		name := strings.TrimSpace(model.Name)
		if name == "" {
			name = id
		}
		out = append(out, map[string]any{"id": id, "name": name})
	}
	if len(out) == 0 {
		return defaultPiOpenRouterModels()
	}
	return out
}

func defaultPiOpenRouterModels() []map[string]any {
	return []map[string]any{
		{"id": "openrouter/auto", "name": "OpenRouter Auto"},
	}
}

func openCodeRouterProvider(baseURL string) map[string]any {
	return map[string]any{
		"npm":  "@ai-sdk/openai-compatible",
		"name": "OmniProxy",
		"options": map[string]any{
			"baseURL":     baseURL,
			"apiKey":      localClientAPIKey,
			"setCacheKey": true,
		},
		"models": map[string]any{
			"gpt-5.4":         map[string]any{"name": "GPT-5.4"},
			"deepseek-v4-pro": map[string]any{"name": "DeepSeek V4 Pro"},
			"glm-5.1":         map[string]any{"name": "GLM-5.1"},
			"MiniMax-M2.7":    map[string]any{"name": "MiniMax M2.7"},
			"mimo-v2.5-pro":   map[string]any{"name": "MiMo V2.5 Pro"},
		},
	}
}

func openCodeGeminiProvider(baseURL string) map[string]any {
	return map[string]any{
		"npm":  "@ai-sdk/google",
		"name": "OmniProxy Gemini",
		"options": map[string]any{
			"baseURL": baseURL,
			"apiKey":  localClientAPIKey,
		},
		"models": map[string]any{
			"gemini-3-pro-preview":   map[string]any{"name": "Gemini 3 Pro Preview"},
			"gemini-3-flash-preview": map[string]any{"name": "Gemini 3 Flash Preview"},
			"gemini-2.5-flash-lite":  map[string]any{"name": "Gemini 2.5 Flash Lite"},
		},
	}
}

func openCodeOpenRouterProvider(baseURL string, models map[string]any) map[string]any {
	if len(models) == 0 {
		models = defaultOpenCodeOpenRouterModels()
	}
	return map[string]any{
		"npm":  "@ai-sdk/openai-compatible",
		"name": "OmniProxy OpenRouter",
		"options": map[string]any{
			"baseURL":     baseURL,
			"apiKey":      localClientAPIKey,
			"setCacheKey": true,
		},
		"models": models,
	}
}

func openCodeModelsFromOpenRouter(models []openRouterModelResponse) map[string]any {
	out := map[string]any{}
	for _, model := range models {
		id := strings.TrimSpace(model.ID)
		if id == "" {
			continue
		}
		name := strings.TrimSpace(model.Name)
		if name == "" {
			name = id
		}
		out[id] = map[string]any{"name": name}
	}
	if len(out) == 0 {
		return defaultOpenCodeOpenRouterModels()
	}
	return out
}

func defaultOpenCodeOpenRouterModels() map[string]any {
	return map[string]any{
		"openrouter/auto": map[string]any{"name": "OpenRouter Auto"},
	}
}

func openCodeCustomProvider(baseURL string) map[string]any {
	return map[string]any{
		"npm":  "@ai-sdk/openai-compatible",
		"name": "OmniProxy Custom Gateway",
		"options": map[string]any{
			"baseURL":     baseURL,
			"apiKey":      localClientAPIKey,
			"setCacheKey": true,
		},
		"models": map[string]any{
			"custom-model": map[string]any{"name": "Custom Gateway Model"},
		},
	}
}
