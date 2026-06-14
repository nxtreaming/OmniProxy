package config

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"

	"OmniProxyBackend/internal/storage"
)

const (
	SchedulingModeQueue    = "queue"
	SchedulingModeBalanced = "balanced"

	WebSocketModeEnabled  = "enabled"
	WebSocketModeDisabled = "disabled"

	TaskAutomationLaunchModeMedia   = "media"
	TaskAutomationLaunchModeLinuxDO = "linuxdo"

	TaskAutomationBrowserDefault = "default"
	TaskAutomationBrowserEdge    = "edge"
	TaskAutomationBrowserChrome  = "chrome"
	TaskAutomationBrowserFirefox = "firefox"

	MimoCredentialPriorityAPIKey    = "api_key"
	MimoCredentialPriorityTokenPlan = "mimo_token_plan"
)

var defaultOutboundProxyModels = []string{
	"gpt-*",
	"claude-*",
	"gemini-*",
	"*/*",
}

var defaultOutboundProxyProviders = []string{
	"openai",
	"anthropic",
	"gemini",
	"openrouter",
	"zo",
}

type Config struct {
	ProxyPort                          int      `json:"proxyPort"`
	ControlPort                        int      `json:"controlPort"`
	SchedulingMode                     string   `json:"schedulingMode"`
	WebSocketMode                      string   `json:"websocketMode"`
	CheckBetaUpdates                   bool     `json:"checkBetaUpdates"`
	TaskAutomationEnabled              bool     `json:"taskAutomationEnabled"`
	TaskAutomationClients              []string `json:"taskAutomationClients"`
	TaskAutomationLaunchMode           string   `json:"taskAutomationLaunchMode"`
	TaskAutomationLaunchTarget         string   `json:"taskAutomationLaunchTarget"`
	TaskAutomationFallbackURL          string   `json:"taskAutomationFallbackUrl"`
	TaskAutomationBrowser              string   `json:"taskAutomationBrowser"`
	TaskAutomationBrowserUserDataDir   string   `json:"taskAutomationBrowserUserDataDir"`
	TaskAutomationBrowserProfile       string   `json:"taskAutomationBrowserProfile"`
	TaskAutomationReturnToClient       bool     `json:"taskAutomationReturnToClient"`
	TaskAutomationIdleSeconds          int      `json:"taskAutomationIdleSeconds"`
	TaskAutomationReturnDelaySeconds   int      `json:"taskAutomationReturnDelaySeconds"`
	OutboundProxyEnabled               bool     `json:"outboundProxyEnabled"`
	OutboundProxyURL                   string   `json:"outboundProxyUrl"`
	OutboundProxyProviders             []string `json:"outboundProxyProviders"`
	OutboundProxyModels                []string `json:"outboundProxyModels"`
	UpstreamBaseURL                    string   `json:"upstreamBaseUrl"`
	OpenAIBaseURL                      string   `json:"openaiBaseUrl"`
	AnthropicBaseURL                   string   `json:"anthropicBaseUrl"`
	DeepSeekBaseURL                    string   `json:"deepseekBaseUrl"`
	DeepSeekAnthropicBaseURL           string   `json:"deepseekAnthropicBaseUrl"`
	KimiBaseURL                        string   `json:"kimiBaseUrl"`
	ZhipuBaseURL                       string   `json:"zhipuBaseUrl"`
	ZhipuAnthropicBaseURL              string   `json:"zhipuAnthropicBaseUrl"`
	MiniMaxBaseURL                     string   `json:"minimaxBaseUrl"`
	MiniMaxAnthropicBaseURL            string   `json:"minimaxAnthropicBaseUrl"`
	GeminiBaseURL                      string   `json:"geminiBaseUrl"`
	OpenRouterBaseURL                  string   `json:"openrouterBaseUrl"`
	TokenRouterBaseURL                 string   `json:"tokenrouterBaseUrl"`
	Sub2APIBaseURL                     string   `json:"sub2apiBaseUrl"`
	NewAPIBaseURL                      string   `json:"newapiBaseUrl"`
	AnyRouterBaseURL                   string   `json:"anyrouterBaseUrl"`
	ZoBaseURL                          string   `json:"zoBaseUrl"`
	CustomGatewayBaseURL               string   `json:"customGatewayBaseUrl"`
	CustomGatewayAnthropicBaseURL      string   `json:"customGatewayAnthropicBaseUrl"`
	XiaomiBaseURL                      string   `json:"xiaomiBaseUrl"`
	XiaomiAPIBaseURL                   string   `json:"xiaomiApiBaseUrl"`
	XiaomiAPIAnthropicBaseURL          string   `json:"xiaomiApiAnthropicBaseUrl"`
	XiaomiTokenPlanBaseURL             string   `json:"xiaomiTokenPlanBaseUrl"`
	XiaomiTokenPlanAnthropicBaseURL    string   `json:"xiaomiTokenPlanAnthropicBaseUrl"`
	XiaomiTokenPlanSGPBaseURL          string   `json:"xiaomiTokenPlanSgpBaseUrl"`
	XiaomiTokenPlanSGPAnthropicBaseURL string   `json:"xiaomiTokenPlanSgpAnthropicBaseUrl"`
	XiaomiTokenPlanAMSBaseURL          string   `json:"xiaomiTokenPlanAmsBaseUrl"`
	XiaomiTokenPlanAMSAnthropicBaseURL string   `json:"xiaomiTokenPlanAmsAnthropicBaseUrl"`
	XiaomiCredentialPriority           string   `json:"xiaomiCredentialPriority"`
	CodexBaseURL                       string   `json:"codexBaseUrl"`
	SwitchThreshold                    int      `json:"switchThreshold"`
	MaxRetries                         int      `json:"maxRetries"`
	HistoryRetentionDays               int      `json:"historyRetentionDays"`
	CodexUsageEndpoint                 string   `json:"codexUsageEndpoint"`
}

