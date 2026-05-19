package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"OmniProxyBackend/internal/claudedesktop"
	"OmniProxyBackend/internal/logs"
)

const (
	mimoModel            = "mimo-v2.5-pro"
	mimoLongContextModel = "mimo-v2.5-pro[1m]"
	mimoStandardModel    = "mimo-v2.5"
	deepSeekProModel     = "deepseek-v4-pro[1m]"
	deepSeekFastModel    = "deepseek-v4-flash"
	kimiCodingModel      = "kimi-for-coding"
	zhipuGLMModel        = "glm-5.1"
	omniProxyMimoAuth    = "omniproxy"
	maxClaudeModels      = 4
)

type mimoConfigureResult struct {
	ConfigPath    string   `json:"configPath,omitempty"`
	SettingsPath  string   `json:"settingsPath,omitempty"`
	ClaudePath    string   `json:"claudePath,omitempty"`
	BackupPath    string   `json:"backupPath,omitempty"`
	BaseURL       string   `json:"baseUrl,omitempty"`
	Model         string   `json:"model,omitempty"`
	Models        []string `json:"models,omitempty"`
	EnvConfigured bool     `json:"envConfigured,omitempty"`
	Message       string   `json:"message"`
}

type claudeModelsConfigureRequest struct {
	Models []string `json:"models"`
}

type claudeModelTarget struct {
	Model       string
	Name        string
	Description string
	LogMessage  string
	Message     string
}

var (
	claudeMimoTarget = claudeModelTarget{
		Model:       mimoLongContextModel,
		Name:        "MiMo-V2.5-Pro [1m]",
		Description: "Xiaomi MiMo-V2.5-Pro 1M context routed through OmniProxy",
		LogMessage:  "mimo claude configured",
		Message:     "Claude Code 已配置为通过 OmniProxy 使用 Xiaomi MiMo",
	}
	claudeDeepSeekTarget = claudeModelTarget{
		Model:       deepSeekProModel,
		Name:        "DeepSeek V4 Pro",
		Description: "DeepSeek V4 Pro routed through OmniProxy",
		LogMessage:  "deepseek claude configured",
		Message:     "Claude Code 已配置为通过 OmniProxy 使用 DeepSeek",
	}
	claudeKimiTarget = claudeModelTarget{
		Model:       kimiCodingModel,
		Name:        "Kimi for Coding",
		Description: "Kimi Code routed through OmniProxy",
		LogMessage:  "kimi claude configured",
		Message:     "Claude Code 已配置为通过 OmniProxy 使用 Kimi",
	}
	claudeZhipuTarget = claudeModelTarget{
		Model:       zhipuGLMModel,
		Name:        "GLM-5.1",
		Description: "Zhipu GLM-5.1 routed through OmniProxy",
		LogMessage:  "zhipu claude configured",
		Message:     "Claude Code 已配置为通过 OmniProxy 使用 Zhipu GLM",
	}
)

type claudeModelSelectionError struct {
	message string
}

func (e *claudeModelSelectionError) Error() string {
	return e.message
}

func (a *appServer) configureMimoClaude() (mimoConfigureResult, error) {
	return a.configureClaudeWithWriter(claudeMimoTarget, func(path string, baseURL string) error {
		return writeMimoClaudeSettings(path, baseURL)
	})
}

func (a *appServer) configureDeepSeekClaude() (mimoConfigureResult, error) {
	return a.configureClaudeWithWriter(claudeDeepSeekTarget, func(path string, baseURL string) error {
		return writeDeepSeekClaudeSettings(path, baseURL)
	})
}

func (a *appServer) configureKimiClaude() (mimoConfigureResult, error) {
	return a.configureClaudeWithWriter(claudeKimiTarget, func(path string, baseURL string) error {
		return writeKimiClaudeSettings(path, baseURL)
	})
}

func (a *appServer) configureZhipuClaude() (mimoConfigureResult, error) {
	return a.configureClaudeWithWriter(claudeZhipuTarget, func(path string, baseURL string) error {
		return writeZhipuClaudeSettings(path, baseURL)
	})
}

func (a *appServer) configureClaudeModels(req claudeModelsConfigureRequest) (mimoConfigureResult, error) {
	return a.configureSelectedClaudeModels(req, "selected claude models configured", func(targets []claudeModelTarget) string {
		return fmt.Sprintf("Claude Code 已配置 %d 个模型：%s", len(targets), strings.Join(claudeModelNames(targets), "、"))
	})
}

