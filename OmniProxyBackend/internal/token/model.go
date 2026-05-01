package token

import "time"

type Status string

const (
	StatusActive    Status = "active"
	StatusLow       Status = "low"
	StatusExhausted Status = "exhausted"
	StatusInvalid   Status = "invalid"
)

const (
	ProviderOpenAI    = "openai"
	ProviderAnthropic = "anthropic"
	ProviderDeepSeek  = "deepseek"
	ProviderKimi      = "kimi"
	ProviderXiaomi    = "xiaomi"
	ProviderZhipu     = "zhipu"
	ProviderMiniMax   = "minimax"
	ProviderGemini    = "gemini"
	ProviderCustom    = "custom"
)

const (
	CredentialTypeAPIKey        = "api_key"
	CredentialTypeCodexAuthJSON = "codex_auth_json"
	CredentialTypeMimoTokenPlan = "mimo_token_plan"
	CredentialTypeCodingPlan    = "coding_plan"
	CredentialTypeClaudeOAuth   = "claude_oauth_json"
)

const (
	MimoRegionCN  = "cn"
	MimoRegionSGP = "sgp"
)

type Token struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Provider       string     `json:"provider"`
	CredentialType string     `json:"credentialType"`
	Region         string     `json:"region,omitempty"`
	TokenValue     string     `json:"tokenValue"`
	Remaining      int        `json:"remaining"`
	Usage          UsageInfo  `json:"usage"`
	Stats          TokenStats `json:"stats"`
	Health         HealthInfo `json:"health"`
	Status         Status     `json:"status"`
	LastUsedAt     *time.Time `json:"lastUsedAt,omitempty"`
	LastError      string     `json:"lastError,omitempty"`
	CooldownUntil  *time.Time `json:"cooldownUntil,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

type UsageInfo struct {
	Source                     string     `json:"source,omitempty"`
	PlanType                   string     `json:"planType,omitempty"`
	LimitReached               bool       `json:"limitReached,omitempty"`
	PrimaryUsedPercent         int        `json:"primaryUsedPercent,omitempty"`
	PrimaryRemainingPercent    int        `json:"primaryRemainingPercent,omitempty"`
	PrimaryResetAt             int64      `json:"primaryResetAt,omitempty"`
	SecondaryUsedPercent       int        `json:"secondaryUsedPercent,omitempty"`
	SecondaryRemainingPercent  int        `json:"secondaryRemainingPercent,omitempty"`
	SecondaryResetAt           int64      `json:"secondaryResetAt,omitempty"`
	APIRemaining               int        `json:"apiRemaining,omitempty"`
	BalanceRemaining           float64    `json:"balanceRemaining,omitempty"`
	BalanceTotal               float64    `json:"balanceTotal,omitempty"`
	BalanceUsed                float64    `json:"balanceUsed,omitempty"`
	BalanceUnit                string     `json:"balanceUnit,omitempty"`
	SubscriptionQuotaAvailable bool       `json:"subscriptionQuotaAvailable,omitempty"`
	Message                    string     `json:"message,omitempty"`
	UpdatedAt                  *time.Time `json:"updatedAt,omitempty"`
}

type TokenStats struct {
	RequestCount     int64             `json:"requestCount"`
	InputTokens      int64             `json:"inputTokens"`
	OutputTokens     int64             `json:"outputTokens"`
	TotalTokens      int64             `json:"totalTokens"`
	LastInputTokens  int               `json:"lastInputTokens,omitempty"`
	LastOutputTokens int               `json:"lastOutputTokens,omitempty"`
	LastTotalTokens  int               `json:"lastTotalTokens,omitempty"`
	Daily            []DailyTokenUsage `json:"daily,omitempty"`
	UpdatedAt        *time.Time        `json:"updatedAt,omitempty"`
}

type HealthInfo struct {
	LastCheckedAt     *time.Time `json:"lastCheckedAt,omitempty"`
	NextCheckAt       *time.Time `json:"nextCheckAt,omitempty"`
	ConsecutiveErrors int        `json:"consecutiveErrors,omitempty"`
	LastStatus        int        `json:"lastStatus,omitempty"`
	LastMessage       string     `json:"lastMessage,omitempty"`
}

type DailyTokenUsage struct {
	Date         string `json:"date"`
	RequestCount int64  `json:"requestCount"`
	InputTokens  int64  `json:"inputTokens"`
	OutputTokens int64  `json:"outputTokens"`
	TotalTokens  int64  `json:"totalTokens"`
}

type TokenConsumption struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
	TotalTokens  int `json:"totalTokens"`
}

type UpsertRequest struct {
	Name           string `json:"name"`
	Provider       string `json:"provider"`
	CredentialType string `json:"credentialType"`
	Region         string `json:"region,omitempty"`
	TokenValue     string `json:"tokenValue"`
}
