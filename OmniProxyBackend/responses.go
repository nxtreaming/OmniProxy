package main

import (
	"time"

	"OmniProxyBackend/internal/history"
	"OmniProxyBackend/internal/logs"
	"OmniProxyBackend/internal/proxy"
	"OmniProxyBackend/internal/token"
)

type tokenResponse struct {
	ID               string             `json:"id"`
	Name             string             `json:"name"`
	Provider         string             `json:"provider"`
	CredentialType   string             `json:"credentialType"`
	HasTokenValue    bool               `json:"hasTokenValue"`
	MaskedTokenValue string             `json:"maskedTokenValue,omitempty"`
	Remaining        int                `json:"remaining"`
	Usage            usageResponse      `json:"usage"`
	Stats            tokenStatsResponse `json:"stats"`
	Health           healthResponse     `json:"health"`
	Status           token.Status       `json:"status"`
	LastUsedAt       string             `json:"lastUsedAt,omitempty"`
	LastError        string             `json:"lastError,omitempty"`
	CooldownUntil    string             `json:"cooldownUntil,omitempty"`
	CreatedAt        string             `json:"createdAt"`
	UpdatedAt        string             `json:"updatedAt"`
}

type usageResponse struct {
	Source                     string  `json:"source,omitempty"`
	PlanType                   string  `json:"planType,omitempty"`
	LimitReached               bool    `json:"limitReached,omitempty"`
	PrimaryUsedPercent         int     `json:"primaryUsedPercent,omitempty"`
	PrimaryRemainingPercent    int     `json:"primaryRemainingPercent,omitempty"`
	PrimaryResetAt             int64   `json:"primaryResetAt,omitempty"`
	SecondaryUsedPercent       int     `json:"secondaryUsedPercent,omitempty"`
	SecondaryRemainingPercent  int     `json:"secondaryRemainingPercent,omitempty"`
	SecondaryResetAt           int64   `json:"secondaryResetAt,omitempty"`
	APIRemaining               int     `json:"apiRemaining,omitempty"`
	BalanceRemaining           float64 `json:"balanceRemaining,omitempty"`
	BalanceTotal               float64 `json:"balanceTotal,omitempty"`
	BalanceUsed                float64 `json:"balanceUsed,omitempty"`
	BalanceUnit                string  `json:"balanceUnit,omitempty"`
	SubscriptionQuotaAvailable bool    `json:"subscriptionQuotaAvailable,omitempty"`
	Message                    string  `json:"message,omitempty"`
	UpdatedAt                  string  `json:"updatedAt,omitempty"`
}

type tokenStatsResponse struct {
	RequestCount     int64                   `json:"requestCount"`
	InputTokens      int64                   `json:"inputTokens"`
	OutputTokens     int64                   `json:"outputTokens"`
	TotalTokens      int64                   `json:"totalTokens"`
	LastInputTokens  int                     `json:"lastInputTokens,omitempty"`
	LastOutputTokens int                     `json:"lastOutputTokens,omitempty"`
	LastTotalTokens  int                     `json:"lastTotalTokens,omitempty"`
	Daily            []token.DailyTokenUsage `json:"daily,omitempty"`
	UpdatedAt        string                  `json:"updatedAt,omitempty"`
}

type healthResponse struct {
	LastCheckedAt     string `json:"lastCheckedAt,omitempty"`
	NextCheckAt       string `json:"nextCheckAt,omitempty"`
	ConsecutiveErrors int    `json:"consecutiveErrors,omitempty"`
	LastStatus        int    `json:"lastStatus,omitempty"`
	LastMessage       string `json:"lastMessage,omitempty"`
}

type validationResponse struct {
	OK          bool           `json:"ok"`
	Status      int            `json:"status"`
	Duration    int64          `json:"durationMs"`
	Remaining   *int           `json:"remaining,omitempty"`
	Usage       *usageResponse `json:"usage,omitempty"`
	Message     string         `json:"message"`
	CheckedPath string         `json:"checkedPath"`
}

