package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"OmniProxyBackend/internal/logs"
	"OmniProxyBackend/internal/token"
)

const codexSub2APILocalAPIKey = "sk-omniproxy-local-sub2api"

type codexConfigureResult struct {
	ConfigPath       string `json:"configPath"`
	AuthPath         string `json:"authPath"`
	BackupPath       string `json:"backupPath"`
	BaseURL          string `json:"baseUrl"`
	ImportedAuth     bool   `json:"importedAuth"`
	AuthAlreadyAdded bool   `json:"authAlreadyAdded"`
	AuthUpdated      bool   `json:"authUpdated"`
	Message          string `json:"message"`
}

func (a *appServer) configureCodex() (codexConfigureResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return codexConfigureResult{}, err
	}

	a.mu.Lock()
	cfg := a.cfg
	a.mu.Unlock()

	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		return codexConfigureResult{}, err
	}

	baseURL := fmt.Sprintf("http://127.0.0.1:%d/codex/v1", cfg.ProxyPort)
	model := strings.TrimSpace(cfg.GatewayRoutes.Codex.Model)
	if model == "" {
		model = "gpt-5.4"
	}
	configPath := filepath.Join(codexDir, "config.toml")
	if err := writeCodexOpenAIResponsesConfig(configPath, baseURL, model); err != nil {
		return codexConfigureResult{}, err
	}

	result := codexConfigureResult{
		ConfigPath: configPath,
		AuthPath:   filepath.Join(codexDir, "auth.json"),
		BackupPath: configPath + ".omniproxy.bak",
		BaseURL:    baseURL,
	}

	authBytes, err := os.ReadFile(result.AuthPath)
	authValue := strings.TrimSpace(string(authBytes))
	switch {
	case err == nil:
		fields, ok := token.ExtractCodexAuthFields(authValue)
		if !ok || strings.TrimSpace(fields.Email) == "" || !fields.HasSupportedToken() {
			a.logs.Add(logs.Entry{Level: logs.LevelWarn, Message: "codex auth skipped: auth.json is not importable Codex auth"})
			break
		}
		req := token.UpsertRequest{
			Provider:       token.ProviderOpenAI,
			CredentialType: token.CredentialTypeCodexAuthJSON,
			TokenValue:     authValue,
		}
		item, addErr := a.tokens.Add(req)
		if addErr == nil {
			result.ImportedAuth = true
			a.logs.Add(logs.Entry{Level: logs.LevelInfo, TokenName: item.Name, Message: "codex auth imported"})
		} else if errors.Is(addErr, token.ErrDuplicateName) {
			email, ok := token.ExtractCodexEmail(string(authBytes))
			if !ok {
				return codexConfigureResult{}, addErr
			}
			existing, findErr := a.tokens.FindByName(token.ProviderOpenAI, email)
			if findErr != nil {
				return codexConfigureResult{}, findErr
			}
			if existing.CredentialType != token.CredentialTypeCodexAuthJSON {
				return codexConfigureResult{}, addErr
			}
			if strings.TrimSpace(existing.TokenValue) == authValue {
				result.AuthAlreadyAdded = true
				a.logs.Add(logs.Entry{Level: logs.LevelInfo, TokenName: existing.Name, Message: "codex auth already imported"})
				break
			}
			updated, updateErr := a.tokens.Update(existing.ID, req)
			if updateErr != nil {
				return codexConfigureResult{}, updateErr
			}
			result.AuthUpdated = true
			a.logs.Add(logs.Entry{Level: logs.LevelInfo, TokenName: updated.Name, Message: "codex auth synced"})
		} else {
			return codexConfigureResult{}, addErr
		}
	case errors.Is(err, os.ErrNotExist):
	default:
		return codexConfigureResult{}, err
	}

	authState, err := ensureCodexOpenAIAPIKey(result.AuthPath)
	if err != nil {
		return codexConfigureResult{}, err
	}
	switch authState {
	case "created":
		if !result.ImportedAuth && !result.AuthAlreadyAdded {
			result.AuthUpdated = true
		}
	case "updated":
		if !result.ImportedAuth && !result.AuthAlreadyAdded {
			result.AuthUpdated = true
		}
	}

	parts := []string{"Codex 已切换到 OmniProxy 网关入口"}
	if result.ImportedAuth {
		parts = append(parts, "已导入 auth.json")
	} else if result.AuthUpdated {
		parts = append(parts, "已同步 auth.json / 本地占位 OPENAI_API_KEY")
	} else if result.AuthAlreadyAdded {
		parts = append(parts, "auth.json 账号已存在")
	} else {
		parts = append(parts, "未找到可导入的 Codex auth.json，请先运行 codex login 或手动添加账号")
	}
	parts = append(parts, "后端平台请在 OmniProxy 网关路由中选择")
	result.Message = strings.Join(parts, "；")
	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "codex configured for omniproxy"})
	return result, nil
}