func (a *appServer) configureClaudeDesktopModels(req claudeModelsConfigureRequest) (mimoConfigureResult, error) {
	targets, err := normalizeClaudeModelTargets(req.Models)
	if err != nil {
		return mimoConfigureResult{}, err
	}
	a.mu.Lock()
	baseURL := claudedesktop.GatewayBaseURL(a.cfg.ProxyPort)
	a.mu.Unlock()

	paths, err := claudedesktop.CurrentPaths()
	if err != nil {
		return mimoConfigureResult{}, err
	}
	routes := claudeDesktopRoutesForTargets(targets)
	if err := writeClaudeDesktopProfile(paths, baseURL, routes); err != nil {
		return mimoConfigureResult{}, err
	}

	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "claude desktop models configured"})
	return mimoConfigureResult{
		ConfigPath: paths.ProfilePath,
		BackupPath: paths.MetaPath,
		BaseURL:    baseURL,
		Model:      targets[0].Model,
		Models:     claudeModelIDs(targets),
		Message:    fmt.Sprintf("Claude Code Desktop 已配置 %d 个模型：%s，请完全退出并重启 Claude Desktop", len(targets), strings.Join(claudeModelNames(targets), "、")),
	}, nil
}

func (a *appServer) configureSelectedClaudeModels(req claudeModelsConfigureRequest, logMessage string, message func([]claudeModelTarget) string) (mimoConfigureResult, error) {
	targets, err := normalizeClaudeModelTargets(req.Models)
	if err != nil {
		return mimoConfigureResult{}, err
	}

	models := claudeModelIDs(targets)
	target := claudeModelTarget{
		Model:      targets[0].Model,
		LogMessage: logMessage,
		Message:    message(targets),
	}
	result, err := a.configureClaudeWithWriter(target, func(path string, baseURL string) error {
		return writeSelectedClaudeSettings(path, baseURL, targets)
	})
	if err != nil {
		return mimoConfigureResult{}, err
	}
	result.Models = models
	return result, nil
}

func (a *appServer) configureClaudeWithWriter(target claudeModelTarget, writeSettings func(string, string) error) (mimoConfigureResult, error) {
	a.mu.Lock()
	baseURL := fmt.Sprintf("http://127.0.0.1:%d/anthropic-router", a.cfg.ProxyPort)
	a.mu.Unlock()

	return a.configureClaudeWithBaseURL(target, baseURL, writeSettings)
}

func (a *appServer) configureClaudeWithBaseURL(target claudeModelTarget, baseURL string, writeSettings func(string, string) error) (mimoConfigureResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return mimoConfigureResult{}, err
	}

	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return mimoConfigureResult{}, err
	}

	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := writeSettings(settingsPath, baseURL); err != nil {
		return mimoConfigureResult{}, err
	}

	claudePath := filepath.Join(home, ".claude.json")
	if err := writeMimoClaudeOnboarding(claudePath); err != nil {
		return mimoConfigureResult{}, err
	}

	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: target.LogMessage})
	return mimoConfigureResult{
		SettingsPath: settingsPath,
		ClaudePath:   claudePath,
		BackupPath:   settingsPath + ".omniproxy.bak",
		BaseURL:      baseURL,
		Model:        target.Model,
		Message:      target.Message,
	}, nil
}

func (a *appServer) restoreMimoClaudeConfig() (mimoConfigureResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return mimoConfigureResult{}, err
	}

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := restoreBackup(settingsPath, settingsPath+".omniproxy.bak"); err != nil {
		return mimoConfigureResult{}, err
	}
	claudePath := filepath.Join(home, ".claude.json")
	if err := restoreBackup(claudePath, claudePath+".omniproxy.bak"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return mimoConfigureResult{}, err
	}

	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "mimo claude config restored"})
	return mimoConfigureResult{
		SettingsPath: settingsPath,
		ClaudePath:   claudePath,
		BackupPath:   settingsPath + ".omniproxy.bak",
		Message:      "Claude Code 配置已恢复",
	}, nil
}

func (a *appServer) restoreDeepSeekClaudeConfig() (mimoConfigureResult, error) {
	return a.restoreMimoClaudeConfig()
}

