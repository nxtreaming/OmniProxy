package config

import (
	"encoding/json"
	"errors"
	"os"
	"strings"

	"OmniProxyBackend/internal/storage"
)

const (
	SchedulingModeQueue    = "queue"
	SchedulingModeBalanced = "balanced"

	WebSocketModeEnabled  = "enabled"
	WebSocketModeDisabled = "disabled"

	MimoCredentialPriorityAPIKey    = "api_key"
	MimoCredentialPriorityTokenPlan = "mimo_token_plan"
)

type Config struct {
	ProxyPort                       int    `json:"proxyPort"`
	ControlPort                     int    `json:"controlPort"`
	SchedulingMode                  string `json:"schedulingMode"`
	WebSocketMode                   string `json:"websocketMode"`
	UpstreamBaseURL                 string `json:"upstreamBaseUrl"`
	OpenAIBaseURL                   string `json:"openaiBaseUrl"`
	AnthropicBaseURL                string `json:"anthropicBaseUrl"`
	DeepSeekBaseURL                 string `json:"deepseekBaseUrl"`
	DeepSeekAnthropicBaseURL        string `json:"deepseekAnthropicBaseUrl"`
	KimiBaseURL                     string `json:"kimiBaseUrl"`
	ZhipuBaseURL                    string `json:"zhipuBaseUrl"`
	ZhipuAnthropicBaseURL           string `json:"zhipuAnthropicBaseUrl"`
	MiniMaxBaseURL                  string `json:"minimaxBaseUrl"`
	MiniMaxAnthropicBaseURL         string `json:"minimaxAnthropicBaseUrl"`
	GeminiBaseURL                   string `json:"geminiBaseUrl"`
	CustomGatewayBaseURL            string `json:"customGatewayBaseUrl"`
	CustomGatewayAnthropicBaseURL   string `json:"customGatewayAnthropicBaseUrl"`
	XiaomiBaseURL                   string `json:"xiaomiBaseUrl"`
	XiaomiAPIBaseURL                string `json:"xiaomiApiBaseUrl"`
	XiaomiAPIAnthropicBaseURL       string `json:"xiaomiApiAnthropicBaseUrl"`
	XiaomiTokenPlanBaseURL          string `json:"xiaomiTokenPlanBaseUrl"`
	XiaomiTokenPlanAnthropicBaseURL string `json:"xiaomiTokenPlanAnthropicBaseUrl"`
	XiaomiPlatformCookie            string `json:"xiaomiPlatformCookie,omitempty"`
	XiaomiCredentialPriority        string `json:"xiaomiCredentialPriority"`
	CodexBaseURL                    string `json:"codexBaseUrl"`
	SwitchThreshold                 int    `json:"switchThreshold"`
	MaxRetries                      int    `json:"maxRetries"`
	CodexUsageEndpoint              string `json:"codexUsageEndpoint"`
}