func (a *appServer) restoreCodexConfig() (codexConfigureResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return codexConfigureResult{}, err
	}

	codexDir := filepath.Join(home, ".codex")
	configPath := filepath.Join(codexDir, "config.toml")
	backupPath := configPath + ".omniproxy.bak"
	backup, err := os.ReadFile(backupPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return codexConfigureResult{}, errors.New("未找到 Codex 原始配置备份")
		}
		return codexConfigureResult{}, err
	}
	if err := os.WriteFile(configPath, backup, 0o600); err != nil {
		return codexConfigureResult{}, err
	}

	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "codex config restored"})
	return codexConfigureResult{
		ConfigPath: configPath,
		AuthPath:   filepath.Join(codexDir, "auth.json"),
		BackupPath: backupPath,
		Message:    "Codex 配置已恢复到一键配置前的原始配置",
	}, nil
}

func writeCodexOmniProxyConfig(path string, baseURL string) error {
	content, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	lines := []string{}
	if text != "" {
		lines = strings.Split(strings.TrimRight(text, "\n"), "\n")
	}

	lines = setRootStringKey(lines, "model_provider", "openai")
	lines = setRootStringKey(lines, "openai_base_url", baseURL)
	lines = removeRootKey(lines, "preferred_auth_method")
	lines = removeRootKey(lines, "chatgpt_base_url")
	lines = removeRootKey(lines, "disable_response_storage")
	lines = removeTomlSection(lines, "[model_providers.openai]")
	lines = removeTomlSection(lines, "[model_providers.omniproxy]")
	lines = removeTomlSection(lines, "[model_providers.anyrouter]")
	lines = removeTomlSection(lines, "[model_providers.prem]")

	next := strings.Join(lines, "\n") + "\n"
	if len(content) > 0 {
		backupPath := path + ".omniproxy.bak"
		if _, err := os.Stat(backupPath); errors.Is(err, os.ErrNotExist) {
			_ = os.WriteFile(backupPath, content, 0o600)
		}
	}
	return os.WriteFile(path, []byte(next), 0o600)
}