func (a *appServer) restoreKimiClaudeConfig() (mimoConfigureResult, error) {
	return a.restoreMimoClaudeConfig()
}

func (a *appServer) restoreZhipuClaudeConfig() (mimoConfigureResult, error) {
	return a.restoreMimoClaudeConfig()
}

func (a *appServer) restoreClaudeDesktopConfig() (mimoConfigureResult, error) {
	paths, err := claudedesktop.CurrentPaths()
	if err != nil {
		return mimoConfigureResult{}, err
	}
	if err := writeClaudeDesktopDeploymentMode(paths.NormalConfigPath, "1p"); err != nil {
		return mimoConfigureResult{}, err
	}
	if err := writeClaudeDesktopDeploymentMode(paths.ThreePConfigPath, "1p"); err != nil {
		return mimoConfigureResult{}, err
	}
	if err := removeFileIfExists(paths.ProfilePath); err != nil {
		return mimoConfigureResult{}, err
	}
	if err := removeFileIfExists(paths.RoutesPath); err != nil {
		return mimoConfigureResult{}, err
	}
	if err := writeClaudeDesktopMeta(paths.MetaPath, false); err != nil {
		return mimoConfigureResult{}, err
	}

	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "claude desktop config restored"})
	return mimoConfigureResult{
		ConfigPath: paths.ProfilePath,
		BackupPath: paths.MetaPath,
		Message:    "Claude Desktop 配置已恢复为官方模式",
	}, nil
}

func writeMimoClaudeSettings(path string, baseURL string) error {
	data, err := readJSONObject(path)
	if err != nil {
		return err
	}
	if err := backupFile(path, path+".omniproxy.bak", []byte("{}\n")); err != nil {
		return err
	}

	env := cleanClaudeEnv(data)
	env["ANTHROPIC_BASE_URL"] = baseURL
	env["ANTHROPIC_AUTH_TOKEN"] = omniProxyMimoAuth
	env["ANTHROPIC_MODEL"] = mimoLongContextModel
	env["ANTHROPIC_DEFAULT_OPUS_MODEL"] = mimoLongContextModel
	env["ANTHROPIC_DEFAULT_OPUS_MODEL_NAME"] = "MiMo-V2.5-Pro [1m]"
	env["ANTHROPIC_DEFAULT_OPUS_MODEL_DESCRIPTION"] = "Xiaomi MiMo-V2.5-Pro 1M context routed through OmniProxy"
	env["ANTHROPIC_DEFAULT_SONNET_MODEL"] = mimoStandardModel
	env["ANTHROPIC_DEFAULT_SONNET_MODEL_NAME"] = "MiMo-V2.5"
	env["ANTHROPIC_DEFAULT_SONNET_MODEL_DESCRIPTION"] = "Xiaomi MiMo-V2.5 routed through OmniProxy"
	env["ANTHROPIC_DEFAULT_HAIKU_MODEL"] = mimoStandardModel
	env["ANTHROPIC_DEFAULT_HAIKU_MODEL_NAME"] = "MiMo-V2.5"
	env["ANTHROPIC_DEFAULT_HAIKU_MODEL_DESCRIPTION"] = "Xiaomi MiMo-V2.5 routed through OmniProxy"
	env["CLAUDE_CODE_SUBAGENT_MODEL"] = mimoStandardModel
	env["ANTHROPIC_CUSTOM_MODEL_OPTION"] = mimoModel
	env["ANTHROPIC_CUSTOM_MODEL_OPTION_NAME"] = "MiMo-V2.5-Pro"
	env["ANTHROPIC_CUSTOM_MODEL_OPTION_DESCRIPTION"] = "Xiaomi MiMo-V2.5-Pro routed through OmniProxy"
	data["env"] = env
	return writeJSONObject(path, data)
}

