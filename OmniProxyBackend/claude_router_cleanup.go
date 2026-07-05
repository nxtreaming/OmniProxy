package main

import (
	"errors"
	"os"
	"strings"

	"omniproxy/internal/clientconfig"
)

func removeFileIfExists(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func cleanClaudeEnv(data map[string]any) map[string]any {
	env, _ := data["env"].(map[string]any)
	if env == nil {
		env = map[string]any{}
	}
	removeClaudeRouterSettings(data, env)
	return env
}

func claudeRouterAvailableModels() []string {
	return []string{
		"best",
		"opus",
		"opus[1m]",
		"opusplan",
		"sonnet",
		"sonnet[1m]",
		"haiku",
		"claude-opus-4-7",
		"claude-opus-4-6",
		"claude-opus-4-5-20251101",
		"claude-opus-4-1-20250422",
		"claude-sonnet-4-6",
		"claude-sonnet-4-5-20250929",
		"claude-sonnet-4-20250514",
		"claude-haiku-4-5-20251001",
		mimoModel,
		mimoLongContextModel,
		mimoStandardModel,
		deepSeekProModel,
		deepSeekProLegacy,
		deepSeekFastModel,
		kimiCodingModel,
		zhipuGLMModel,
		premClaudeModel,
	}
}

func claudeRouterModelOverrides() map[string]string {
	return map[string]string{
		"claude-opus-4-7":            mimoLongContextModel,
		"claude-opus-4-6":            mimoStandardModel,
		"claude-opus-4-5-20251101":   deepSeekProModel,
		"claude-opus-4-1-20250422":   mimoStandardModel,
		"claude-sonnet-4-6":          deepSeekFastModel,
		"claude-sonnet-4-5-20250929": deepSeekProModel,
		"claude-sonnet-4-20250514":   mimoStandardModel,
		"claude-haiku-4-5-20251001":  deepSeekFastModel,
	}
}

func clearKnownRouterModelValue(env map[string]any, key string) {
	value, ok := env[key].(string)
	if ok && isKnownRouterDefaultModel(value) {
		delete(env, key)
	}
}

func removeClaudeRouterSettings(data map[string]any, env map[string]any) {
	clearKnownRouterModelValue(env, "ANTHROPIC_MODEL")
	clearKnownRouterModelValue(env, "CLAUDE_CODE_SUBAGENT_MODEL")
	clearKnownRouterModelGroup(env, "ANTHROPIC_DEFAULT_OPUS_MODEL")
	clearKnownRouterModelGroup(env, "ANTHROPIC_DEFAULT_SONNET_MODEL")
	clearKnownRouterModelGroup(env, "ANTHROPIC_DEFAULT_HAIKU_MODEL")
	clearKnownRouterModelGroup(env, "ANTHROPIC_CUSTOM_MODEL_OPTION")
	clearDeepSeekEffortOverride(env)
	removeClaudeRouterAvailableModels(data)
	removeClaudeRouterModelOverrides(data)
}

func removeClaudeRouterAvailableModels(data map[string]any) {
	existing, ok := data["availableModels"].([]any)
	if !ok {
		return
	}

	known := map[string]bool{}
	for _, model := range claudeRouterAvailableModels() {
		known[model] = true
	}

	filtered := []string{}
	for _, value := range existing {
		text, ok := value.(string)
		if !ok {
			continue
		}
		text = strings.TrimSpace(text)
		if text == "" || known[text] {
			continue
		}
		filtered = append(filtered, text)
	}
	if len(filtered) == 0 {
		delete(data, "availableModels")
		return
	}
	data["availableModels"] = filtered
}

func removeClaudeRouterModelOverrides(data map[string]any) {
	existing, ok := data["modelOverrides"].(map[string]any)
	if !ok {
		return
	}

	for key, value := range claudeRouterModelOverrides() {
		if existingValue, ok := existing[key].(string); ok && isKnownRouterOverrideValue(value, existingValue) {
			delete(existing, key)
		}
	}
	if len(existing) == 0 {
		delete(data, "modelOverrides")
	}
}

func isKnownRouterOverrideValue(current string, existing string) bool {
	existing = strings.TrimSpace(existing)
	if strings.EqualFold(existing, current) {
		return true
	}
	if strings.EqualFold(current, deepSeekProModel) && strings.EqualFold(existing, deepSeekProLegacy) {
		return true
	}
	return strings.EqualFold(current, mimoLongContextModel) && strings.EqualFold(existing, mimoModel)
}

func clearKnownRouterModelGroup(env map[string]any, key string) {
	value, ok := env[key].(string)
	if !ok || !isKnownRouterDefaultModel(value) {
		return
	}
	delete(env, key)
	delete(env, key+"_NAME")
	delete(env, key+"_DESCRIPTION")
	delete(env, key+"_SUPPORTED_CAPABILITIES")
}

func clearDeepSeekEffortOverride(env map[string]any) {
	value, ok := env["CLAUDE_CODE_EFFORT_LEVEL"].(string)
	if ok && strings.EqualFold(strings.TrimSpace(value), "max") {
		delete(env, "CLAUDE_CODE_EFFORT_LEVEL")
	}
}

func isKnownRouterDefaultModel(value string) bool {
	model := strings.TrimSpace(value)
	return strings.EqualFold(model, mimoModel) ||
		strings.EqualFold(model, mimoLongContextModel) ||
		strings.EqualFold(model, mimoStandardModel) ||
		strings.EqualFold(model, deepSeekProModel) ||
		strings.EqualFold(model, deepSeekProLegacy) ||
		strings.EqualFold(model, deepSeekFastModel) ||
		strings.EqualFold(model, kimiCodingModel) ||
		strings.EqualFold(model, zhipuGLMModel) ||
		strings.EqualFold(model, anyRouterClaudeModel) ||
		strings.EqualFold(model, zoClaudeModel) ||
		strings.EqualFold(model, zoClaudeSonnetModel) ||
		strings.EqualFold(model, premClaudeModel)
}

func writeMimoClaudeOnboarding(path string) error {
	data, err := clientconfig.ReadJSONObject(path)
	if err != nil {
		return err
	}
	if err := clientconfig.BackupFile(path, path+".omniproxy.bak", []byte("{}\n")); err != nil {
		return err
	}
	data["hasCompletedOnboarding"] = true
	return clientconfig.WriteJSONObject(path, data)
}
