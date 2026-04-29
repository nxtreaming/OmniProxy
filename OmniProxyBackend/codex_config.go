package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"OmniProxyBackend/internal/logs"
	"OmniProxyBackend/internal/token"
)

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

	baseURL := fmt.Sprintf("http://127.0.0.1:%d/backend-api/codex", cfg.ProxyPort)
	configPath := filepath.Join(codexDir, "config.toml")
	if err := writeCodexOmniProxyConfig(configPath, baseURL); err != nil {
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

	parts := []string{"Codex 已切换到 OmniProxy 本地代理"}
	if result.ImportedAuth {
		parts = append(parts, "已导入 auth.json")
	} else if result.AuthUpdated {
		parts = append(parts, "已同步 auth.json")
	} else if result.AuthAlreadyAdded {
		parts = append(parts, "auth.json 账号已存在")
	} else {
		parts = append(parts, "未找到 auth.json，请先运行 codex login 或手动添加账号")
	}
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
	lines = removeRootKey(lines, "chatgpt_base_url")
	lines = removeTomlSection(lines, "[model_providers.openai]")
	lines = removeTomlSection(lines, "[model_providers.omniproxy]")

	next := strings.Join(lines, "\n") + "\n"
	if len(content) > 0 {
		backupPath := path + ".omniproxy.bak"
		if _, err := os.Stat(backupPath); errors.Is(err, os.ErrNotExist) {
			_ = os.WriteFile(backupPath, content, 0o600)
		}
	}
	return os.WriteFile(path, []byte(next), 0o600)
}

func setRootStringKey(lines []string, key string, value string) []string {
	replacement := fmt.Sprintf(`%s = "%s"`, key, tomlEscape(value))
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