func Default() Config {
	return Config{
		ProxyPort:                       DefaultProxyPort(),
		ControlPort:                     DefaultControlPort(),
		SchedulingMode:                  SchedulingModeQueue,
		WebSocketMode:                   WebSocketModeEnabled,
		UpstreamBaseURL:                 "https://api.openai.com",
		OpenAIBaseURL:                   "https://api.openai.com",
		AnthropicBaseURL:                "https://api.anthropic.com",
		DeepSeekBaseURL:                 "https://api.deepseek.com",
		DeepSeekAnthropicBaseURL:        "https://api.deepseek.com/anthropic",
		KimiBaseURL:                     "https://api.kimi.com/coding",
		ZhipuBaseURL:                    "https://open.bigmodel.cn/api/paas/v4",
		ZhipuAnthropicBaseURL:           "https://open.bigmodel.cn/api/anthropic",
		MiniMaxBaseURL:                  "https://api.minimaxi.com/v1",
		MiniMaxAnthropicBaseURL:         "https://api.minimaxi.com/anthropic",
		GeminiBaseURL:                   "https://generativelanguage.googleapis.com",
		CustomGatewayBaseURL:            "",
		CustomGatewayAnthropicBaseURL:   "",
		XiaomiBaseURL:                   "",
		XiaomiAPIBaseURL:                "https://api.xiaomimimo.com/v1",
		XiaomiAPIAnthropicBaseURL:       "https://api.xiaomimimo.com/anthropic",
		XiaomiTokenPlanBaseURL:          "https://token-plan-cn.xiaomimimo.com/v1",
		XiaomiTokenPlanAnthropicBaseURL: "https://token-plan-cn.xiaomimimo.com/anthropic",
		XiaomiPlatformCookie:            "",
		XiaomiCredentialPriority:        MimoCredentialPriorityTokenPlan,
		CodexBaseURL:                    "https://chatgpt.com/backend-api/codex",
		SwitchThreshold:                 15,
		MaxRetries:                      2,
		CodexUsageEndpoint:              "https://chatgpt.com/backend-api/wham/usage",
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
		ProxyPort                       *int    `json:"proxyPort"`
		ControlPort                     *int    `json:"controlPort"`
		SchedulingMode                  *string `json:"schedulingMode"`
		WebSocketMode                   *string `json:"websocketMode"`
		UpstreamBaseURL                 *string `json:"upstreamBaseUrl"`
		OpenAIBaseURL                   *string `json:"openaiBaseUrl"`
		AnthropicBaseURL                *string `json:"anthropicBaseUrl"`
		DeepSeekBaseURL                 *string `json:"deepseekBaseUrl"`
		DeepSeekAnthropicBaseURL        *string `json:"deepseekAnthropicBaseUrl"`
		KimiBaseURL                     *string `json:"kimiBaseUrl"`
		ZhipuBaseURL                    *string `json:"zhipuBaseUrl"`
		ZhipuAnthropicBaseURL           *string `json:"zhipuAnthropicBaseUrl"`
		MiniMaxBaseURL                  *string `json:"minimaxBaseUrl"`
		MiniMaxAnthropicBaseURL         *string `json:"minimaxAnthropicBaseUrl"`
		GeminiBaseURL                   *string `json:"geminiBaseUrl"`
		CustomGatewayBaseURL            *string `json:"customGatewayBaseUrl"`
		CustomGatewayAnthropicBaseURL   *string `json:"customGatewayAnthropicBaseUrl"`
		XiaomiBaseURL                   *string `json:"xiaomiBaseUrl"`
		XiaomiAPIBaseURL                *string `json:"xiaomiApiBaseUrl"`
		XiaomiAPIAnthropicBaseURL       *string `json:"xiaomiApiAnthropicBaseUrl"`
		XiaomiTokenPlanBaseURL          *string `json:"xiaomiTokenPlanBaseUrl"`
		XiaomiTokenPlanAnthropicBaseURL *string `json:"xiaomiTokenPlanAnthropicBaseUrl"`
		XiaomiPlatformCookie            *string `json:"xiaomiPlatformCookie"`
		XiaomiCredentialPriority        *string `json:"xiaomiCredentialPriority"`
		CodexBaseURL                    *string `json:"codexBaseUrl"`
		SwitchThreshold                 *int    `json:"switchThreshold"`
		MaxRetries                      *int    `json:"maxRetries"`
		CodexUsageEndpoint              *string `json:"codexUsageEndpoint"`
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
	if saved.XiaomiPlatformCookie != nil {
		cfg.XiaomiPlatformCookie = *saved.XiaomiPlatformCookie
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
	cfg.XiaomiPlatformCookie = strings.TrimSpace(cfg.XiaomiPlatformCookie)
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
	if cfg.CodexUsageEndpoint == "" {
		cfg.CodexUsageEndpoint = defaults.CodexUsageEndpoint
	}
	return cfg
}