func writeCodexOpenAIResponsesConfig(path string, baseURL string, model string) error {
	content, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	lines := []string{}
	if text != "" {
		lines = strings.Split(strings.TrimRight(text, "\n"), "\n")
	}

	lines = setRootStringKey(lines, "model_provider", "OpenAI")
	lines = setRootStringKey(lines, "model", model)
	lines = setRootStringKey(lines, "review_model", model)
	lines = setRootStringKey(lines, "model_reasoning_effort", "xhigh")
	lines = setRootStringKey(lines, "network_access", "enabled")
	lines = setRootRawKey(lines, "windows_wsl_setup_acknowledged", "true")
	lines = setRootRawKey(lines, "model_context_window", "1000000")
	lines = setRootRawKey(lines, "model_auto_compact_token_limit", "900000")
	lines = removeRootKey(lines, "preferred_auth_method")
	lines = removeRootKey(lines, "openai_base_url")
	lines = removeRootKey(lines, "chatgpt_base_url")
	lines = removeRootKey(lines, "disable_response_storage")
	lines = removeTomlSection(lines, "[model_providers.openai]")
	lines = removeTomlSection(lines, "[model_providers.OpenAI]")
	lines = removeTomlSection(lines, "[model_providers.omniproxy]")
	lines = removeTomlSection(lines, "[model_providers.sub2api]")
	lines = removeTomlSection(lines, "[model_providers.newapi]")
	lines = removeTomlSection(lines, "[model_providers.zo]")
	lines = removeTomlSection(lines, "[model_providers.anyrouter]")
	lines = removeTomlSection(lines, "[model_providers.prem]")
	lines = appendTomlSection(lines, []string{
		"[model_providers.OpenAI]",
		`name = "OpenAI"`,
		fmt.Sprintf(`base_url = "%s"`, tomlEscape(baseURL)),
		`wire_api = "responses"`,
		"requires_openai_auth = true",
	})

	next := strings.Join(lines, "\n") + "\n"
	if len(content) > 0 {
		backupPath := path + ".omniproxy.bak"
		if _, err := os.Stat(backupPath); errors.Is(err, os.ErrNotExist) {
			_ = os.WriteFile(backupPath, content, 0o600)
		}
	}
	return os.WriteFile(path, []byte(next), 0o600)
}

func ensureCodexOpenAIAPIKey(path string) (string, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		data, marshalErr := json.MarshalIndent(map[string]any{
			"OPENAI_API_KEY": codexSub2APILocalAPIKey,
		}, "", "  ")
		if marshalErr != nil {
			return "", marshalErr
		}
		return "created", os.WriteFile(path, append(data, '\n'), 0o600)
	}
	if err != nil {
		return "", err
	}

	var payload map[string]any
	if err := json.Unmarshal(content, &payload); err != nil {
		return "", err
	}
	if value, ok := payload["OPENAI_API_KEY"].(string); ok && strings.TrimSpace(value) != "" {
		return "existing", nil
	}
	payload["OPENAI_API_KEY"] = codexSub2APILocalAPIKey
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	return "updated", os.WriteFile(path, append(data, '\n'), 0o600)
}

func setRootStringKey(lines []string, key string, value string) []string {
	replacement := fmt.Sprintf(`%s = "%s"`, key, tomlEscape(value))
	return setRootLine(lines, key, replacement)
}

func setRootRawKey(lines []string, key string, value string) []string {
	return setRootLine(lines, key, fmt.Sprintf("%s = %s", key, value))
}

func setRootLine(lines []string, key string, replacement string) []string {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			break
		}
		if tomlKey(trimmed) == key {
			lines[i] = replacement
			return lines
		}
	}

	insertAt := len(lines)
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "[") {
			insertAt = i
			break
		}
	}

	next := make([]string, 0, len(lines)+2)
	next = append(next, lines[:insertAt]...)
	next = append(next, replacement)
	if insertAt < len(lines) && strings.TrimSpace(lines[insertAt]) != "" {
		next = append(next, "")
	}
	next = append(next, lines[insertAt:]...)
	return next
}

func appendTomlSection(lines []string, section []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > 0 {
		lines = append(lines, "")
	}
	return append(lines, section...)
}

func removeTomlSection(lines []string, section string) []string {
	next := make([]string, 0, len(lines))
	skip := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.EqualFold(trimmed, section) {
			skip = true
			continue
		}
		if skip && strings.HasPrefix(trimmed, "[") {
			skip = false
		}
		if !skip {
			next = append(next, line)
		}
	}
	return next
}

func tomlKey(line string) string {
	if line == "" || strings.HasPrefix(line, "#") {
		return ""
	}
	beforeValue, _, ok := strings.Cut(line, "=")
	if !ok {
		return ""
	}
	return strings.TrimSpace(beforeValue)
}

func tomlEscape(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `"`, `\"`)
}

func removeRootKey(lines []string, key string) []string {
	next := make([]string, 0, len(lines))
	inRoot := true
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			inRoot = false
		}
		if inRoot && tomlKey(trimmed) == key {
			continue
		}
		next = append(next, line)
	}
	return next
}
