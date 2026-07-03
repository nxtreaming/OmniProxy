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
	geminiDefaultModel            = "gemini-3-pro-preview"
	openAICompatibleDefaultModel  = "gpt-5.4"
	deepSeekTUIClientHeader       = "DeepSeek-TUI"
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
	if err := writeDeepSeekTUIConfig(configPath, baseURL, localClientAPIKey, model); err != nil {
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
	if err := restoreBackup(configPath, configPath+".omniproxy.bak"); err != nil {
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

	if err := writeGeminiEnv(envPath, baseURL, localClientAPIKey, model); err != nil {
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

func writeDeepSeekTUIConfig(path string, baseURL string, apiKey string, model string) error {
	if err := backupFile(path, path+".omniproxy.bak", []byte("\n")); err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	lines := splitTextLines(strings.TrimPrefix(string(raw), "\ufeff"))
	lines = setTOMLStringValue(lines, "", "provider", "omniproxy")
	lines = setTOMLStringValue(lines, "", "default_text_model", model)
	lines = setTOMLStringValue(lines, "providers.omniproxy", "api_key", apiKey)
	lines = setTOMLStringValue(lines, "providers.omniproxy", "base_url", baseURL)
	lines = setTOMLStringValue(lines, "providers.omniproxy", "model", model)
	lines = setTOMLRawValueIfMissing(lines, "providers.omniproxy", "http_headers", fmt.Sprintf(`{ "X-OmniProxy-Client" = %s }`, tomlString(deepSeekTUIClientHeader)))

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}

func splitTextLines(raw string) []string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	if raw == "" {
		return nil
	}
	lines := strings.Split(raw, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func setTOMLStringValue(lines []string, section string, key string, value string) []string {
	return setTOMLRawValue(lines, section, key, tomlString(value))
}

func setTOMLRawValue(lines []string, section string, key string, rawValue string) []string {
	start, end, foundSection := tomlSectionBounds(lines, section)
	if !foundSection && section != "" {
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, "["+section+"]")
		start = len(lines)
		end = len(lines)
	}

	replacement := key + " = " + rawValue
	matched := false
	out := make([]string, 0, len(lines))
	for i, line := range lines {
		if i >= start && i < end && tomlLineKey(line) == key {
			if !matched {
				out = append(out, replacement)
				matched = true
			}
			continue
		}
		out = append(out, line)
	}
	if matched {
		return out
	}
	return insertLine(lines, end, replacement)
}

func setTOMLRawValueIfMissing(lines []string, section string, key string, rawValue string) []string {
	start, end, foundSection := tomlSectionBounds(lines, section)
	if foundSection {
		for i := start; i < end; i++ {
			if tomlLineKey(lines[i]) == key {
				return lines
			}
		}
	}
	return setTOMLRawValue(lines, section, key, rawValue)
}

func tomlSectionBounds(lines []string, section string) (int, int, bool) {
	if section == "" {
		for index, line := range lines {
			if _, ok := tomlSectionName(line); ok {
				return 0, index, true
			}
		}
		return 0, len(lines), true
	}
	for index, line := range lines {
		name, ok := tomlSectionName(line)
		if !ok || name != section {
			continue
		}
		end := len(lines)
		for next := index + 1; next < len(lines); next++ {
			if _, ok := tomlSectionName(lines[next]); ok {
				end = next
				break
			}
		}
		return index + 1, end, true
	}
	return len(lines), len(lines), false
}

func tomlSectionName(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "#") || !strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "[[") {
		return "", false
	}
	end := strings.Index(trimmed, "]")
	if end <= 1 {
		return "", false
	}
	return strings.TrimSpace(trimmed[1:end]), true
}

func tomlLineKey(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "[") {
		return ""
	}
	key, _, ok := strings.Cut(trimmed, "=")
	if !ok {
		return ""
	}
	return strings.TrimSpace(key)
}

func insertLine(lines []string, index int, line string) []string {
	if index < 0 {
		index = 0
	}
	if index > len(lines) {
		index = len(lines)
	}
	lines = append(lines, "")
	copy(lines[index+1:], lines[index:])
	lines[index] = line
	return lines
}