type logResponse struct {
	ID        int64      `json:"id"`
	Time      string     `json:"time"`
	Level     logs.Level `json:"level"`
	Method    string     `json:"method,omitempty"`
	Path      string     `json:"path,omitempty"`
	Model     string     `json:"model,omitempty"`
	Status    int        `json:"status,omitempty"`
	Duration  int64      `json:"durationMs,omitempty"`
	TokenName string     `json:"tokenName,omitempty"`
	Message   string     `json:"message"`
}

type historyResponse struct {
	ID                int64                  `json:"id"`
	Time              string                 `json:"time"`
	Level             string                 `json:"level"`
	Method            string                 `json:"method,omitempty"`
	Path              string                 `json:"path,omitempty"`
	Provider          string                 `json:"provider,omitempty"`
	Protocol          string                 `json:"protocol,omitempty"`
	Model             string                 `json:"model,omitempty"`
	Status            int                    `json:"status,omitempty"`
	Duration          int64                  `json:"durationMs,omitempty"`
	TokenID           string                 `json:"tokenId,omitempty"`
	TokenName         string                 `json:"tokenName,omitempty"`
	InputTokens       int                    `json:"inputTokens,omitempty"`
	OutputTokens      int                    `json:"outputTokens,omitempty"`
	TotalTokens       int                    `json:"totalTokens,omitempty"`
	CooldownTriggered bool                   `json:"cooldownTriggered,omitempty"`
	RetryChain        []retryAttemptResponse `json:"retryChain,omitempty"`
	Message           string                 `json:"message"`
}

type retryAttemptResponse struct {
	Attempt           int    `json:"attempt"`
	Provider          string `json:"provider,omitempty"`
	Protocol          string `json:"protocol,omitempty"`
	Model             string `json:"model,omitempty"`
	Status            int    `json:"status,omitempty"`
	Duration          int64  `json:"durationMs,omitempty"`
	TokenID           string `json:"tokenId,omitempty"`
	TokenName         string `json:"tokenName,omitempty"`
	CooldownTriggered bool   `json:"cooldownTriggered,omitempty"`
	Message           string `json:"message,omitempty"`
}

func tokenResponses(items []token.Token) []tokenResponse {
	out := make([]tokenResponse, len(items))
	for i, item := range items {
		out[i] = tokenResponseFor(item)
	}
	return out
}

func tokenResponseFor(item token.Token) tokenResponse {
	return tokenResponse{
		ID:               item.ID,
		Name:             item.Name,
		Provider:         item.Provider,
		CredentialType:   item.CredentialType,
		HasTokenValue:    item.TokenValue != "",
		MaskedTokenValue: maskedTokenValue(item),
		Remaining:        item.Remaining,
		Usage:            usageResponseFor(item.Usage),
		Stats:            tokenStatsResponseFor(item.Stats),
		Health:           healthResponseFor(item.Health),
		Status:           item.Status,
		LastUsedAt:       timePtrString(item.LastUsedAt),
		LastError:        item.LastError,
		CooldownUntil:    timePtrString(item.CooldownUntil),
		CreatedAt:        timeString(item.CreatedAt),
		UpdatedAt:        timeString(item.UpdatedAt),
	}
}

func usageResponseFor(usage token.UsageInfo) usageResponse {
	return usageResponse{
		Source:                     usage.Source,
		PlanType:                   usage.PlanType,
		LimitReached:               usage.LimitReached,
		PrimaryUsedPercent:         usage.PrimaryUsedPercent,
		PrimaryRemainingPercent:    usage.PrimaryRemainingPercent,
		PrimaryResetAt:             usage.PrimaryResetAt,
		SecondaryUsedPercent:       usage.SecondaryUsedPercent,
		SecondaryRemainingPercent:  usage.SecondaryRemainingPercent,
		SecondaryResetAt:           usage.SecondaryResetAt,
		APIRemaining:               usage.APIRemaining,
		BalanceRemaining:           usage.BalanceRemaining,
		BalanceTotal:               usage.BalanceTotal,
		BalanceUsed:                usage.BalanceUsed,
		BalanceUnit:                usage.BalanceUnit,
		SubscriptionQuotaAvailable: usage.SubscriptionQuotaAvailable,
		Message:                    usage.Message,
		UpdatedAt:                  timePtrString(usage.UpdatedAt),
	}
}

