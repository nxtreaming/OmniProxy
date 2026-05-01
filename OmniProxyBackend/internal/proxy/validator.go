package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
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

var (
	mimoPlatformAPIBaseURL       = "https://platform.xiaomimimo.com/api/v1"
	mimoTokenPlanPlatformBaseURL = "https://platform.xiaomimimo.com/api/v1/tokenPlan"
	zhipuCodingPlanUsageURL      = "https://api.z.ai/api/monitor/usage/quota/limit"
	zhipuAPIBalanceURL           = "https://bigmodel.cn/api/biz/tokenAccounts/list/my"
)

func NewValidator(cfg config.Config) (*Validator, error) {
	cfg = config.Normalize(cfg)
	for name, baseURL := range map[string]string{
		"openai":                          cfg.OpenAIBaseURL,
		"anthropic":                       cfg.AnthropicBaseURL,
		"deepseek":                        cfg.DeepSeekBaseURL,
		"deepseek_anthropic":              cfg.DeepSeekAnthropicBaseURL,
		"kimi":                            cfg.KimiBaseURL,
		"zhipu":                           cfg.ZhipuBaseURL,
		"zhipu_anthropic":                 cfg.ZhipuAnthropicBaseURL,
		"minimax":                         cfg.MiniMaxBaseURL,
		"minimax_anthropic":               cfg.MiniMaxAnthropicBaseURL,
		"gemini":                          cfg.GeminiBaseURL,
		"custom_gateway":                  cfg.CustomGatewayBaseURL,
		"custom_gateway_anthropic":        cfg.CustomGatewayAnthropicBaseURL,
		"xiaomi_api":                      cfg.XiaomiAPIBaseURL,
		"xiaomi_api_anthropic":            cfg.XiaomiAPIAnthropicBaseURL,
		"xiaomi_token_plan":               cfg.XiaomiTokenPlanBaseURL,
		"xiaomi_token_plan_anthropic":     cfg.XiaomiTokenPlanAnthropicBaseURL,
		"xiaomi_token_plan_sgp":           cfg.XiaomiTokenPlanSGPBaseURL,
		"xiaomi_token_plan_sgp_anthropic": cfg.XiaomiTokenPlanSGPAnthropicBaseURL,
		"codex":                           cfg.CodexBaseURL,
		"codex_usage":                     cfg.CodexUsageEndpoint,
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
	if result.OK && selected.CredentialType != token.CredentialTypeCodexAuthJSON {
		if usage, remaining, ok := v.queryProviderQuota(ctx, selected); ok {
			result.Usage = &usage
			if remaining != nil {
				result.Remaining = remaining
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
	case token.ProviderGemini:
		path = "/v1beta/models"
	case token.ProviderXiaomi:
		path = "/models"
	case token.ProviderZhipu, token.ProviderMiniMax, token.ProviderCustom:
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
	case token.ProviderZhipu:
		return v.cfg.ZhipuBaseURL
	case token.ProviderMiniMax:
		return v.cfg.MiniMaxBaseURL
	case token.ProviderGemini:
		return v.cfg.GeminiBaseURL
	case token.ProviderCustom:
		return v.cfg.CustomGatewayBaseURL
	case token.ProviderXiaomi:
		if selected.CredentialType == token.CredentialTypeMimoTokenPlan {
			if selected.Region == token.MimoRegionSGP {
				return v.cfg.XiaomiTokenPlanSGPBaseURL
			}
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

func (v *Validator) queryProviderQuota(ctx context.Context, selected token.Token) (token.UsageInfo, *int, bool) {
	if token.NormalizeProvider(selected.Provider) == token.ProviderXiaomi && selected.CredentialType == token.CredentialTypeMimoTokenPlan {
		return v.queryMimoTokenPlanUsage(ctx, selected)
	}
	if token.NormalizeProvider(selected.Provider) == token.ProviderZhipu && selected.CredentialType == token.CredentialTypeCodingPlan {
		return v.queryZhipuCodingUsage(ctx, selected)
	}

	if selected.CredentialType != "" && selected.CredentialType != token.CredentialTypeAPIKey {
		return token.UsageInfo{}, nil, false
	}

	switch token.NormalizeProvider(selected.Provider) {
	case token.ProviderDeepSeek:
		return v.queryDeepSeekBalance(ctx, selected)
	case token.ProviderKimi:
		return v.queryKimiCodingUsage(ctx, selected)
	case token.ProviderZhipu:
		return v.queryZhipuAPIBalance(ctx, selected)
	case token.ProviderMiniMax:
		return v.queryMiniMaxCodingUsage(ctx, selected)
	case token.ProviderXiaomi:
		return v.queryMimoBalance(ctx, selected)
	default:
		return token.UsageInfo{}, nil, false
	}
}

func (v *Validator) queryMimoTokenPlanUsage(ctx context.Context, selected token.Token) (token.UsageInfo, *int, bool) {
	if strings.TrimSpace(v.cfg.XiaomiPlatformCookie) == "" {
		return token.UsageInfo{}, nil, false
	}

	target, err := joinURLPath(mimoTokenPlanPlatformBaseURL, "/usage")
	if err != nil {
		return token.UsageInfo{}, nil, false
	}
	body, ok := v.queryMimoPlatformJSON(ctx, selected, target, "https://platform.xiaomimimo.com/console/plan-manage")
	if !ok {
		return token.UsageInfo{}, nil, false
	}

	payload, err := decodeObject(body)
	if err != nil {
		return token.UsageInfo{}, nil, false
	}
	data, ok := responseDataObject(payload)
	if !ok {
		return token.UsageInfo{}, nil, false
	}

	usage := token.UsageInfo{
		Source:   token.ProviderXiaomi,
		PlanType: "MiMo Token Plan",
		Message:  "MiMo Token Plan usage",
	}
	primaryAvailable := false
	secondaryAvailable := false

	if used, remaining, ok := mimoUsageWindowFromMetric(data["monthUsage"]); ok {
		usage.PrimaryUsedPercent = used
		usage.PrimaryRemainingPercent = remaining
		usage.SubscriptionQuotaAvailable = true
		primaryAvailable = true
	}
	if used, remaining, ok := mimoUsageWindowFromMetric(data["usage"]); ok {
		usage.SecondaryUsedPercent = used
		usage.SecondaryRemainingPercent = remaining
		usage.SubscriptionQuotaAvailable = true
		secondaryAvailable = true
	}
	if !usage.SubscriptionQuotaAvailable {
		return token.UsageInfo{}, nil, false
	}

	if detail, ok := v.queryMimoTokenPlanDetail(ctx, selected); ok {
		if planName, ok := stringFromAny(detail["planName"]); ok {
			usage.PlanType = "MiMo " + planName
		}
		if resetAt := unixSecondsFromAny(detail["currentPeriodEnd"]); resetAt > 0 {
			usage.PrimaryResetAt = resetAt
			usage.SecondaryResetAt = resetAt
		}
		if boolFromAny(detail["expired"], false) {
			usage.LimitReached = true
		}
	}

	remaining := usage.PrimaryRemainingPercent
	if !primaryAvailable && secondaryAvailable {
		remaining = usage.SecondaryRemainingPercent
	}
	if usage.LimitReached {
		remaining = 0
	} else {
		usage.LimitReached = remaining <= 0
	}
	return usage, &remaining, true
}

func (v *Validator) queryMimoTokenPlanDetail(ctx context.Context, selected token.Token) (map[string]any, bool) {
	target, err := joinURLPath(mimoTokenPlanPlatformBaseURL, "/detail")
	if err != nil {
		return nil, false
	}
	body, ok := v.queryMimoPlatformJSON(ctx, selected, target, "https://platform.xiaomimimo.com/console/plan-manage")
	if !ok {
		return nil, false
	}
	payload, err := decodeObject(body)
	if err != nil {
		return nil, false
	}
	return responseDataObject(payload)
}

func (v *Validator) queryMimoBalance(ctx context.Context, selected token.Token) (token.UsageInfo, *int, bool) {
	if strings.TrimSpace(v.cfg.XiaomiPlatformCookie) == "" {
		return token.UsageInfo{}, nil, false
	}

	target, err := joinURLPath(mimoPlatformAPIBaseURL, "/balance")
	if err != nil {
		return token.UsageInfo{}, nil, false
	}
	body, ok := v.queryMimoPlatformJSON(ctx, selected, target, "https://platform.xiaomimimo.com/console/balance")
	if !ok {
		return token.UsageInfo{}, nil, false
	}

	payload, err := decodeObject(body)
	if err != nil {
		return token.UsageInfo{}, nil, false
	}
	data, ok := responseDataObject(payload)
	if !ok {
		return token.UsageInfo{}, nil, false
	}

	balance, ok := floatFromAny(data["balance"])
	if !ok {
		return token.UsageInfo{}, nil, false
	}
	unit := "CNY"
	if currency, ok := stringFromAny(data["currency"]); ok {
		unit = currency
	}
	remaining := 100
	if balance <= 0 {
		remaining = 0
	}

	return token.UsageInfo{
		Source:           token.ProviderXiaomi,
		BalanceRemaining: balance,
		BalanceUnit:      unit,
		LimitReached:     balance <= 0,
		Message:          "MiMo account balance",
	}, &remaining, true
}

func (v *Validator) queryMimoPlatformJSON(ctx context.Context, selected token.Token, target string, referer string) ([]byte, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, false
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Timezone", "Asia/Shanghai")
	req.Header.Set("Referer", referer)
	req.Header.Set("Cookie", strings.TrimSpace(v.cfg.XiaomiPlatformCookie))
	if err := applyAuth(req.Header, selected); err != nil {
		return nil, false
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, false
	}
	defer closeBody(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, false
	}
	return body, true
}

func (v *Validator) queryDeepSeekBalance(ctx context.Context, selected token.Token) (token.UsageInfo, *int, bool) {
	target, err := joinURLPath(v.cfg.DeepSeekBaseURL, "/user/balance")
	if err != nil {
		return token.UsageInfo{}, nil, false
	}
	body, ok := v.queryJSON(ctx, selected, target)
	if !ok {
		return token.UsageInfo{}, nil, false
	}

	payload, err := decodeObject(body)
	if err != nil {
		return token.UsageInfo{}, nil, false
	}

	infos, _ := payload["balance_infos"].([]any)
	balance, unit, found := deepSeekBalanceFromInfos(infos)
	if !found {
		return token.UsageInfo{}, nil, false
	}

	available := boolFromAny(payload["is_available"], true)
	remaining := 100
	if !available || balance <= 0 {
		remaining = 0
	}

	return token.UsageInfo{
		Source:           token.ProviderDeepSeek,
		BalanceRemaining: balance,
		BalanceUnit:      unit,
		LimitReached:     !available || balance <= 0,
		Message:          "DeepSeek balance",
	}, &remaining, true
}

type deepSeekBalanceEntry struct {
	unit    string
	balance float64
}

func deepSeekBalanceFromInfos(infos []any) (float64, string, bool) {
	entries := make([]deepSeekBalanceEntry, 0, len(infos))
	for _, raw := range infos {
		info, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		balance, ok := deepSeekBalanceValue(info)
		if !ok {
			continue
		}
		unit := "CNY"
		if currency, ok := stringFromAny(info["currency"]); ok && strings.TrimSpace(currency) != "" {
			unit = strings.ToUpper(strings.TrimSpace(currency))
		}
		entries = append(entries, deepSeekBalanceEntry{
			unit:    unit,
			balance: balance,
		})
	}
	if len(entries) == 0 {
		return 0, "", false
	}
	for _, preferredUnit := range []string{"CNY", "USD"} {
		for _, entry := range entries {
			if entry.unit == preferredUnit && entry.balance > 0 {
				return entry.balance, entry.unit, true
			}
		}
	}
	for _, entry := range entries {
		if entry.balance > 0 {
			return entry.balance, entry.unit, true
		}
	}
	return entries[0].balance, entries[0].unit, true
}

func deepSeekBalanceValue(info map[string]any) (float64, bool) {
	if balance, ok := floatFromAny(info["total_balance"]); ok {
		return balance, true
	}
	granted, grantedOK := floatFromAny(info["granted_balance"])
	toppedUp, toppedUpOK := floatFromAny(info["topped_up_balance"])
	if grantedOK || toppedUpOK {
		return granted + toppedUp, true
	}
	return 0, false
}

func (v *Validator) queryKimiCodingUsage(ctx context.Context, selected token.Token) (token.UsageInfo, *int, bool) {
	target, err := joinURLPath(v.cfg.KimiBaseURL, "/v1/usages")
	if err != nil {
		return token.UsageInfo{}, nil, false
	}
	body, ok := v.queryJSON(ctx, selected, target)
	if !ok {
		return token.UsageInfo{}, nil, false
	}

	payload, err := decodeObject(body)
	if err != nil {
		return token.UsageInfo{}, nil, false
	}

	usage := token.UsageInfo{
		Source:   token.ProviderKimi,
		PlanType: "Kimi Token Plan",
		Message:  "Kimi coding usage",
	}

	if limits, ok := payload["limits"].([]any); ok {
		for _, raw := range limits {
			limitItem, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			detail, ok := limitItem["detail"].(map[string]any)
			if !ok {
				continue
			}
			used, remaining, resetAt, ok := usageWindowFromLimit(detail)
			if !ok {
				continue
			}
			usage.PrimaryUsedPercent = used
			usage.PrimaryRemainingPercent = remaining
			usage.PrimaryResetAt = resetAt
			usage.SubscriptionQuotaAvailable = true
			break
		}
	}

	if raw, ok := payload["usage"].(map[string]any); ok {
		used, remaining, resetAt, ok := usageWindowFromLimit(raw)
		if ok {
			usage.SecondaryUsedPercent = used
			usage.SecondaryRemainingPercent = remaining
			usage.SecondaryResetAt = resetAt
			usage.SubscriptionQuotaAvailable = true
		}
	}

	if !usage.SubscriptionQuotaAvailable {
		return token.UsageInfo{}, nil, false
	}
	primaryAvailable := usage.PrimaryUsedPercent != 0 || usage.PrimaryRemainingPercent != 0 || usage.PrimaryResetAt != 0
	remaining := usage.PrimaryRemainingPercent
	if !primaryAvailable && usage.SecondaryRemainingPercent > 0 {
		remaining = usage.SecondaryRemainingPercent
	}
	usage.LimitReached = remaining <= 0
	return usage, &remaining, true
}

func (v *Validator) queryZhipuAPIBalance(ctx context.Context, selected token.Token) (token.UsageInfo, *int, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, zhipuAPIBalanceURL, nil)
	if err != nil {
		return token.UsageInfo{}, nil, false
	}
	req.Header.Set("Accept", "application/json")
	secret, err := credentialSecret(selected)
	if err != nil {
		return token.UsageInfo{}, nil, false
	}
	req.Header.Set("Authorization", "Bearer "+secret)

	resp, err := v.client.Do(req)
	if err != nil {
		return token.UsageInfo{}, nil, false
	}
	defer closeBody(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return token.UsageInfo{}, nil, false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return token.UsageInfo{}, nil, false
	}
	payload, err := decodeObject(body)
	if err != nil {
		return token.UsageInfo{}, nil, false
	}
	if boolFromAny(payload["success"], true) == false {
		return token.UsageInfo{}, nil, false
	}

	rows := zhipuBalanceRows(payload)
	if len(rows) == 0 {
		return token.UsageInfo{}, nil, false
	}
	total := 0.0
	for _, row := range rows {
		if balance, ok := zhipuBalanceValue(row); ok {
			total += balance
		}
	}

	remaining := 100
	if total <= 0 {
		remaining = 0
	}
	return token.UsageInfo{
		Source:           token.ProviderZhipu,
		PlanType:         "Zhipu GLM API Key",
		BalanceRemaining: total,
		BalanceUnit:      "Token",
		LimitReached:     total <= 0,
		Message:          "Zhipu GLM API balance",
	}, &remaining, true
}

func zhipuBalanceRows(payload map[string]any) []any {
	if rows, ok := payload["rows"].([]any); ok {
		return rows
	}
	if data, ok := payload["data"].(map[string]any); ok {
		if rows, ok := data["rows"].([]any); ok {
			return rows
		}
		if rows, ok := data["list"].([]any); ok {
			return rows
		}
	}
	return nil
}

func zhipuBalanceValue(row any) (float64, bool) {
	item, ok := row.(map[string]any)
	if !ok {
		return 0, false
	}
	for _, key := range []string{"tokenBalance", "balance", "availableBalance", "remaining"} {
		if balance, ok := floatFromAny(item[key]); ok {
			return balance, true
		}
	}
	return 0, false
}

func (v *Validator) queryZhipuCodingUsage(ctx context.Context, selected token.Token) (token.UsageInfo, *int, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, zhipuCodingPlanUsageURL, nil)
	if err != nil {
		return token.UsageInfo{}, nil, false
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Language", "en-US,en")
	secret, err := credentialSecret(selected)
	if err != nil {
		return token.UsageInfo{}, nil, false
	}
	req.Header.Set("Authorization", secret)

	resp, err := v.client.Do(req)
	if err != nil {
		return token.UsageInfo{}, nil, false
	}
	defer closeBody(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return token.UsageInfo{}, nil, false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return token.UsageInfo{}, nil, false
	}
	payload, err := decodeObject(body)
	if err != nil {
		return token.UsageInfo{}, nil, false
	}
	if boolFromAny(payload["success"], true) == false {
		return token.UsageInfo{}, nil, false
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		return token.UsageInfo{}, nil, false
	}

	usage := token.UsageInfo{
		Source:   token.ProviderZhipu,
		PlanType: "Zhipu GLM Token Plan",
		Message:  "Zhipu GLM coding usage",
	}
	if level, ok := stringFromAny(data["level"]); ok {
		usage.PlanType = "Zhipu GLM " + level
	}

	primaryFound := false
	secondaryFound := false
	if limits, ok := data["limits"].([]any); ok {
		for _, raw := range limits {
			limitItem, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			limitType, _ := stringFromAny(limitItem["type"])
			if limitType != "TOKENS_LIMIT" {
				continue
			}
			used, remaining, resetAt, ok := zhipuCodingUsageWindow(limitItem)
			if !ok {
				continue
			}
			if zhipuCodingLimitIsWeekly(limitItem) || primaryFound {
				usage.SecondaryUsedPercent = used
				usage.SecondaryRemainingPercent = remaining
				usage.SecondaryResetAt = resetAt
				secondaryFound = true
			} else {
				usage.PrimaryUsedPercent = used
				usage.PrimaryRemainingPercent = remaining
				usage.PrimaryResetAt = resetAt
				primaryFound = true
			}
			usage.SubscriptionQuotaAvailable = true
		}
	}
	if !usage.SubscriptionQuotaAvailable {
		return token.UsageInfo{}, nil, false
	}

	remaining := usage.PrimaryRemainingPercent
	if !primaryFound && secondaryFound {
		remaining = usage.SecondaryRemainingPercent
	} else if primaryFound && secondaryFound && usage.SecondaryRemainingPercent < remaining {
		remaining = usage.SecondaryRemainingPercent
	}
	usage.LimitReached = remaining <= 0
	return usage, &remaining, true
}

func zhipuCodingUsageWindow(limitItem map[string]any) (int, int, int64, bool) {
	if usedValue, ok := floatFromAny(limitItem["percentage"]); ok {
		used := percent(usedValue)
		return used, 100 - used, unixSecondsFromAny(limitItem["nextResetTime"]), true
	}
	total, totalOK := floatFromAny(limitItem["usage"])
	usedCount, usedOK := floatFromAny(limitItem["currentValue"])
	if !totalOK || !usedOK || total <= 0 {
		return 0, 0, 0, false
	}
	used := percent((usedCount / total) * 100)
	remaining := 100 - used
	if remaining < 0 {
		remaining = 0
	}
	return used, remaining, unixSecondsFromAny(limitItem["nextResetTime"]), true
}

func zhipuCodingLimitIsWeekly(limitItem map[string]any) bool {
	unit, unitOK := floatFromAny(limitItem["unit"])
	number, numberOK := floatFromAny(limitItem["number"])
	if unitOK && numberOK {
		if int(unit) == 6 && int(number) >= 7 {
			return true
		}
		if int(number) >= 7 {
			return true
		}
	}
	window, _ := stringFromAny(limitItem["name"])
	if strings.Contains(strings.ToLower(window), "week") || strings.Contains(window, "周") {
		return true
	}
	return false
}

func (v *Validator) queryMiniMaxCodingUsage(ctx context.Context, selected token.Token) (token.UsageInfo, *int, bool) {
	target, err := joinURLPath(v.cfg.MiniMaxBaseURL, "/api/openplatform/coding_plan/remains")
	if err != nil {
		return token.UsageInfo{}, nil, false
	}
	body, ok := v.queryJSON(ctx, selected, target)
	if !ok {
		return token.UsageInfo{}, nil, false
	}

	payload, err := decodeObject(body)
	if err != nil {
		return token.UsageInfo{}, nil, false
	}
	if baseResp, ok := payload["base_resp"].(map[string]any); ok {
		statusCode, _ := floatFromAny(baseResp["status_code"])
		if statusCode != 0 {
			return token.UsageInfo{}, nil, false
		}
	}

	modelRemains, ok := payload["model_remains"].([]any)
	if !ok || len(modelRemains) == 0 {
		return token.UsageInfo{}, nil, false
	}
	first, ok := modelRemains[0].(map[string]any)
	if !ok {
		return token.UsageInfo{}, nil, false
	}

	usage := token.UsageInfo{
		Source:   token.ProviderMiniMax,
		PlanType: "MiniMax Token Plan",
		Message:  "MiniMax coding usage",
	}
	if modelName, ok := stringFromAny(first["model"]); ok {
		usage.PlanType = "MiniMax " + modelName
	}

	primaryAvailable := false
	secondaryAvailable := false
	if used, remaining, resetAt, ok := minimaxUsageWindow(first, "current_interval_total_count", "current_interval_usage_count", "end_time"); ok {
		usage.PrimaryUsedPercent = used
		usage.PrimaryRemainingPercent = remaining
		usage.PrimaryResetAt = resetAt
		usage.SubscriptionQuotaAvailable = true
		primaryAvailable = true
	}
	if used, remaining, resetAt, ok := minimaxUsageWindow(first, "current_weekly_total_count", "current_weekly_usage_count", "weekly_end_time"); ok {
		usage.SecondaryUsedPercent = used
		usage.SecondaryRemainingPercent = remaining
		usage.SecondaryResetAt = resetAt
		usage.SubscriptionQuotaAvailable = true
		secondaryAvailable = true
	}
	if !usage.SubscriptionQuotaAvailable {
		return token.UsageInfo{}, nil, false
	}

	remaining := usage.PrimaryRemainingPercent
	if !primaryAvailable && secondaryAvailable {
		remaining = usage.SecondaryRemainingPercent
	}
	usage.LimitReached = remaining <= 0
	return usage, &remaining, true
}

func minimaxUsageWindow(value map[string]any, totalKey string, remainingKey string, resetKey string) (int, int, int64, bool) {
	total, ok := floatFromAny(value[totalKey])
	if !ok || total <= 0 {
		return 0, 0, 0, false
	}
	remainingValue, ok := floatFromAny(value[remainingKey])
	if !ok {
		return 0, 0, 0, false
	}
	used := percent(((total - remainingValue) / total) * 100)
	remaining := 100 - used
	if remaining < 0 {
		remaining = 0
	}
	return used, remaining, unixSecondsFromAny(value[resetKey]), true
}

func (v *Validator) queryJSON(ctx context.Context, selected token.Token, target string) ([]byte, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, false
	}
	req.Header.Set("Accept", "application/json")
	if err := applyAuth(req.Header, selected); err != nil {
		return nil, false
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, false
	}
	defer closeBody(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, false
	}
	return body, true
}

func responseDataObject(payload map[string]any) (map[string]any, bool) {
	if code, ok := floatFromAny(payload["code"]); ok && code != 0 {
		return nil, false
	}
	data, ok := payload["data"].(map[string]any)
	return data, ok
}

func mimoUsageWindowFromMetric(value any) (int, int, bool) {
	metric, ok := value.(map[string]any)
	if !ok {
		return 0, 0, false
	}

	if used, ok := mimoUsagePercentFromItems(metric["items"]); ok {
		return used, 100 - used, true
	}
	if used, ok := percentFromUsageRatio(metric["percent"]); ok {
		return used, 100 - used, true
	}
	return 0, 0, false
}

func mimoUsagePercentFromItems(value any) (int, bool) {
	items, ok := value.([]any)
	if !ok {
		return 0, false
	}

	var usedTokens float64
	var limitTokens float64
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		limit, limitOK := floatFromAny(item["limit"])
		used, usedOK := floatFromAny(item["used"])
		if !limitOK || !usedOK || limit <= 0 {
			continue
		}
		usedTokens += used
		limitTokens += limit
	}
	if limitTokens <= 0 {
		return 0, false
	}
	return percent((usedTokens / limitTokens) * 100), true
}

func percentFromUsageRatio(value any) (int, bool) {
	ratio, ok := floatFromAny(value)
	if !ok {
		return 0, false
	}
	if ratio <= 1 {
		ratio *= 100
	}
	return percent(ratio), true
}

func usageWindowFromLimit(value map[string]any) (int, int, int64, bool) {
	limit, ok := floatFromAny(value["limit"])
	if !ok || limit <= 0 {
		return 0, 0, 0, false
	}
	remainingValue, ok := floatFromAny(value["remaining"])
	if !ok {
		return 0, 0, 0, false
	}
	used := percent(((limit - remainingValue) / limit) * 100)
	remaining := 100 - used
	if remaining < 0 {
		remaining = 0
	}
	resetAt := unixSecondsFromAny(value["resetTime"])
	return used, remaining, resetAt, true
}

func joinURLPath(baseURL string, path string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	out := *base
	out.Path = singleJoiningSlash(base.Path, path)
	out.RawQuery = ""
	return out.String(), nil
}

func decodeObject(body []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var payload map[string]any
	if err := decoder.Decode(&payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func floatFromAny(value any) (float64, bool) {
	switch typed := value.(type) {
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func boolFromAny(value any, fallback bool) bool {
	typed, ok := value.(bool)
	if !ok {
		return fallback
	}
	return typed
}

func unixSecondsFromAny(value any) int64 {
	switch typed := value.(type) {
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return normalizeUnixSeconds(parsed)
		}
	case float64:
		return normalizeUnixSeconds(int64(typed))
	case int64:
		return normalizeUnixSeconds(typed)
	case int:
		return normalizeUnixSeconds(int64(typed))
	case string:
		text := strings.TrimSpace(typed)
		if parsed, err := strconv.ParseInt(text, 10, 64); err == nil {
			return normalizeUnixSeconds(parsed)
		}
		if parsed, err := time.Parse(time.RFC3339Nano, text); err == nil {
			return parsed.Unix()
		}
		for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04"} {
			if parsed, err := time.ParseInLocation(layout, text, time.Local); err == nil {
				return parsed.Unix()
			}
		}
	}
	return 0
}

func normalizeUnixSeconds(value int64) int64 {
	if value > 1_000_000_000_000 {
		return value / 1000
	}
	return value
}