func writeDeepSeekClaudeSettings(path string, baseURL string) error {
	data, err := readJSONObject(path)
	if err != nil {
		return err
	}
	if err := backupFile(path, path+".omniproxy.bak", []byte("{}\n")); err != nil {
		return err
	}

	env := cleanClaudeEnv(data)
	env["ANTHROPIC_BASE_URL"] = baseURL
	env["ANTHROPIC_AUTH_TOKEN"] = omniProxyMimoAuth
	env["ANTHROPIC_MODEL"] = deepSeekProModel
	env["ANTHROPIC_DEFAULT_OPUS_MODEL"] = deepSeekProModel
	env["ANTHROPIC_DEFAULT_OPUS_MODEL_NAME"] = "DeepSeek V4 Pro"
	env["ANTHROPIC_DEFAULT_OPUS_MODEL_DESCRIPTION"] = "DeepSeek V4 Pro routed through OmniProxy"
	env["ANTHROPIC_DEFAULT_SONNET_MODEL"] = deepSeekProModel
	env["ANTHROPIC_DEFAULT_SONNET_MODEL_NAME"] = "DeepSeek V4 Pro"
	env["ANTHROPIC_DEFAULT_SONNET_MODEL_DESCRIPTION"] = "DeepSeek V4 Pro routed through OmniProxy"
	env["ANTHROPIC_DEFAULT_HAIKU_MODEL"] = deepSeekFastModel
	env["ANTHROPIC_DEFAULT_HAIKU_MODEL_NAME"] = "DeepSeek V4 Flash"
	env["ANTHROPIC_DEFAULT_HAIKU_MODEL_DESCRIPTION"] = "DeepSeek V4 Flash routed through OmniProxy"
	env["CLAUDE_CODE_SUBAGENT_MODEL"] = deepSeekFastModel
	env["CLAUDE_CODE_EFFORT_LEVEL"] = "max"
	data["env"] = env
	return writeJSONObject(path, data)
}

func writeKimiClaudeSettings(path string, baseURL string) error {
	return writeClaudeSingleModelSettings(path, baseURL, claudeKimiTarget)
}

func writeZhipuClaudeSettings(path string, baseURL string) error {
	return writeClaudeSingleModelSettings(path, baseURL, claudeZhipuTarget)
}

func writeSelectedClaudeSettings(path string, baseURL string, targets []claudeModelTarget) error {
	if len(targets) == 0 {
		return &claudeModelSelectionError{message: "至少选择一个 Claude Code 模型"}
	}

	data, err := readJSONObject(path)
	if err != nil {
		return err
	}
	if err := backupFile(path, path+".omniproxy.bak", []byte("{}\n")); err != nil {
		return err
	}

	env := cleanClaudeEnv(data)
	env["ANTHROPIC_BASE_URL"] = baseURL
	env["ANTHROPIC_AUTH_TOKEN"] = omniProxyMimoAuth
	env["ANTHROPIC_MODEL"] = targets[0].Model
	setClaudeModelGroup(env, "ANTHROPIC_DEFAULT_OPUS_MODEL", claudeTargetAt(targets, 0))
	setClaudeModelGroup(env, "ANTHROPIC_DEFAULT_SONNET_MODEL", claudeTargetAt(targets, 1))
	setClaudeModelGroup(env, "ANTHROPIC_DEFAULT_HAIKU_MODEL", claudeTargetAt(targets, 2))
	env["CLAUDE_CODE_SUBAGENT_MODEL"] = claudeTargetAt(targets, 1).Model
	if len(targets) == maxClaudeModels {
		setClaudeModelGroup(env, "ANTHROPIC_CUSTOM_MODEL_OPTION", targets[maxClaudeModels-1])
	}
	if claudeTargetsInclude(targets, deepSeekProModel) {
		env["CLAUDE_CODE_EFFORT_LEVEL"] = "max"
	}
	data["env"] = env
	return writeJSONObject(path, data)
}