func Default() Config {
	return Config{
		ProxyPort:                          DefaultProxyPort(),
		ControlPort:                        DefaultControlPort(),
		SchedulingMode:                     SchedulingModeQueue,
		WebSocketMode:                      WebSocketModeEnabled,
		CheckBetaUpdates:                   false,
		TaskAutomationEnabled:              false,
		TaskAutomationClients:              []string{"codex", "claude", "claude-desktop"},
		TaskAutomationLaunchMode:           TaskAutomationLaunchModeMedia,
		TaskAutomationLaunchTarget:         "",
		TaskAutomationFallbackURL:          "https://www.douyin.com",
		TaskAutomationBrowser:              TaskAutomationBrowserDefault,
		TaskAutomationBrowserUserDataDir:   "",
		TaskAutomationBrowserProfile:       "",
		TaskAutomationReturnToClient:       true,
		TaskAutomationIdleSeconds:          5,
		TaskAutomationReturnDelaySeconds:   3,
		OutboundProxyEnabled:               false,
		OutboundProxyURL:                   "http://127.0.0.1:10808",
		OutboundProxyProviders:             append([]string(nil), defaultOutboundProxyProviders...),
		OutboundProxyModels:                append([]string(nil), defaultOutboundProxyModels...),
		UpstreamBaseURL:                    "https://api.openai.com",
		OpenAIBaseURL:                      "https://api.openai.com",
		AnthropicBaseURL:                   "https://api.anthropic.com",
		DeepSeekBaseURL:                    "https://api.deepseek.com",
		DeepSeekAnthropicBaseURL:           "https://api.deepseek.com/anthropic",
		KimiBaseURL:                        "https://api.kimi.com/coding",
		ZhipuBaseURL:                       "https://open.bigmodel.cn/api/paas/v4",
		ZhipuAnthropicBaseURL:              "https://open.bigmodel.cn/api/anthropic",
		MiniMaxBaseURL:                     "https://api.minimaxi.com/v1",
		MiniMaxAnthropicBaseURL:            "https://api.minimaxi.com/anthropic",
		GeminiBaseURL:                      "https://generativelanguage.googleapis.com",
		OpenRouterBaseURL:                  "https://openrouter.ai/api/v1",
		TokenRouterBaseURL:                 "https://api.tokenrouter.io",
		Sub2APIBaseURL:                     "https://aiapi.aicnio.com",
		NewAPIBaseURL:                      "http://127.0.0.1:3000",
		AnyRouterBaseURL:                   "https://anyrouter.top",
		ZoBaseURL:                          "https://api.zo.computer",
		CustomGatewayBaseURL:               "",
		CustomGatewayAnthropicBaseURL:      "",
		XiaomiBaseURL:                      "",
		XiaomiAPIBaseURL:                   "https://api.xiaomimimo.com/v1",
		XiaomiAPIAnthropicBaseURL:          "https://api.xiaomimimo.com/anthropic",
		XiaomiTokenPlanBaseURL:             "https://token-plan-cn.xiaomimimo.com/v1",
		XiaomiTokenPlanAnthropicBaseURL:    "https://token-plan-cn.xiaomimimo.com/anthropic",
		XiaomiTokenPlanSGPBaseURL:          "https://token-plan-sgp.xiaomimimo.com/v1",
		XiaomiTokenPlanSGPAnthropicBaseURL: "https://token-plan-sgp.xiaomimimo.com/anthropic",
		XiaomiTokenPlanAMSBaseURL:          "https://token-plan-ams.xiaomimimo.com/v1",
		XiaomiTokenPlanAMSAnthropicBaseURL: "https://token-plan-ams.xiaomimimo.com/anthropic",
		XiaomiCredentialPriority:           MimoCredentialPriorityTokenPlan,
		CodexBaseURL:                       "https://chatgpt.com/backend-api/codex",
		SwitchThreshold:                    15,
		MaxRetries:                         2,
		HistoryRetentionDays:               14,
		CodexUsageEndpoint:                 "https://chatgpt.com/backend-api/wham/usage",
	}
}