func tomlString(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
		"\r", `\r`,
		"\t", `\t`,
	)
	return `"` + replacer.Replace(value) + `"`
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
	tokenRouterBaseURL := fmt.Sprintf("http://127.0.0.1:%d/tokenrouter/v1", port)
	zoBaseURL := fmt.Sprintf("http://127.0.0.1:%d/zo/v1", port)
	customBaseURL := fmt.Sprintf("http://127.0.0.1:%d/custom/v1", port)
	openRouterModels := a.openCodeOpenRouterModels()

	if err := writeOpenCodeConfig(configPath, routerBaseURL, geminiBaseURL, openRouterBaseURL, tokenRouterBaseURL, zoBaseURL, customBaseURL, openRouterModels); err != nil {
		return clientConfigureResult{}, err
	}

	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "opencode configured for omniproxy"})
	return clientConfigureResult{
		ConfigPath: configPath,
		BackupPath: configPath + ".omniproxy.bak",
		BaseURL:    routerBaseURL,
		ProviderID: opencodeOmniProviderID,
		Message:    "OpenCode 已添加 OmniProxy 网关 provider，后端平台请在网关路由中选择",
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
	zoBaseURL := fmt.Sprintf("http://127.0.0.1:%d/zo/v1", port)
	openRouterModels := a.piOpenRouterModels()

	if err := writePiModelsConfig(configPath, routerBaseURL, zoBaseURL, openRouterModels); err != nil {
		return clientConfigureResult{}, err
	}

	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "pi configured for omniproxy"})
	return clientConfigureResult{
		ConfigPath: configPath,
		BackupPath: configPath + ".omniproxy.bak",
		BaseURL:    routerBaseURL,
		ProviderID: piOmniProviderID,
		Message:    "Pi Coding Agent 已添加 OmniProxy 网关 provider，后端平台请在网关路由中选择",
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

func writeOpenCodeConfig(path string, routerBaseURL string, geminiBaseURL string, openRouterBaseURL string, tokenRouterBaseURL string, zoBaseURL string, customBaseURL string, openRouterModels map[string]any) error {
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
	for _, id := range []string{opencodeGeminiProviderID, opencodeOpenRouterProviderID, opencodeTokenRouterProviderID, opencodeZoProviderID, opencodeCustomProviderID} {
		delete(providers, id)
	}
	providers[opencodeOmniProviderID] = openCodeRouterProvider(routerBaseURL, openRouterModels)
	data["provider"] = providers

	return writeJSONObject(path, data)
}

func writePiModelsConfig(path string, routerBaseURL string, zoBaseURL string, openRouterModels []map[string]any) error {
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
	for _, id := range []string{piAnthropicProviderID, piGeminiProviderID, piOpenRouterProviderID, piZoProviderID, piCustomProviderID} {
		delete(providers, id)
	}
	providers[piOmniProviderID] = piOpenAIProvider("OmniProxy", routerBaseURL, piRouterModels(openRouterModels), true)
	data["providers"] = providers

	return writeJSONObject(path, data)
}

func piRouterModels(openRouterModels []map[string]any) []map[string]any {
	models := []map[string]any{
		{"id": "gpt-5.4", "name": "GPT-5.4"},
		{"id": "deepseek-v4-pro", "name": "DeepSeek V4 Pro"},
		{"id": "kimi-for-coding", "name": "Kimi for Coding"},
		{"id": "glm-5.1", "name": "GLM-5.1"},
		{"id": "MiniMax-M2.7", "name": "MiniMax M2.7"},
		{"id": "auto:balance", "name": "TokenRouter Auto Balance"},
		{"id": "auto:quality", "name": "TokenRouter Auto Quality"},
		{"id": "auto:speed", "name": "TokenRouter Auto Speed"},
		{"id": "auto:cost", "name": "TokenRouter Auto Cost"},
		piReasoningModel(mimoModel, "MiMo V2.5 Pro"),
		{"id": "custom-model", "name": "Custom Gateway Model"},
	}
	seen := map[string]bool{}
	for _, model := range models {
		if id, ok := model["id"].(string); ok {
			seen[strings.ToLower(strings.TrimSpace(id))] = true
		}
	}
	for _, model := range openRouterModels {
		id, _ := model["id"].(string)
		key := strings.ToLower(strings.TrimSpace(id))
		if key == "" || seen[key] {
			continue
		}
		models = append(models, model)
		seen[key] = true
	}
	if !seen["openrouter/auto"] {
		models = append(models, defaultPiOpenRouterModels()...)
	}
	return models
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

func openCodeRouterProvider(baseURL string, extraModels map[string]any) map[string]any {
	models := map[string]any{
		"gpt-5.4":         map[string]any{"name": "GPT-5.4"},
		"deepseek-v4-pro": map[string]any{"name": "DeepSeek V4 Pro"},
		"glm-5.1":         map[string]any{"name": "GLM-5.1"},
		"MiniMax-M2.7":    map[string]any{"name": "MiniMax M2.7"},
		"mimo-v2.5-pro":   map[string]any{"name": "MiMo V2.5 Pro"},
		"auto:balance":    map[string]any{"name": "TokenRouter Auto Balance"},
		"custom-model":    map[string]any{"name": "Custom Gateway Model"},
	}
	for id, model := range extraModels {
		if strings.TrimSpace(id) == "" {
			continue
		}
		if _, exists := models[id]; exists {
			continue
		}
		models[id] = model
	}
	return map[string]any{
		"npm":  "@ai-sdk/openai-compatible",
		"name": "OmniProxy",
		"options": map[string]any{
			"baseURL":     baseURL,
			"apiKey":      localClientAPIKey,
			"setCacheKey": true,
		},
		"models": models,
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

func openCodeTokenRouterProvider(baseURL string) map[string]any {
	return map[string]any{
		"npm":  "@ai-sdk/openai-compatible",
		"name": "OmniProxy TokenRouter",
		"options": map[string]any{
			"baseURL":     baseURL,
			"apiKey":      localClientAPIKey,
			"setCacheKey": true,
		},
		"models": map[string]any{
			"auto:balance": map[string]any{"name": "Auto Balance"},
			"auto:quality": map[string]any{"name": "Auto Quality"},
			"auto:speed":   map[string]any{"name": "Auto Speed"},
			"auto:cost":    map[string]any{"name": "Auto Cost"},
		},
	}
}

func openCodeZoProvider(baseURL string) map[string]any {
	return map[string]any{
		"npm":  "@ai-sdk/openai-compatible",
		"name": "OmniProxy Zo Computer",
		"options": map[string]any{
			"baseURL":     baseURL,
			"apiKey":      localClientAPIKey,
			"setCacheKey": true,
		},
		"models": map[string]any{
			"claude-opus-4-7":   map[string]any{"name": "Zo Claude Opus 4.7"},
			"claude-sonnet-4-6": map[string]any{"name": "Zo Claude Sonnet 4.6"},
			"gemini-3.1-pro":    map[string]any{"name": "Zo Gemini 3.1 Pro"},
			"glm-5":             map[string]any{"name": "Zo GLM 5"},
			"minimax-2.7":       map[string]any{"name": "Zo MiniMax 2.7"},
			"gpt-5.4":           map[string]any{"name": "Zo GPT-5.4"},
			"gpt-5.4-mini":      map[string]any{"name": "Zo GPT-5.4 mini"},
			"gpt-5.5":           map[string]any{"name": "Zo GPT-5.5"},
			"deepseek-v4-pro":   map[string]any{"name": "Zo DeepSeek V4 Pro"},
		},
	}
}

func piZoModels() []map[string]any {
	return []map[string]any{
		{"id": "claude-opus-4-7", "name": "Zo Claude Opus 4.7"},
		{"id": "claude-sonnet-4-6", "name": "Zo Claude Sonnet 4.6"},
		{"id": "gemini-3.1-pro", "name": "Zo Gemini 3.1 Pro"},
		{"id": "glm-5", "name": "Zo GLM 5"},
		{"id": "minimax-2.7", "name": "Zo MiniMax 2.7"},
		{"id": "gpt-5.4", "name": "Zo GPT-5.4"},
		{"id": "gpt-5.4-mini", "name": "Zo GPT-5.4 mini"},
		{"id": "gpt-5.5", "name": "Zo GPT-5.5"},
		{"id": "deepseek-v4-pro", "name": "Zo DeepSeek V4 Pro"},
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