func writeClaudeSingleModelSettings(path string, baseURL string, target claudeModelTarget) error {
	data, err := readJSONObject(path)
	if err != nil {
		return err
	}
	if err := backupFile(path, path+".omniproxy.bak", []byte("{}\n")); err != nil {
		return err
	}

	env, _ := data["env"].(map[string]any)
	if env == nil {
		env = map[string]any{}
	}
	removeClaudeRouterSettings(data, env)

	env["ANTHROPIC_BASE_URL"] = baseURL
	env["ANTHROPIC_AUTH_TOKEN"] = omniProxyMimoAuth
	env["ANTHROPIC_MODEL"] = target.Model
	env["ANTHROPIC_DEFAULT_OPUS_MODEL"] = target.Model
	env["ANTHROPIC_DEFAULT_OPUS_MODEL_NAME"] = target.Name
	env["ANTHROPIC_DEFAULT_OPUS_MODEL_DESCRIPTION"] = target.Description
	env["ANTHROPIC_DEFAULT_SONNET_MODEL"] = target.Model
	env["ANTHROPIC_DEFAULT_SONNET_MODEL_NAME"] = target.Name
	env["ANTHROPIC_DEFAULT_SONNET_MODEL_DESCRIPTION"] = target.Description
	env["ANTHROPIC_DEFAULT_HAIKU_MODEL"] = target.Model
	env["ANTHROPIC_DEFAULT_HAIKU_MODEL_NAME"] = target.Name
	env["ANTHROPIC_DEFAULT_HAIKU_MODEL_DESCRIPTION"] = target.Description
	env["CLAUDE_CODE_SUBAGENT_MODEL"] = target.Model
	env["ANTHROPIC_CUSTOM_MODEL_OPTION"] = target.Model
	env["ANTHROPIC_CUSTOM_MODEL_OPTION_NAME"] = target.Name
	env["ANTHROPIC_CUSTOM_MODEL_OPTION_DESCRIPTION"] = target.Description
	data["env"] = env
	return writeJSONObject(path, data)
}

func setClaudeModelGroup(env map[string]any, key string, target claudeModelTarget) {
	env[key] = target.Model
	env[key+"_NAME"] = target.Name
	env[key+"_DESCRIPTION"] = target.Description
}

func claudeTargetAt(targets []claudeModelTarget, index int) claudeModelTarget {
	if index < len(targets) {
		return targets[index]
	}
	return targets[len(targets)-1]
}

func claudeTargetsInclude(targets []claudeModelTarget, model string) bool {
	for _, target := range targets {
		if strings.EqualFold(target.Model, model) {
			return true
		}
	}
	return false
}

func normalizeClaudeModelTargets(models []string) ([]claudeModelTarget, error) {
	if len(models) == 0 {
		return nil, &claudeModelSelectionError{message: "至少选择一个 Claude Code 模型"}
	}

	targetsByModel := claudeSelectableTargetsByModel()
	seen := map[string]bool{}
	targets := make([]claudeModelTarget, 0, len(models))
	for _, raw := range models {
		model := normalizeClaudeModelID(raw)
		if model == "" {
			continue
		}
		target, ok := targetsByModel[model]
		if !ok {
			return nil, &claudeModelSelectionError{message: fmt.Sprintf("不支持的 Claude Code 模型：%s", strings.TrimSpace(raw))}
		}
		if seen[target.Model] {
			continue
		}
		targets = append(targets, target)
		seen[target.Model] = true
		if len(targets) > maxClaudeModels {
			return nil, &claudeModelSelectionError{message: fmt.Sprintf("Claude Code 最多选择 %d 个模型", maxClaudeModels)}
		}
	}
	if len(targets) == 0 {
		return nil, &claudeModelSelectionError{message: "至少选择一个 Claude Code 模型"}
	}
	return targets, nil
}

func normalizeClaudeModelID(model string) string {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "deepseek-v4-pro", "deepseek-4-pro":
		return deepSeekProModel
	case "mimo-v2.5-pro-1m", "mimo-2.5-pro-1m":
		return mimoLongContextModel
	case "mimo-2.5-pro":
		return mimoModel
	case "mimo-2.5":
		return mimoStandardModel
	default:
		return strings.ToLower(strings.TrimSpace(model))
	}
}

func claudeSelectableTargetsByModel() map[string]claudeModelTarget {
	targets := map[string]claudeModelTarget{}
	for _, target := range claudeSelectableTargets() {
		targets[strings.ToLower(target.Model)] = target
	}
	return targets
}

func claudeSelectableTargets() []claudeModelTarget {
	return []claudeModelTarget{
		claudeDeepSeekTarget,
		{
			Model:       deepSeekFastModel,
			Name:        "DeepSeek V4 Flash",
			Description: "DeepSeek V4 Flash routed through OmniProxy",
		},
		claudeMimoTarget,
		{
			Model:       mimoModel,
			Name:        "MiMo-V2.5-Pro",
			Description: "Xiaomi MiMo-V2.5-Pro routed through OmniProxy",
		},
		{
			Model:       mimoStandardModel,
			Name:        "MiMo-V2.5",
			Description: "Xiaomi MiMo-V2.5 routed through OmniProxy",
		},
		claudeKimiTarget,
		claudeZhipuTarget,
	}
}