type Store struct {
	path string
	file *storage.JSONStore[Config]
}

func NewStore(path string) *Store {
	return &Store{path: path, file: storage.NewJSONStore[Config](path)}
}

func (s *Store) Load() (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	if len(data) == 0 {
		return cfg, nil
	}

	var saved struct {
		ProxyPort                          *int      `json:"proxyPort"`
		ControlPort                        *int      `json:"controlPort"`
		SchedulingMode                     *string   `json:"schedulingMode"`
		WebSocketMode                      *string   `json:"websocketMode"`
		CheckBetaUpdates                   *bool     `json:"checkBetaUpdates"`
		TaskAutomationEnabled              *bool     `json:"taskAutomationEnabled"`
		TaskAutomationClients              *[]string `json:"taskAutomationClients"`
		TaskAutomationLaunchMode           *string   `json:"taskAutomationLaunchMode"`
		TaskAutomationLaunchTarget         *string   `json:"taskAutomationLaunchTarget"`
		TaskAutomationFallbackURL          *string   `json:"taskAutomationFallbackUrl"`
		TaskAutomationBrowser              *string   `json:"taskAutomationBrowser"`
		TaskAutomationBrowserUserDataDir   *string   `json:"taskAutomationBrowserUserDataDir"`
		TaskAutomationBrowserProfile       *string   `json:"taskAutomationBrowserProfile"`
		TaskAutomationReturnToClient       *bool     `json:"taskAutomationReturnToClient"`
		TaskAutomationIdleSeconds          *int      `json:"taskAutomationIdleSeconds"`
		TaskAutomationReturnDelaySeconds   *int      `json:"taskAutomationReturnDelaySeconds"`
		OutboundProxyEnabled               *bool     `json:"outboundProxyEnabled"`
		OutboundProxyURL                   *string   `json:"outboundProxyUrl"`
		OutboundProxyProviders             *[]string `json:"outboundProxyProviders"`
		OutboundProxyModels                *[]string `json:"outboundProxyModels"`
		UpstreamBaseURL                    *string   `json:"upstreamBaseUrl"`
		OpenAIBaseURL                      *string   `json:"openaiBaseUrl"`
		AnthropicBaseURL                   *string   `json:"anthropicBaseUrl"`
		DeepSeekBaseURL                    *string   `json:"deepseekBaseUrl"`
		DeepSeekAnthropicBaseURL           *string   `json:"deepseekAnthropicBaseUrl"`
		KimiBaseURL                        *string   `json:"kimiBaseUrl"`
		ZhipuBaseURL                       *string   `json:"zhipuBaseUrl"`
		ZhipuAnthropicBaseURL              *string   `json:"zhipuAnthropicBaseUrl"`
		MiniMaxBaseURL                     *string   `json:"minimaxBaseUrl"`
		MiniMaxAnthropicBaseURL            *string   `json:"minimaxAnthropicBaseUrl"`
		GeminiBaseURL                      *string   `json:"geminiBaseUrl"`
		OpenRouterBaseURL                  *string   `json:"openrouterBaseUrl"`
		TokenRouterBaseURL                 *string   `json:"tokenrouterBaseUrl"`
		Sub2APIBaseURL                     *string   `json:"sub2apiBaseUrl"`
		NewAPIBaseURL                      *string   `json:"newapiBaseUrl"`
		AnyRouterBaseURL                   *string   `json:"anyrouterBaseUrl"`
		ZoBaseURL                          *string   `json:"zoBaseUrl"`
		CustomGatewayBaseURL               *string   `json:"customGatewayBaseUrl"`
		CustomGatewayAnthropicBaseURL      *string   `json:"customGatewayAnthropicBaseUrl"`
		XiaomiBaseURL                      *string   `json:"xiaomiBaseUrl"`
		XiaomiAPIBaseURL                   *string   `json:"xiaomiApiBaseUrl"`
		XiaomiAPIAnthropicBaseURL          *string   `json:"xiaomiApiAnthropicBaseUrl"`
		XiaomiTokenPlanBaseURL             *string   `json:"xiaomiTokenPlanBaseUrl"`
		XiaomiTokenPlanAnthropicBaseURL    *string   `json:"xiaomiTokenPlanAnthropicBaseUrl"`
		XiaomiTokenPlanSGPBaseURL          *string   `json:"xiaomiTokenPlanSgpBaseUrl"`
		XiaomiTokenPlanSGPAnthropicBaseURL *string   `json:"xiaomiTokenPlanSgpAnthropicBaseUrl"`
		XiaomiTokenPlanAMSBaseURL          *string   `json:"xiaomiTokenPlanAmsBaseUrl"`
		XiaomiTokenPlanAMSAnthropicBaseURL *string   `json:"xiaomiTokenPlanAmsAnthropicBaseUrl"`
		XiaomiCredentialPriority           *string   `json:"xiaomiCredentialPriority"`
		CodexBaseURL                       *string   `json:"codexBaseUrl"`
		SwitchThreshold                    *int      `json:"switchThreshold"`
		MaxRetries                         *int      `json:"maxRetries"`
		HistoryRetentionDays               *int      `json:"historyRetentionDays"`
		CodexUsageEndpoint                 *string   `json:"codexUsageEndpoint"`
	}
	if err := json.Unmarshal(data, &saved); err != nil {
		return cfg, err
	}

	if saved.ProxyPort != nil && *saved.ProxyPort > 0 {
		cfg.ProxyPort = *saved.ProxyPort
	}
	if saved.ControlPort != nil && *saved.ControlPort > 0 {
		cfg.ControlPort = *saved.ControlPort
	}
	if saved.SchedulingMode != nil {
		cfg.SchedulingMode = *saved.SchedulingMode
	}
	if saved.WebSocketMode != nil {
		cfg.WebSocketMode = *saved.WebSocketMode
	}
	if saved.CheckBetaUpdates != nil {
		cfg.CheckBetaUpdates = *saved.CheckBetaUpdates
	}
	if saved.TaskAutomationEnabled != nil {
		cfg.TaskAutomationEnabled = *saved.TaskAutomationEnabled
	}
	if saved.TaskAutomationClients != nil {
		cfg.TaskAutomationClients = append([]string(nil), (*saved.TaskAutomationClients)...)
	}
	if saved.TaskAutomationLaunchMode != nil {
		cfg.TaskAutomationLaunchMode = *saved.TaskAutomationLaunchMode
	}
	if saved.TaskAutomationLaunchTarget != nil {
		cfg.TaskAutomationLaunchTarget = *saved.TaskAutomationLaunchTarget
	}
	if saved.TaskAutomationFallbackURL != nil {
		cfg.TaskAutomationFallbackURL = *saved.TaskAutomationFallbackURL
	}
	if saved.TaskAutomationBrowser != nil {
		cfg.TaskAutomationBrowser = *saved.TaskAutomationBrowser
	}
	if saved.TaskAutomationBrowserUserDataDir != nil {
		cfg.TaskAutomationBrowserUserDataDir = *saved.TaskAutomationBrowserUserDataDir
	}
	if saved.TaskAutomationBrowserProfile != nil {
		cfg.TaskAutomationBrowserProfile = *saved.TaskAutomationBrowserProfile
	}
	if saved.TaskAutomationReturnToClient != nil {
		cfg.TaskAutomationReturnToClient = *saved.TaskAutomationReturnToClient
	}
	if saved.TaskAutomationIdleSeconds != nil {
		cfg.TaskAutomationIdleSeconds = *saved.TaskAutomationIdleSeconds
	}
	if saved.TaskAutomationReturnDelaySeconds != nil {
		cfg.TaskAutomationReturnDelaySeconds = *saved.TaskAutomationReturnDelaySeconds
	}
	if saved.OutboundProxyEnabled != nil {
		cfg.OutboundProxyEnabled = *saved.OutboundProxyEnabled
	}
	if saved.OutboundProxyURL != nil {
		cfg.OutboundProxyURL = *saved.OutboundProxyURL
	}
	if saved.OutboundProxyProviders != nil {
		cfg.OutboundProxyProviders = append([]string(nil), (*saved.OutboundProxyProviders)...)
	} else if saved.OutboundProxyModels != nil {
		cfg.OutboundProxyProviders = providersFromOutboundProxyModels(*saved.OutboundProxyModels)
	}
	if saved.OutboundProxyModels != nil {
		cfg.OutboundProxyModels = append([]string(nil), (*saved.OutboundProxyModels)...)
	}
	if saved.UpstreamBaseURL != nil && *saved.UpstreamBaseURL != "" {
		cfg.UpstreamBaseURL = *saved.UpstreamBaseURL
	}
	if saved.OpenAIBaseURL != nil && *saved.OpenAIBaseURL != "" {
		cfg.OpenAIBaseURL = *saved.OpenAIBaseURL
	}
	if saved.AnthropicBaseURL != nil && *saved.AnthropicBaseURL != "" {
		cfg.AnthropicBaseURL = *saved.AnthropicBaseURL
	}
	if saved.DeepSeekBaseURL != nil && *saved.DeepSeekBaseURL != "" {
		cfg.DeepSeekBaseURL = *saved.DeepSeekBaseURL
	}
	if saved.DeepSeekAnthropicBaseURL != nil && *saved.DeepSeekAnthropicBaseURL != "" {
		cfg.DeepSeekAnthropicBaseURL = *saved.DeepSeekAnthropicBaseURL
	}
	if saved.KimiBaseURL != nil && *saved.KimiBaseURL != "" {
		cfg.KimiBaseURL = *saved.KimiBaseURL
	}
	if saved.ZhipuBaseURL != nil && *saved.ZhipuBaseURL != "" {
		cfg.ZhipuBaseURL = *saved.ZhipuBaseURL
	}
	if saved.ZhipuAnthropicBaseURL != nil && *saved.ZhipuAnthropicBaseURL != "" {
		cfg.ZhipuAnthropicBaseURL = *saved.ZhipuAnthropicBaseURL
	}
	if saved.MiniMaxBaseURL != nil && *saved.MiniMaxBaseURL != "" {
		cfg.MiniMaxBaseURL = *saved.MiniMaxBaseURL
	}
	if saved.MiniMaxAnthropicBaseURL != nil && *saved.MiniMaxAnthropicBaseURL != "" {
		cfg.MiniMaxAnthropicBaseURL = *saved.MiniMaxAnthropicBaseURL
	}
	if saved.GeminiBaseURL != nil && *saved.GeminiBaseURL != "" {
		cfg.GeminiBaseURL = *saved.GeminiBaseURL
	}
	if saved.OpenRouterBaseURL != nil && *saved.OpenRouterBaseURL != "" {
		cfg.OpenRouterBaseURL = *saved.OpenRouterBaseURL
	}
	if saved.TokenRouterBaseURL != nil && *saved.TokenRouterBaseURL != "" {
		cfg.TokenRouterBaseURL = *saved.TokenRouterBaseURL
	}
	if saved.Sub2APIBaseURL != nil && *saved.Sub2APIBaseURL != "" {
		cfg.Sub2APIBaseURL = *saved.Sub2APIBaseURL
	}
	if saved.NewAPIBaseURL != nil && *saved.NewAPIBaseURL != "" {
		cfg.NewAPIBaseURL = *saved.NewAPIBaseURL
	}
	if saved.AnyRouterBaseURL != nil && *saved.AnyRouterBaseURL != "" {
		cfg.AnyRouterBaseURL = *saved.AnyRouterBaseURL
	}
	if saved.ZoBaseURL != nil && *saved.ZoBaseURL != "" {
		cfg.ZoBaseURL = *saved.ZoBaseURL
	}
	if saved.CustomGatewayBaseURL != nil {
		cfg.CustomGatewayBaseURL = *saved.CustomGatewayBaseURL
	}
	if saved.CustomGatewayAnthropicBaseURL != nil {
		cfg.CustomGatewayAnthropicBaseURL = *saved.CustomGatewayAnthropicBaseURL
	}
	if saved.XiaomiBaseURL != nil {
		cfg.XiaomiBaseURL = *saved.XiaomiBaseURL
	}
	if saved.XiaomiAPIBaseURL != nil && *saved.XiaomiAPIBaseURL != "" {
		cfg.XiaomiAPIBaseURL = *saved.XiaomiAPIBaseURL
	} else if cfg.XiaomiBaseURL != "" {
		cfg.XiaomiAPIBaseURL = cfg.XiaomiBaseURL
	}
	if saved.XiaomiAPIAnthropicBaseURL != nil && *saved.XiaomiAPIAnthropicBaseURL != "" {
		cfg.XiaomiAPIAnthropicBaseURL = *saved.XiaomiAPIAnthropicBaseURL
	}
	if saved.XiaomiTokenPlanBaseURL != nil && *saved.XiaomiTokenPlanBaseURL != "" {
		cfg.XiaomiTokenPlanBaseURL = *saved.XiaomiTokenPlanBaseURL
	}
	if saved.XiaomiTokenPlanAnthropicBaseURL != nil && *saved.XiaomiTokenPlanAnthropicBaseURL != "" {
		cfg.XiaomiTokenPlanAnthropicBaseURL = *saved.XiaomiTokenPlanAnthropicBaseURL
	}
	if saved.XiaomiTokenPlanSGPBaseURL != nil && *saved.XiaomiTokenPlanSGPBaseURL != "" {
		cfg.XiaomiTokenPlanSGPBaseURL = *saved.XiaomiTokenPlanSGPBaseURL
	}
	if saved.XiaomiTokenPlanSGPAnthropicBaseURL != nil && *saved.XiaomiTokenPlanSGPAnthropicBaseURL != "" {
		cfg.XiaomiTokenPlanSGPAnthropicBaseURL = *saved.XiaomiTokenPlanSGPAnthropicBaseURL
	}
	if saved.XiaomiTokenPlanAMSBaseURL != nil && *saved.XiaomiTokenPlanAMSBaseURL != "" {
		cfg.XiaomiTokenPlanAMSBaseURL = *saved.XiaomiTokenPlanAMSBaseURL
	}
	if saved.XiaomiTokenPlanAMSAnthropicBaseURL != nil && *saved.XiaomiTokenPlanAMSAnthropicBaseURL != "" {
		cfg.XiaomiTokenPlanAMSAnthropicBaseURL = *saved.XiaomiTokenPlanAMSAnthropicBaseURL
	}
	if saved.XiaomiCredentialPriority != nil {
		cfg.XiaomiCredentialPriority = *saved.XiaomiCredentialPriority
	}
	if saved.CodexBaseURL != nil && *saved.CodexBaseURL != "" {
		cfg.CodexBaseURL = *saved.CodexBaseURL
	}
	if saved.SwitchThreshold != nil && *saved.SwitchThreshold > 0 {
		cfg.SwitchThreshold = *saved.SwitchThreshold
	}
	if saved.MaxRetries != nil && *saved.MaxRetries >= 0 {
		cfg.MaxRetries = *saved.MaxRetries
	}
	if saved.HistoryRetentionDays != nil && *saved.HistoryRetentionDays > 0 {
		cfg.HistoryRetentionDays = *saved.HistoryRetentionDays
	}
	if saved.CodexUsageEndpoint != nil && *saved.CodexUsageEndpoint != "" {
		cfg.CodexUsageEndpoint = *saved.CodexUsageEndpoint
	}
	return Normalize(cfg), nil
}

