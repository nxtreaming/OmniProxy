package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"OmniProxyBackend/internal/config"
	"OmniProxyBackend/internal/token"
)

type ValidationResult struct {
	OK          bool             `json:"ok"`
	Status      int              `json:"status"`
	Duration    int64            `json:"durationMs"`
	Remaining   *int             `json:"remaining,omitempty"`
	Usage       *token.UsageInfo `json:"usage,omitempty"`
	Message     string           `json:"message"`
	CheckedPath string           `json:"checkedPath"`
}

type Validator struct {
	cfg    config.Config
	client *http.Client
}

func NewValidator(cfg config.Config) (*Validator, error) {
	cfg = config.Normalize(cfg)
	for name, baseURL := range map[string]string{
		"openai":                      cfg.OpenAIBaseURL,
		"anthropic":                   cfg.AnthropicBaseURL,
		"deepseek":                    cfg.DeepSeekBaseURL,
		"deepseek_anthropic":          cfg.DeepSeekAnthropicBaseURL,
		"kimi":                        cfg.KimiBaseURL,
		"xiaomi_api":                  cfg.XiaomiAPIBaseURL,
		"xiaomi_api_anthropic":        cfg.XiaomiAPIAnthropicBaseURL,
		"xiaomi_token_plan":           cfg.XiaomiTokenPlanBaseURL,
		"xiaomi_token_plan_anthropic": cfg.XiaomiTokenPlanAnthropicBaseURL,
		"codex":                       cfg.CodexBaseURL,
		"codex_usage":                 cfg.CodexUsageEndpoint,
	} {
		if strings.TrimSpace(baseURL) == "" {
			continue
		}
		if _, err := url.ParseRequestURI(baseURL); err != nil {
			return nil, fmt.Errorf("invalid %s url: %w", name, err)
		}
	}

	return &Validator{
		cfg:    cfg,
		client: &http.Client{Timeout: 12 * time.Second},
	}, nil
}

func (v *Validator) Validate(ctx context.Context, selected token.Token) (ValidationResult, error) {
	start := time.Now()
	target, err := v.validationURL(selected)
	if err != nil {
		return ValidationResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return ValidationResult{}, err
	}
	req.Header.Set("Accept", "application/json")
	if err := applyAuth(req.Header, selected); err != nil {
		return ValidationResult{}, err
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return ValidationResult{
			OK:          false,
			Duration:    time.Since(start).Milliseconds(),
			Message:     err.Error(),
			CheckedPath: target,
		}, err
	}
	defer closeBody(resp.Body)
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))

	result := ValidationResult{
		OK:          resp.StatusCode >= 200 && resp.StatusCode < 300,
		Status:      resp.StatusCode,
		Duration:    time.Since(start).Milliseconds(),
		Message:     http.StatusText(resp.StatusCode),
		CheckedPath: target,
	}
	if remaining, ok := parseRemaining(resp.Header); ok {
		result.Remaining = &remaining
		result.Usage = &token.UsageInfo{
			Source:       token.NormalizeProvider(selected.Provider),
			APIRemaining: remaining,
			Message:      "API rate-limit header",
		}
	}
	if result.OK && selected.CredentialType == token.CredentialTypeCodexAuthJSON {
		usage, ok := parseCodexUsage(body)
		if ok {
			result.Usage = &usage
			if usage.SubscriptionQuotaAvailable {
				remaining := usage.PrimaryRemainingPercent
				result.Remaining = &remaining
			}
		}
	}
	return result, nil
}

func (v *Validator) validationURL(selected token.Token) (string, error) {
	if selected.CredentialType == token.CredentialTypeCodexAuthJSON {
		return v.cfg.CodexUsageEndpoint, nil
	}

	baseURL := v.baseURL(selected)
	if baseURL == "" {
		return "", fmt.Errorf("%s upstream base url is not configured", selected.Provider)
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	path := "/v1/models"
	switch token.NormalizeProvider(selected.Provider) {
	case token.ProviderAnthropic:
		path = "/v1/models"
	case token.ProviderXiaomi:
		path = "/models"
	}

	out := *base
	out.Path = singleJoiningSlash(base.Path, path)
	out.RawQuery = ""
	return out.String(), nil
}

func (v *Validator) baseURL(selected token.Token) string {
	switch token.NormalizeProvider(selected.Provider) {
	case token.ProviderAnthropic:
		return v.cfg.AnthropicBaseURL
	case token.ProviderDeepSeek:
		return v.cfg.DeepSeekBaseURL
	case token.ProviderKimi:
		return v.cfg.KimiBaseURL
	case token.ProviderXiaomi:
		if selected.CredentialType == token.CredentialTypeMimoTokenPlan {
			return v.cfg.XiaomiTokenPlanBaseURL
		}
		return v.cfg.XiaomiAPIBaseURL
	default:
		if v.cfg.OpenAIBaseURL != "" {
			return v.cfg.OpenAIBaseURL
		}
		return v.cfg.UpstreamBaseURL
	}
}

func parseCodexUsage(body []byte) (token.UsageInfo, bool) {
	var payload struct {
		PlanType  string `json:"plan_type"`
		RateLimit struct {
			LimitReached  bool `json:"limit_reached"`
			PrimaryWindow *struct {
				UsedPercent float64 `json:"used_percent"`
				ResetAt     int64   `json:"reset_at"`
			} `json:"primary_window"`
			SecondaryWindow *struct {
				UsedPercent float64 `json:"used_percent"`
				ResetAt     int64   `json:"reset_at"`
			} `json:"secondary_window"`
		} `json:"rate_limit"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return token.UsageInfo{}, false
	}

	usage := token.UsageInfo{
		Source:       "codex",
		PlanType:     payload.PlanType,
		LimitReached: payload.RateLimit.LimitReached,
		Message:      "Codex usage endpoint",
	}

	if payload.RateLimit.PrimaryWindow != nil {
		used := percent(payload.RateLimit.PrimaryWindow.UsedPercent)
		usage.PrimaryUsedPercent = used
		usage.PrimaryRemainingPercent = 100 - used
		usage.PrimaryResetAt = payload.RateLimit.PrimaryWindow.ResetAt
		usage.SubscriptionQuotaAvailable = true
	}
	if payload.RateLimit.SecondaryWindow != nil {
		used := percent(payload.RateLimit.SecondaryWindow.UsedPercent)
		usage.SecondaryUsedPercent = used
		usage.SecondaryRemainingPercent = 100 - used
		usage.SecondaryResetAt = payload.RateLimit.SecondaryWindow.ResetAt
		usage.SubscriptionQuotaAvailable = true
	}

	return usage, usage.PlanType != "" || usage.SubscriptionQuotaAvailable
}

func percent(value float64) int {
	rounded := int(math.Round(value))
	if rounded < 0 {
		return 0
	}
	if rounded > 100 {
		return 100
	}
	return rounded
}