func claudeModelIDs(targets []claudeModelTarget) []string {
	models := make([]string, 0, len(targets))
	for _, target := range targets {
		models = append(models, target.Model)
	}
	return models
}

func claudeModelNames(targets []claudeModelTarget) []string {
	names := make([]string, 0, len(targets))
	for _, target := range targets {
		names = append(names, target.Name)
	}
	return names
}

func claudeDesktopRoutesForTargets(targets []claudeModelTarget) []claudedesktop.ModelRoute {
	routeIDs := []string{
		"claude-sonnet-4-6",
		"claude-opus-4-7",
		"claude-haiku-4-5",
		"claude-sonnet-4-6-r2",
	}
	routes := make([]claudedesktop.ModelRoute, 0, len(targets))
	for index, target := range targets {
		if index >= len(routeIDs) {
			break
		}
		label := strings.TrimSpace(target.Name)
		if label == "" {
			label = target.Model
		}
		routes = append(routes, claudedesktop.ModelRoute{
			RouteID:       routeIDs[index],
			UpstreamModel: target.Model,
			LabelOverride: label,
			Supports1M:    strings.Contains(strings.ToLower(target.Model), "[1m]"),
		})
	}
	return routes
}

func writeClaudeDesktopProfile(paths claudedesktop.Paths, baseURL string, routes []claudedesktop.ModelRoute) error {
	if err := writeClaudeDesktopDeploymentMode(paths.NormalConfigPath, "3p"); err != nil {
		return err
	}
	if err := writeClaudeDesktopDeploymentMode(paths.ThreePConfigPath, "3p"); err != nil {
		return err
	}
	if err := writeJSONObject(paths.ProfilePath, claudedesktop.BuildGatewayProfile(baseURL, routes)); err != nil {
		return err
	}
	if err := claudedesktop.WriteRoutes(paths.RoutesPath, routes); err != nil {
		return err
	}
	return writeClaudeDesktopMeta(paths.MetaPath, true)
}

func writeClaudeDesktopDeploymentMode(path string, mode string) error {
	data, err := readJSONObject(path)
	if err != nil {
		return err
	}
	data["deploymentMode"] = mode
	return writeJSONObject(path, data)
}

func writeClaudeDesktopMeta(path string, apply bool) error {
	data, err := readJSONObject(path)
	if err != nil {
		return err
	}
	entries := make([]any, 0)
	if existing, ok := data["entries"].([]any); ok {
		for _, entry := range existing {
			item, ok := entry.(map[string]any)
			if ok && item["id"] == claudedesktop.ProfileID {
				continue
			}
			entries = append(entries, entry)
		}
	}
	if apply {
		entries = append(entries, map[string]any{
			"id":   claudedesktop.ProfileID,
			"name": claudedesktop.ProfileName,
		})
		data["appliedId"] = claudedesktop.ProfileID
	} else if data["appliedId"] == claudedesktop.ProfileID {
		delete(data, "appliedId")
	}
	data["entries"] = entries
	return writeJSONObject(path, data)
}

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
		deepSeekFastModel,
		kimiCodingModel,
		zhipuGLMModel,
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
		strings.EqualFold(model, deepSeekFastModel) ||
		strings.EqualFold(model, kimiCodingModel) ||
		strings.EqualFold(model, zhipuGLMModel)
}

func writeMimoClaudeOnboarding(path string) error {
	data, err := readJSONObject(path)
	if err != nil {
		return err
	}
	if err := backupFile(path, path+".omniproxy.bak", []byte("{}\n")); err != nil {
		return err
	}
	data["hasCompletedOnboarding"] = true
	return writeJSONObject(path, data)
}

func readJSONObject(path string) (map[string]any, error) {
	data := map[string]any{}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return data, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return data, nil
	}
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func writeJSONObject(path string, data map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o600)
}

func backupFile(path string, backupPath string, missingContent []byte) error {
	if _, err := os.Stat(backupPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		raw = missingContent
	}
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(backupPath, raw, 0o600)
}

func restoreBackup(path string, backupPath string) error {
	raw, err := os.ReadFile(backupPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}
