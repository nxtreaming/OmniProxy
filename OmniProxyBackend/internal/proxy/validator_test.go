package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"OmniProxyBackend/internal/config"
	"OmniProxyBackend/internal/token"
)

func TestValidatorUsesAnthropicAPIKey(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Fatalf("expected no Authorization header, got %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("x-api-key") != "anthropic-token-value" {
			t.Fatalf("unexpected x-api-key header: %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Fatal("expected anthropic-version header")
		}
		w.Header().Set("x-ratelimit-remaining", "42")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	validator, err := NewValidator(config.Config{
		ProxyPort:        3000,
		ControlPort:      3890,
		UpstreamBaseURL:  upstream.URL,
		AnthropicBaseURL: upstream.URL,
		SwitchThreshold:  15,
		MaxRetries:       1,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := validator.Validate(t.Context(), token.Token{
		Provider:   "anthropic",
		TokenValue: "anthropic-token-value",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("expected validation to pass: %#v", result)
	}
	if result.Remaining == nil || *result.Remaining != 42 {
		t.Fatalf("expected remaining 42, got %#v", result.Remaining)
	}
}

func TestValidatorQueriesDeepSeekBalance(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer deepseek-token-value" {
			t.Fatalf("unexpected Authorization header: %q", r.Header.Get("Authorization"))
		}
		switch r.URL.Path {
		case "/v1/models":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[]}`))
		case "/user/balance":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"is_available": true,
				"balance_infos": [
					{"currency": "CNY", "total_balance": "12.50"}
				]
			}`))
		default:
			t.Fatalf("unexpected validation path: %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	validator, err := NewValidator(config.Config{
		ProxyPort:       3000,
		ControlPort:     3890,
		DeepSeekBaseURL: upstream.URL,
		SwitchThreshold: 15,
		MaxRetries:      1,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := validator.Validate(t.Context(), token.Token{
		Provider:   token.ProviderDeepSeek,
		TokenValue: "deepseek-token-value",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Usage == nil {
		t.Fatal("expected balance usage details")
	}
	if result.Usage.BalanceUnit != "CNY" || result.Usage.BalanceRemaining != 12.5 {
		t.Fatalf("unexpected balance usage: %#v", result.Usage)
	}
	if result.Remaining == nil || *result.Remaining != 100 {
		t.Fatalf("expected positive balance to map to remaining 100, got %#v", result.Remaining)
	}
}

func TestValidatorQueriesKimiCodingUsage(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer kimi-token-value" {
			t.Fatalf("unexpected Authorization header: %q", r.Header.Get("Authorization"))
		}
		switch r.URL.Path {
		case "/coding/v1/models":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[]}`))
		case "/coding/v1/usages":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"limits": [
					{"detail": {"limit": 1000, "remaining": 750, "resetTime": 1760000000000}}
				],
				"usage": {"limit": "2000", "remaining": "1400", "resetTime": "1760500000"}
			}`))
		default:
			t.Fatalf("unexpected validation path: %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	validator, err := NewValidator(config.Config{
		ProxyPort:       3000,
		ControlPort:     3890,
		KimiBaseURL:     upstream.URL + "/coding",
		SwitchThreshold: 15,
		MaxRetries:      1,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := validator.Validate(t.Context(), token.Token{
		Provider:   token.ProviderKimi,
		TokenValue: "kimi-token-value",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Usage == nil || !result.Usage.SubscriptionQuotaAvailable {
		t.Fatalf("expected token plan usage details, got %#v", result.Usage)
	}
	if result.Usage.PrimaryUsedPercent != 25 || result.Usage.PrimaryRemainingPercent != 75 {
		t.Fatalf("unexpected primary usage: %#v", result.Usage)
	}
	if result.Usage.SecondaryUsedPercent != 30 || result.Usage.SecondaryRemainingPercent != 70 {
		t.Fatalf("unexpected secondary usage: %#v", result.Usage)
	}
	if result.Usage.PrimaryResetAt != 1760000000 || result.Usage.SecondaryResetAt != 1760500000 {
		t.Fatalf("unexpected reset times: %#v", result.Usage)
	}
	if result.Remaining == nil || *result.Remaining != 75 {
		t.Fatalf("expected result remaining 75, got %#v", result.Remaining)
	}
}

func TestValidatorQueriesZhipuCodingUsage(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer zhipu-token-value" {
			t.Fatalf("unexpected model Authorization header: %q", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/api/paas/v4/models" {
			t.Fatalf("unexpected validation path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer upstream.Close()

	quota := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "zhipu-token-value" {
			t.Fatalf("unexpected quota Authorization header: %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"success": true,
			"data": {
				"level": "Coding",
				"limits": [
					{"type": "TOKENS_LIMIT", "percentage": 25, "nextResetTime": 1760000000000}
				]
			}
		}`))
	}))
	defer quota.Close()

	originalURL := zhipuCodingPlanUsageURL
	zhipuCodingPlanUsageURL = quota.URL + "/api/monitor/usage/quota/limit"
	defer func() {
		zhipuCodingPlanUsageURL = originalURL
	}()

	validator, err := NewValidator(config.Config{
		ProxyPort:       3000,
		ControlPort:     3890,
		ZhipuBaseURL:    upstream.URL + "/api/paas/v4",
		SwitchThreshold: 15,
		MaxRetries:      1,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := validator.Validate(t.Context(), token.Token{
		Provider:   token.ProviderZhipu,
		TokenValue: "zhipu-token-value",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Usage == nil || result.Usage.PrimaryUsedPercent != 25 || result.Usage.PrimaryRemainingPercent != 75 {
		t.Fatalf("unexpected zhipu usage: %#v", result.Usage)
	}
	if result.Remaining == nil || *result.Remaining != 75 {
		t.Fatalf("expected remaining 75, got %#v", result.Remaining)
	}
}

func TestValidatorQueriesMiniMaxCodingUsage(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer minimax-token-value" {
			t.Fatalf("unexpected Authorization header: %q", r.Header.Get("Authorization"))
		}
		switch r.URL.Path {
		case "/v1/models":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[]}`))
		case "/v1/api/openplatform/coding_plan/remains":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"base_resp": {"status_code": 0, "status_msg": ""},
				"model_remains": [
					{
						"model": "MiniMax-M2.7",
						"current_interval_total_count": 100,
						"current_interval_usage_count": 80,
						"end_time": 1760000000000,
						"current_weekly_total_count": 1000,
						"current_weekly_usage_count": 600,
						"weekly_end_time": 1760500000000
					}
				]
			}`))
		default:
			t.Fatalf("unexpected validation path: %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	validator, err := NewValidator(config.Config{
		ProxyPort:       3000,
		ControlPort:     3890,
		MiniMaxBaseURL:  upstream.URL + "/v1",
		SwitchThreshold: 15,
		MaxRetries:      1,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := validator.Validate(t.Context(), token.Token{
		Provider:   token.ProviderMiniMax,
		TokenValue: "minimax-token-value",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Usage == nil || result.Usage.PrimaryRemainingPercent != 80 || result.Usage.SecondaryRemainingPercent != 60 {
		t.Fatalf("unexpected minimax usage: %#v", result.Usage)
	}
	if result.Remaining == nil || *result.Remaining != 80 {
		t.Fatalf("expected remaining 80, got %#v", result.Remaining)
	}
}

func TestValidatorUsesGeminiAPIKey(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") != "gemini-token-value" {
			t.Fatalf("unexpected x-goog-api-key header: %q", r.Header.Get("x-goog-api-key"))
		}
		if r.Header.Get("Authorization") != "" {
			t.Fatalf("expected no Authorization header, got %q", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/v1beta/models" {
			t.Fatalf("unexpected validation path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer upstream.Close()

	validator, err := NewValidator(config.Config{
		ProxyPort:       3000,
		ControlPort:     3890,
		GeminiBaseURL:   upstream.URL,
		SwitchThreshold: 15,
		MaxRetries:      1,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := validator.Validate(t.Context(), token.Token{
		Provider:   token.ProviderGemini,
		TokenValue: "gemini-token-value",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("expected gemini validation to pass: %#v", result)
	}
}

func TestValidatorQueriesMimoTokenPlanUsage(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("api-key") != "tp-mimo-token-plan" {
			t.Fatalf("unexpected api-key header: %q", r.Header.Get("api-key"))
		}
		switch r.URL.Path {
		case "/token-plan/v1/models":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[]}`))
		case "/platform/api/v1/tokenPlan/usage":
			if r.Header.Get("X-Timezone") != "Asia/Shanghai" {
				t.Fatalf("unexpected X-Timezone header: %q", r.Header.Get("X-Timezone"))
			}
			if r.Header.Get("Cookie") != "serviceToken=console-session; userId=123" {
				t.Fatalf("unexpected Cookie header: %q", r.Header.Get("Cookie"))
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"code": 0,
				"message": "",
				"data": {
					"monthUsage": {
						"percent": 0.8813,
						"items": [
							{"name": "month_total_token", "used": 52877400, "limit": 60000000, "percent": 0.8813}
						]
					},
					"usage": {
						"percent": 0.88,
						"items": [
							{"name": "plan_total_token", "used": 52877400, "limit": 60000000, "percent": 0.88},
							{"name": "compensation_total_token", "used": 0, "limit": 0, "percent": 0}
						]
					}
				}
			}`))
		case "/platform/api/v1/tokenPlan/detail":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"code": 0,
				"message": "",
				"data": {
					"planName": "Lite",
					"currentPeriodEnd": "2026-05-03 23:59:59",
					"expired": false
				}
			}`))
		default:
			t.Fatalf("unexpected validation path: %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	originalMimoPlatformBaseURL := mimoTokenPlanPlatformBaseURL
	mimoTokenPlanPlatformBaseURL = upstream.URL + "/platform/api/v1/tokenPlan"
	defer func() {
		mimoTokenPlanPlatformBaseURL = originalMimoPlatformBaseURL
	}()

	validator, err := NewValidator(config.Config{
		ProxyPort:              3000,
		ControlPort:            3890,
		XiaomiTokenPlanBaseURL: upstream.URL + "/token-plan/v1",
		XiaomiPlatformCookie:   "serviceToken=console-session; userId=123",
		SwitchThreshold:        15,
		MaxRetries:             1,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := validator.Validate(t.Context(), token.Token{
		Provider:       token.ProviderXiaomi,
		CredentialType: token.CredentialTypeMimoTokenPlan,
		TokenValue:     "tp-mimo-token-plan",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Usage == nil || !result.Usage.SubscriptionQuotaAvailable {
		t.Fatalf("expected MiMo token plan usage details, got %#v", result.Usage)
	}
	if result.Usage.PlanType != "MiMo Lite" {
		t.Fatalf("expected MiMo Lite plan type, got %q", result.Usage.PlanType)
	}
	if result.Usage.PrimaryUsedPercent != 88 || result.Usage.PrimaryRemainingPercent != 12 {
		t.Fatalf("unexpected primary usage: %#v", result.Usage)
	}
	if result.Usage.SecondaryUsedPercent != 88 || result.Usage.SecondaryRemainingPercent != 12 {
		t.Fatalf("unexpected secondary usage: %#v", result.Usage)
	}
	expectedReset := time.Date(2026, 5, 3, 23, 59, 59, 0, time.Local).Unix()
	if result.Usage.PrimaryResetAt != expectedReset || result.Usage.SecondaryResetAt != expectedReset {
		t.Fatalf("unexpected reset times: %#v", result.Usage)
	}
	if result.Remaining == nil || *result.Remaining != 12 {
		t.Fatalf("expected result remaining 12, got %#v", result.Remaining)
	}
}

func TestValidatorQueriesMimoBalance(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("api-key") != "sk-mimo-api-key" {
			t.Fatalf("unexpected api-key header: %q", r.Header.Get("api-key"))
		}
		switch r.URL.Path {
		case "/mimo/v1/models":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[]}`))
		case "/platform/api/v1/balance":
			if r.Header.Get("Cookie") != "serviceToken=console-session; userId=123" {
				t.Fatalf("unexpected Cookie header: %q", r.Header.Get("Cookie"))
			}
			if r.Header.Get("Referer") != "https://platform.xiaomimimo.com/console/balance" {
				t.Fatalf("unexpected Referer header: %q", r.Header.Get("Referer"))
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"code": 0,
				"message": "",
				"data": {
					"balance": "94.88",
					"frozenBalance": "0.00",
					"currency": "CNY",
					"overdraftLimit": "0.00",
					"remainingOverdraftLimit": "0.00",
					"giftBalance": "94.88",
					"cashBalance": "0.00"
				}
			}`))
		default:
			t.Fatalf("unexpected validation path: %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	originalMimoPlatformAPIBaseURL := mimoPlatformAPIBaseURL
	mimoPlatformAPIBaseURL = upstream.URL + "/platform/api/v1"
	defer func() {
		mimoPlatformAPIBaseURL = originalMimoPlatformAPIBaseURL
	}()

	validator, err := NewValidator(config.Config{
		ProxyPort:            3000,
		ControlPort:          3890,
		XiaomiAPIBaseURL:     upstream.URL + "/mimo/v1",
		XiaomiPlatformCookie: "serviceToken=console-session; userId=123",
		SwitchThreshold:      15,
		MaxRetries:           1,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := validator.Validate(t.Context(), token.Token{
		Provider:       token.ProviderXiaomi,
		CredentialType: token.CredentialTypeAPIKey,
		TokenValue:     "sk-mimo-api-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Usage == nil {
		t.Fatal("expected MiMo balance usage details")
	}
	if result.Usage.BalanceUnit != "CNY" || result.Usage.BalanceRemaining != 94.88 {
		t.Fatalf("unexpected balance usage: %#v", result.Usage)
	}
	if result.Remaining == nil || *result.Remaining != 100 {
		t.Fatalf("expected positive balance to map to remaining 100, got %#v", result.Remaining)
	}
}

func TestValidatorParsesCodexUsage(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer codex-access-token" {
			t.Fatalf("unexpected Authorization header: %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("ChatGPT-Account-Id") != "account-123" {
			t.Fatalf("unexpected account header: %q", r.Header.Get("ChatGPT-Account-Id"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"plan_type": "team",
			"rate_limit": {
				"limit_reached": false,
				"primary_window": {"used_percent": 27, "reset_at": 1760000000},
				"secondary_window": {"used_percent": 41, "reset_at": 1760500000}
			}
		}`))
	}))
	defer upstream.Close()

	validator, err := NewValidator(config.Config{
		ProxyPort:          3000,
		ControlPort:        3890,
		OpenAIBaseURL:      "https://api.openai.com",
		CodexUsageEndpoint: upstream.URL,
		SwitchThreshold:    15,
		MaxRetries:         1,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := validator.Validate(t.Context(), token.Token{
		Provider:       token.ProviderOpenAI,
		CredentialType: token.CredentialTypeCodexAuthJSON,
		TokenValue: `{
			"tokens": {
				"access_token": "codex-access-token",
				"account_id": "account-123",
				"id_token": "codex-id-token"
			}
		}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Usage == nil {
		t.Fatal("expected usage details")
	}
	if result.Usage.PlanType != "team" {
		t.Fatalf("expected team plan, got %q", result.Usage.PlanType)
	}
	if result.Usage.PrimaryRemainingPercent != 73 {
		t.Fatalf("expected 73 primary remaining, got %d", result.Usage.PrimaryRemainingPercent)
	}
	if result.Usage.SecondaryRemainingPercent != 59 {
		t.Fatalf("expected 59 secondary remaining, got %d", result.Usage.SecondaryRemainingPercent)
	}
	if result.Remaining == nil || *result.Remaining != 73 {
		t.Fatalf("expected result remaining 73, got %#v", result.Remaining)
	}
}
