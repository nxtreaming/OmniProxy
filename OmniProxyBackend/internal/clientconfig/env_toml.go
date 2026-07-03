package clientconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const deepSeekTUIClientHeader = "DeepSeek-TUI"

func WriteGeminiEnv(path string, baseURL string, apiKey string, model string) error {
	env, err := ReadEnvFile(path)
	if err != nil {
		return err
	}
	if err := BackupFile(path, path+".omniproxy.bak", []byte("\n")); err != nil {
		return err
	}

	env["GOOGLE_GEMINI_BASE_URL"] = baseURL
	env["GEMINI_API_KEY"] = apiKey
	env["GEMINI_MODEL"] = model
	return WriteEnvFile(path, env)
}

func WriteGeminiSettings(path string, selectedType string) error {
	data, err := ReadJSONObject(path)
	if err != nil {
		return err
	}
	if err := BackupFile(path, path+".omniproxy.bak", []byte("{}\n")); err != nil {
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
	return WriteJSONObject(path, data)
}

func ReadEnvFile(path string) (map[string]string, error) {
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

func WriteEnvFile(path string, env map[string]string) error {
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

func WriteDeepSeekTUIConfig(path string, baseURL string, apiKey string, model string) error {
	if err := BackupFile(path, path+".omniproxy.bak", []byte("\n")); err != nil {
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