func tokenStatsResponseFor(stats token.TokenStats) tokenStatsResponse {
	return tokenStatsResponse{
		RequestCount:     stats.RequestCount,
		InputTokens:      stats.InputTokens,
		OutputTokens:     stats.OutputTokens,
		TotalTokens:      stats.TotalTokens,
		LastInputTokens:  stats.LastInputTokens,
		LastOutputTokens: stats.LastOutputTokens,
		LastTotalTokens:  stats.LastTotalTokens,
		Daily:            append([]token.DailyTokenUsage(nil), stats.Daily...),
		UpdatedAt:        timePtrString(stats.UpdatedAt),
	}
}

func healthResponseFor(health token.HealthInfo) healthResponse {
	return healthResponse{
		LastCheckedAt:     timePtrString(health.LastCheckedAt),
		NextCheckAt:       timePtrString(health.NextCheckAt),
		ConsecutiveErrors: health.ConsecutiveErrors,
		LastStatus:        health.LastStatus,
		LastMessage:       health.LastMessage,
	}
}

func validationResponseFor(result proxy.ValidationResult) validationResponse {
	out := validationResponse{
		OK:          result.OK,
		Status:      result.Status,
		Duration:    result.Duration,
		Remaining:   result.Remaining,
		Message:     result.Message,
		CheckedPath: result.CheckedPath,
	}
	if result.Usage != nil {
		usage := usageResponseFor(*result.Usage)
		out.Usage = &usage
	}
	return out
}

func logResponses(entries []logs.Entry) []logResponse {
	out := make([]logResponse, len(entries))
	for i, entry := range entries {
		out[i] = logResponseFor(entry)
	}
	return out
}

func logResponseFor(entry logs.Entry) logResponse {
	return logResponse{
		ID:        entry.ID,
		Time:      timeString(entry.Time),
		Level:     entry.Level,
		Method:    entry.Method,
		Path:      entry.Path,
		Model:     entry.Model,
		Status:    entry.Status,
		Duration:  entry.Duration,
		TokenName: entry.TokenName,
		Message:   entry.Message,
	}
}

func historyResponses(entries []history.Entry) []historyResponse {
	out := make([]historyResponse, len(entries))
	for i, entry := range entries {
		out[i] = historyResponseFor(entry)
	}
	return out
}

func historyResponseFor(entry history.Entry) historyResponse {
	return historyResponse{
		ID:                entry.ID,
		Time:              timeString(entry.Time),
		Level:             entry.Level,
		Method:            entry.Method,
		Path:              entry.Path,
		Provider:          entry.Provider,
		Protocol:          entry.Protocol,
		Model:             entry.Model,
		Status:            entry.Status,
		Duration:          entry.Duration,
		TokenID:           entry.TokenID,
		TokenName:         entry.TokenName,
		InputTokens:       entry.InputTokens,
		OutputTokens:      entry.OutputTokens,
		TotalTokens:       entry.TotalTokens,
		CooldownTriggered: entry.CooldownTriggered,
		RetryChain:        retryAttemptResponses(entry.RetryChain),
		Message:           entry.Message,
	}
}

func retryAttemptResponses(entries []history.RetryAttempt) []retryAttemptResponse {
	if len(entries) == 0 {
		return nil
	}
	out := make([]retryAttemptResponse, len(entries))
	for i, entry := range entries {
		out[i] = retryAttemptResponse{
			Attempt:           entry.Attempt,
			Provider:          entry.Provider,
			Protocol:          entry.Protocol,
			Model:             entry.Model,
			Status:            entry.Status,
			Duration:          entry.Duration,
			TokenID:           entry.TokenID,
			TokenName:         entry.TokenName,
			CooldownTriggered: entry.CooldownTriggered,
			Message:           entry.Message,
		}
	}
	return out
}

func timeString(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}

func timePtrString(value *time.Time) string {
	if value == nil {
		return ""
	}
	return timeString(*value)
}