func (s *Store) Save(cfg Config) error {
	return s.file.Save(Normalize(cfg))
}

func Normalize(cfg Config) Config {
	defaults := Default()
	if cfg.ProxyPort <= 0 {
		cfg.ProxyPort = defaults.ProxyPort
	}
	if cfg.ControlPort <= 0 {
		cfg.ControlPort = defaults.ControlPort
	}
	switch strings.ToLower(strings.TrimSpace(cfg.SchedulingMode)) {
	case SchedulingModeBalanced:
		cfg.SchedulingMode = SchedulingModeBalanced
	case SchedulingModeQueue, "":
		cfg.SchedulingMode = SchedulingModeQueue
	default:
		cfg.SchedulingMode = defaults.SchedulingMode
	}
	switch strings.ToLower(strings.TrimSpace(cfg.WebSocketMode)) {
	case WebSocketModeDisabled:
		cfg.WebSocketMode = WebSocketModeDisabled
	case WebSocketModeEnabled, "":
		cfg.WebSocketMode = WebSocketModeEnabled
	default:
		cfg.WebSocketMode = defaults.WebSocketMode
	}
	if cfg.TaskAutomationClients == nil {
		cfg.TaskAutomationClients = append([]string(nil), defaults.TaskAutomationClients...)
	} else {
		cfg.TaskAutomationClients = normalizeTaskAutomationClients(cfg.TaskAutomationClients)
	}
	cfg.TaskAutomationLaunchMode = normalizeTaskAutomationLaunchMode(cfg.TaskAutomationLaunchMode)
	cfg.TaskAutomationLaunchTarget = strings.TrimSpace(cfg.TaskAutomationLaunchTarget)
	cfg.TaskAutomationFallbackURL = strings.TrimSpace(cfg.TaskAutomationFallbackURL)
	if cfg.TaskAutomationFallbackURL == "" {
		cfg.TaskAutomationFallbackURL = defaults.TaskAutomationFallbackURL
	}
	cfg.TaskAutomationBrowser = normalizeTaskAutomationBrowser(cfg.TaskAutomationBrowser)
	cfg.TaskAutomationBrowserUserDataDir = strings.TrimSpace(cfg.TaskAutomationBrowserUserDataDir)
	cfg.TaskAutomationBrowserProfile = strings.TrimSpace(cfg.TaskAutomationBrowserProfile)
	if cfg.TaskAutomationIdleSeconds <= 0 {
		cfg.TaskAutomationIdleSeconds = defaults.TaskAutomationIdleSeconds
	}
	if cfg.TaskAutomationIdleSeconds > 600 {
		cfg.TaskAutomationIdleSeconds = 600
	}
	if cfg.TaskAutomationReturnDelaySeconds <= 0 {
		cfg.TaskAutomationReturnDelaySeconds = defaults.TaskAutomationReturnDelaySeconds
	}
	if cfg.TaskAutomationReturnDelaySeconds > 600 {
		cfg.TaskAutomationReturnDelaySeconds = 600
	}
	cfg.OutboundProxyURL = normalizeOutboundProxyURL(cfg.OutboundProxyURL)
	if cfg.OutboundProxyURL == "" {
		cfg.OutboundProxyURL = defaults.OutboundProxyURL
	}
	if cfg.OutboundProxyProviders == nil {
		cfg.OutboundProxyProviders = append([]string(nil), defaults.OutboundProxyProviders...)
	} else {
		cfg.OutboundProxyProviders = normalizeOutboundProxyProviders(cfg.OutboundProxyProviders)
	}
	if cfg.OutboundProxyModels == nil {
		cfg.OutboundProxyModels = append([]string(nil), defaults.OutboundProxyModels...)
	} else {
		cfg.OutboundProxyModels = normalizeOutboundProxyModels(cfg.OutboundProxyModels)
	}
	if cfg.UpstreamBaseURL == "" {
		cfg.UpstreamBaseURL = defaults.UpstreamBaseURL
	}
	if cfg.OpenAIBaseURL == "" {
		cfg.OpenAIBaseURL = cfg.UpstreamBaseURL
	}
	if cfg.AnthropicBaseURL == "" {
		cfg.AnthropicBaseURL = defaults.AnthropicBaseURL
	}
	if cfg.DeepSeekBaseURL == "" {
		cfg.DeepSeekBaseURL = defaults.DeepSeekBaseURL
	}
	if cfg.DeepSeekAnthropicBaseURL == "" {
		cfg.DeepSeekAnthropicBaseURL = defaults.DeepSeekAnthropicBaseURL
	}
	if cfg.KimiBaseURL == "" {
		cfg.KimiBaseURL = defaults.KimiBaseURL
	}
	if cfg.ZhipuBaseURL == "" {
		cfg.ZhipuBaseURL = defaults.ZhipuBaseURL
	}
	if cfg.ZhipuAnthropicBaseURL == "" {
		cfg.ZhipuAnthropicBaseURL = defaults.ZhipuAnthropicBaseURL
	}
	if cfg.MiniMaxBaseURL == "" {
		cfg.MiniMaxBaseURL = defaults.MiniMaxBaseURL
	}
	if cfg.MiniMaxAnthropicBaseURL == "" {
		cfg.MiniMaxAnthropicBaseURL = defaults.MiniMaxAnthropicBaseURL
	}
	if cfg.GeminiBaseURL == "" {
		cfg.GeminiBaseURL = defaults.GeminiBaseURL
	}
	if cfg.OpenRouterBaseURL == "" {
		cfg.OpenRouterBaseURL = defaults.OpenRouterBaseURL
	}
	if cfg.TokenRouterBaseURL == "" {
		cfg.TokenRouterBaseURL = defaults.TokenRouterBaseURL
	}
	if cfg.Sub2APIBaseURL == "" {
		cfg.Sub2APIBaseURL = defaults.Sub2APIBaseURL
	}
	if cfg.NewAPIBaseURL == "" {
		cfg.NewAPIBaseURL = defaults.NewAPIBaseURL
	}
	if cfg.AnyRouterBaseURL == "" {
		cfg.AnyRouterBaseURL = defaults.AnyRouterBaseURL
	}
	if cfg.ZoBaseURL == "" {
		cfg.ZoBaseURL = defaults.ZoBaseURL
	}
	if cfg.CodexBaseURL == "" {
		cfg.CodexBaseURL = defaults.CodexBaseURL
	}
	if cfg.XiaomiAPIBaseURL == "" {
		if cfg.XiaomiBaseURL != "" {
			cfg.XiaomiAPIBaseURL = cfg.XiaomiBaseURL
		} else {
			cfg.XiaomiAPIBaseURL = defaults.XiaomiAPIBaseURL
		}
	}
	if cfg.XiaomiAPIAnthropicBaseURL == "" {
		cfg.XiaomiAPIAnthropicBaseURL = defaults.XiaomiAPIAnthropicBaseURL
	}
	if cfg.XiaomiTokenPlanBaseURL == "" {
		cfg.XiaomiTokenPlanBaseURL = defaults.XiaomiTokenPlanBaseURL
	}
	if cfg.XiaomiTokenPlanAnthropicBaseURL == "" {
		cfg.XiaomiTokenPlanAnthropicBaseURL = defaults.XiaomiTokenPlanAnthropicBaseURL
	}
	if cfg.XiaomiTokenPlanSGPBaseURL == "" {
		cfg.XiaomiTokenPlanSGPBaseURL = defaults.XiaomiTokenPlanSGPBaseURL
	}
	if cfg.XiaomiTokenPlanSGPAnthropicBaseURL == "" {
		cfg.XiaomiTokenPlanSGPAnthropicBaseURL = defaults.XiaomiTokenPlanSGPAnthropicBaseURL
	}
	if cfg.XiaomiTokenPlanAMSBaseURL == "" {
		cfg.XiaomiTokenPlanAMSBaseURL = defaults.XiaomiTokenPlanAMSBaseURL
	}
	if cfg.XiaomiTokenPlanAMSAnthropicBaseURL == "" {
		cfg.XiaomiTokenPlanAMSAnthropicBaseURL = defaults.XiaomiTokenPlanAMSAnthropicBaseURL
	}
	switch strings.ToLower(strings.TrimSpace(cfg.XiaomiCredentialPriority)) {
	case MimoCredentialPriorityAPIKey, "api":
		cfg.XiaomiCredentialPriority = MimoCredentialPriorityAPIKey
	case MimoCredentialPriorityTokenPlan, "tokenplan", "token_plan", "token-plan":
		cfg.XiaomiCredentialPriority = MimoCredentialPriorityTokenPlan
	case "":
		cfg.XiaomiCredentialPriority = defaults.XiaomiCredentialPriority
	default:
		cfg.XiaomiCredentialPriority = defaults.XiaomiCredentialPriority
	}
	if cfg.SwitchThreshold <= 0 {
		cfg.SwitchThreshold = defaults.SwitchThreshold
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = defaults.MaxRetries
	}
	if cfg.HistoryRetentionDays <= 0 {
		cfg.HistoryRetentionDays = defaults.HistoryRetentionDays
	}
	if cfg.HistoryRetentionDays > 365 {
		cfg.HistoryRetentionDays = 365
	}
	if cfg.CodexUsageEndpoint == "" {
		cfg.CodexUsageEndpoint = defaults.CodexUsageEndpoint
	}
	return cfg
}

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

