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
	XiaomiBaseURL                   string `json:"xiaomiBaseUrl"`
	XiaomiAPIBaseURL                string `json:"xiaomiApiBaseUrl"`
	XiaomiAPIAnthropicBaseURL       string `json:"xiaomiApiAnthropicBaseUrl"`
	XiaomiTokenPlanBaseURL          string `json:"xiaomiTokenPlanBaseUrl"`
	XiaomiTokenPlanAnthropicBaseURL string `json:"xiaomiTokenPlanAnthropicBaseUrl"`
	CodexBaseURL                    string `json:"codexBaseUrl"`
	SwitchThreshold                 int    `json:"switchThreshold"`
	MaxRetries                      int    `json:"maxRetries"`
	CodexUsageEndpoint              string `json:"codexUsageEndpoint"`
}

func Default() Config {
	return Config{
		ProxyPort:                       3000,
		ControlPort:                     3890,
		SchedulingMode:                  SchedulingModeQueue,
		WebSocketMode:                   WebSocketModeEnabled,
		UpstreamBaseURL:                 "https://api.openai.com",
		OpenAIBaseURL:                   "https://api.openai.com",
		AnthropicBaseURL:                "https://api.anthropic.com",
		DeepSeekBaseURL:                 "https://api.deepseek.com",
		DeepSeekAnthropicBaseURL:        "https://api.deepseek.com/anthropic",
		KimiBaseURL:                     "https://api.kimi.com/coding",
		XiaomiBaseURL:                   "",
		XiaomiAPIBaseURL:                "https://api.xiaomimimo.com/v1",
		XiaomiAPIAnthropicBaseURL:       "https://api.xiaomimimo.com/anthropic",
		XiaomiTokenPlanBaseURL:          "https://token-plan-cn.xiaomimimo.com/v1",
		XiaomiTokenPlanAnthropicBaseURL: "https://token-plan-cn.xiaomimimo.com/anthropic",
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
		XiaomiBaseURL                   *string `json:"xiaomiBaseUrl"`
		XiaomiAPIBaseURL                *string `json:"xiaomiApiBaseUrl"`
		XiaomiAPIAnthropicBaseURL       *string `json:"xiaomiApiAnthropicBaseUrl"`
		XiaomiTokenPlanBaseURL          *string `json:"xiaomiTokenPlanBaseUrl"`
		XiaomiTokenPlanAnthropicBaseURL *string `json:"xiaomiTokenPlanAnthropicBaseUrl"`
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