func normalizeOutboundProxyModels(models []string) []string {
	if len(models) == 0 {
		return []string{}
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		key := strings.ToLower(model)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, model)
	}
	return out
}

func normalizeOutboundProxyProviders(providers []string) []string {
	if len(providers) == 0 {
		return []string{}
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(providers))
	for _, item := range providers {
		value := normalizeOutboundProxyProvider(item)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func normalizeOutboundProxyProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai", "codex":
		return "openai"
	case "anthropic", "claude":
		return "anthropic"
	case "deepseek":
		return "deepseek"
	case "kimi":
		return "kimi"
	case "xiaomi", "mimo":
		return "xiaomi"
	case "zhipu", "glm":
		return "zhipu"
	case "minimax":
		return "minimax"
	case "gemini", "google":
		return "gemini"
	case "openrouter":
		return "openrouter"
	case "tokenrouter":
		return "tokenrouter"
	case "sub2api":
		return "sub2api"
	case "newapi", "new-api", "new api":
		return "newapi"
	case "anyrouter", "any-router", "any router":
		return "anyrouter"
	case "zo", "zocomputer", "zo-computer":
		return "zo"
	case "custom":
		return "custom"
	default:
		return ""
	}
}

func providersFromOutboundProxyModels(models []string) []string {
	normalizedModels := normalizeOutboundProxyModels(models)
	if sameStringSet(normalizedModels, defaultOutboundProxyModels) {
		return append([]string(nil), defaultOutboundProxyProviders...)
	}
	providers := make([]string, 0)
	for _, raw := range normalizedModels {
		model := strings.ToLower(strings.TrimSpace(raw))
		switch {
		case model == "gpt-*" || strings.HasPrefix(model, "gpt-"):
			providers = append(providers, "openai")
		case model == "claude-*" || strings.HasPrefix(model, "claude-"):
			providers = append(providers, "anthropic")
		case model == "gemini-*" || strings.HasPrefix(model, "gemini-"):
			providers = append(providers, "gemini")
		case model == "*/*" || strings.Contains(model, "/"):
			providers = append(providers, "openrouter")
		case strings.HasPrefix(model, "deepseek-"):
			providers = append(providers, "deepseek")
		case strings.HasPrefix(model, "kimi-"):
			providers = append(providers, "kimi")
		case strings.HasPrefix(model, "glm-") || strings.HasPrefix(model, "zhipu-"):
			providers = append(providers, "zhipu")
		case strings.HasPrefix(model, "minimax-"):
			providers = append(providers, "minimax")
		case strings.HasPrefix(model, "mimo-"):
			providers = append(providers, "xiaomi")
		case strings.HasPrefix(model, "auto:") || strings.HasPrefix(model, "tokenrouter:") || strings.HasPrefix(model, "tokenrouter/"):
			providers = append(providers, "tokenrouter")
		case strings.HasPrefix(model, "custom-"):
			providers = append(providers, "custom")
		}
	}
	return normalizeOutboundProxyProviders(providers)
}

func sameStringSet(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	seen := map[string]bool{}
	for _, item := range left {
		seen[strings.ToLower(strings.TrimSpace(item))] = true
	}
	for _, item := range right {
		if !seen[strings.ToLower(strings.TrimSpace(item))] {
			return false
		}
	}
	return true
}

func isPort(value string) bool {
	if value == "" {
		return false
	}
	port, err := strconv.Atoi(value)
	return err == nil && port > 0 && port <= 65535
}
