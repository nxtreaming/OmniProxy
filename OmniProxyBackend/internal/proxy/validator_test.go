package proxy

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

func TestValidatorRoutesCodexUsageThroughOutboundProxy(t *testing.T) {
	upstreamHits := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits++
		_, _ = w.Write([]byte("direct"))
	}))
	defer upstream.Close()

	proxyHits := 0
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHits++
		gotURL := ""
		if r.URL != nil {
			gotURL = r.URL.String()
		}
		if r.URL == nil || r.URL.Scheme != "http" || r.URL.Host != strings.TrimPrefix(upstream.URL, "http://") || r.URL.Path != "/backend-api/wham/usage" {
			t.Fatalf("expected Codex usage absolute upstream URL through proxy, got %q", gotURL)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer codex-access-token" {
			t.Fatalf("unexpected Authorization header: %q", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"plan_type":"plus","rate_limit":{"primary_window":{"used_percent":25,"reset_at":1893456000}}}`))
	}))
	defer proxyServer.Close()

	validator, err := NewValidator(config.Config{
		ProxyPort:              3000,
		ControlPort:            3890,
		CodexUsageEndpoint:     upstream.URL + "/backend-api/wham/usage",
		OutboundProxyEnabled:   true,
		OutboundProxyURL:       proxyServer.URL,
		OutboundProxyProviders: []string{token.ProviderOpenAI},
		SwitchThreshold:        15,
		MaxRetries:             1,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := validator.Validate(t.Context(), token.Token{
		Provider:       token.ProviderOpenAI,
		CredentialType: token.CredentialTypeCodexAuthJSON,
		TokenValue:     `{"access_token":"codex-access-token","account_id":"account-123"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Remaining == nil || *result.Remaining != 75 {
		t.Fatalf("expected proxied Codex usage result with remaining 75, got %#v", result)
	}
	if proxyHits != 1 || upstreamHits != 0 {
		t.Fatalf("expected only proxy hit for Codex usage refresh, proxy=%d upstream=%d", proxyHits, upstreamHits)
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
					{"currency": "USD", "total_balance": "0.00"},
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
	if len(result.Usage.BalancePackages) != 2 {
		t.Fatalf("expected all currency balances to be preserved, got %#v", result.Usage.BalancePackages)
	}
	if result.Usage.BalancePackages[0].Unit != "USD" || result.Usage.BalancePackages[0].BalanceRemaining != 0 {
		t.Fatalf("unexpected USD balance package: %#v", result.Usage.BalancePackages[0])
	}
	if result.Usage.BalancePackages[1].Unit != "CNY" || result.Usage.BalancePackages[1].BalanceRemaining != 12.5 {
		t.Fatalf("unexpected CNY balance package: %#v", result.Usage.BalancePackages[1])
	}
	if result.Remaining == nil || *result.Remaining != 100 {
		t.Fatalf("expected positive balance to map to remaining 100, got %#v", result.Remaining)
	}
}

func TestDeepSeekBalanceFromInfosSelectsPositiveCurrency(t *testing.T) {
	tests := []struct {
		name        string
		infos       []any
		wantBalance float64
		wantUnit    string
	}{
		{
			name: "positive cny after zero usd",
			infos: []any{
				map[string]any{"currency": "USD", "total_balance": "0.00"},
				map[string]any{"currency": "CNY", "total_balance": "12.50"},
			},
			wantBalance: 12.5,
			wantUnit:    "CNY",
		},
		{
			name: "positive usd after zero cny",
			infos: []any{
				map[string]any{"currency": "CNY", "total_balance": "0.00"},
				map[string]any{"currency": "USD", "total_balance": "1.25"},
			},
			wantBalance: 1.25,
			wantUnit:    "USD",
		},
		{
			name: "deterministic cny preference when both are positive",
			infos: []any{
				map[string]any{"currency": "USD", "total_balance": "1.25"},
				map[string]any{"currency": "CNY", "total_balance": "12.50"},
			},
			wantBalance: 12.5,
			wantUnit:    "CNY",
		},
		{
			name: "fallback to granted plus topped up balance",
			infos: []any{
				map[string]any{"currency": "USD", "granted_balance": "0.10", "topped_up_balance": "0.20"},
			},
			wantBalance: 0.3,
			wantUnit:    "USD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			balance, unit, ok := deepSeekBalanceFromInfos(tt.infos)
			if !ok {
				t.Fatal("expected balance info")
			}
			if math.Abs(balance-tt.wantBalance) > 0.000001 || unit != tt.wantUnit {
				t.Fatalf("unexpected balance=%v unit=%q", balance, unit)
			}
		})
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
					{"type": "TOKENS_LIMIT", "percentage": 25, "nextResetTime": 1760000000000, "unit": 3, "number": 5},
					{"type": "TOKENS_LIMIT", "percentage": 100, "nextResetTime": 1760500000000, "unit": 6, "number": 7}
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
		Provider:       token.ProviderZhipu,
		CredentialType: token.CredentialTypeCodingPlan,
		TokenValue:     "zhipu-token-value",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Usage == nil || result.Usage.PrimaryUsedPercent != 25 || result.Usage.PrimaryRemainingPercent != 75 || result.Usage.SecondaryRemainingPercent != 0 {
		t.Fatalf("unexpected zhipu usage: %#v", result.Usage)
	}
	if !result.Usage.LimitReached {
		t.Fatalf("expected weekly limit to mark usage exhausted: %#v", result.Usage)
	}
	if result.Remaining == nil || *result.Remaining != 0 {
		t.Fatalf("expected remaining 0, got %#v", result.Remaining)
	}
}

func TestValidatorQueriesZhipuAPIBalance(t *testing.T) {
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

	balance := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer zhipu-token-value" {
			t.Fatalf("unexpected balance Authorization header: %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"success": true,
			"rows": [
				{"tokenNo": "general", "tokenBalance": 1200},
				{"tokenNo": "glm-4.6", "tokenBalance": "300.5"}
			]
		}`))
	}))
	defer balance.Close()

	originalURL := zhipuAPIBalanceURL
	zhipuAPIBalanceURL = balance.URL + "/api/biz/tokenAccounts/list/my"
	defer func() {
		zhipuAPIBalanceURL = originalURL
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
		Provider:       token.ProviderZhipu,
		CredentialType: token.CredentialTypeAPIKey,
		TokenValue:     "zhipu-token-value",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Usage == nil || result.Usage.BalanceRemaining != 1500.5 || result.Usage.BalanceUnit != "Token" {
		t.Fatalf("unexpected zhipu api balance usage: %#v", result.Usage)
	}
	if result.Remaining == nil || *result.Remaining != 100 {
		t.Fatalf("expected remaining 100, got %#v", result.Remaining)
	}
}

func TestValidatorQueriesZhipuAPIBalanceCountsOnlyTokenPackages(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/paas/v4/models" {
			t.Fatalf("unexpected validation path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer upstream.Close()

	balance := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"total": 5,
			"rows": [
				{"resourcePackageName": "image pack", "availableBalance": 20, "tokenBalance": 20, "consumeType": "TIMES", "status": "EFFECTIVE"},
				{"resourcePackageName": "search pack", "availableBalance": 100, "tokenBalance": 100, "consumeType": "TIMES", "status": "EFFECTIVE"},
				{"resourcePackageName": "general token pack", "availableBalance": 1833309, "tokenBalance": 2000000, "consumeType": "TOKENS", "status": "EFFECTIVE"},
				{"resourcePackageName": "vision token pack", "availableBalance": 6000000, "tokenBalance": 6000000, "consumeType": "TOKENS", "status": "EFFECTIVE"},
				{"resourcePackageName": "expired token pack", "availableBalance": 12000000, "tokenBalance": 12000000, "consumeType": "TOKENS", "status": "EXPIRED"}
			],
			"code": 200,
			"msg": "查询成功"
		}`))
	}))
	defer balance.Close()

	originalURL := zhipuAPIBalanceURL
	zhipuAPIBalanceURL = balance.URL + "/api/biz/tokenAccounts/list/my"
	defer func() {
		zhipuAPIBalanceURL = originalURL
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
		Provider:       token.ProviderZhipu,
		CredentialType: token.CredentialTypeAPIKey,
		TokenValue:     "zhipu-token-value",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Usage == nil || result.Usage.BalanceRemaining != 7833309 || result.Usage.BalanceUnit != "Token" {
		t.Fatalf("unexpected zhipu token package balance: %#v", result.Usage)
	}
	if len(result.Usage.BalancePackages) != 5 {
		t.Fatalf("expected all zhipu balance packages to be preserved, got %#v", result.Usage.BalancePackages)
	}
	if result.Usage.BalancePackages[0].Name != "image pack" ||
		result.Usage.BalancePackages[0].ConsumeType != "TIMES" ||
		result.Usage.BalancePackages[0].Unit != "次" {
		t.Fatalf("unexpected times package detail: %#v", result.Usage.BalancePackages[0])
	}
	if result.Usage.BalancePackages[2].BalanceRemaining != 1833309 ||
		result.Usage.BalancePackages[2].BalanceTotal != 2000000 ||
		result.Usage.BalancePackages[2].Unit != "Token" {
		t.Fatalf("unexpected token package detail: %#v", result.Usage.BalancePackages[2])
	}
	if result.Remaining == nil || *result.Remaining != 100 {
		t.Fatalf("expected remaining 100, got %#v", result.Remaining)
	}
}

func TestValidatorQueriesZhipuAPIBalanceTreatsEmptyRowsAsZero(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/paas/v4/models" {
			t.Fatalf("unexpected validation path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer upstream.Close()

	balance := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"total":0,"rows":[],"code":200,"msg":"查询成功"}`))
	}))
	defer balance.Close()

	originalURL := zhipuAPIBalanceURL
	zhipuAPIBalanceURL = balance.URL + "/api/biz/tokenAccounts/list/my"
	defer func() {
		zhipuAPIBalanceURL = originalURL
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
		Provider:       token.ProviderZhipu,
		CredentialType: token.CredentialTypeAPIKey,
		TokenValue:     "zhipu-token-value",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Usage == nil || result.Usage.BalanceRemaining != 0 || !result.Usage.LimitReached {
		t.Fatalf("unexpected empty zhipu api balance usage: %#v", result.Usage)
	}
	if result.Remaining == nil || *result.Remaining != 0 {
		t.Fatalf("expected remaining 0, got %#v", result.Remaining)
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

func TestValidatorQueriesOpenRouterKeyUsage(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/key" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer openrouter-api-key" {
			t.Fatalf("unexpected Authorization header: %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"usage":12.5,"limit":50,"limit_remaining":37.5}}`))
	}))
	defer upstream.Close()

	validator, err := NewValidator(config.Config{
		OpenRouterBaseURL: upstream.URL + "/api/v1",
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := validator.Validate(context.Background(), token.Token{
		Provider:       token.ProviderOpenRouter,
		CredentialType: token.CredentialTypeAPIKey,
		TokenValue:     "openrouter-api-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Usage == nil || result.Remaining == nil {
		t.Fatalf("expected openrouter validation usage, got %#v", result)
	}
	if *result.Remaining != 75 {
		t.Fatalf("expected 75 percent remaining, got %d", *result.Remaining)
	}
	if result.Usage.BalanceRemaining != 37.5 || result.Usage.BalanceTotal != 50 || result.Usage.BalanceUsed != 12.5 || result.Usage.BalanceUnit != "USD" {
		t.Fatalf("unexpected openrouter usage: %#v", result.Usage)
	}
}

func TestValidatorQueriesOpenRouterUnlimitedKeyUsage(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/key" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"usage":0.017,"limit":null,"limit_remaining":null,"is_free_tier":true}}`))
	}))
	defer upstream.Close()

	validator, err := NewValidator(config.Config{
		OpenRouterBaseURL: upstream.URL + "/api/v1",
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := validator.Validate(context.Background(), token.Token{
		Provider:       token.ProviderOpenRouter,
		CredentialType: token.CredentialTypeAPIKey,
		TokenValue:     "openrouter-api-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Usage == nil || result.Remaining == nil {
		t.Fatalf("expected openrouter validation usage, got %#v", result)
	}
	if *result.Remaining != 100 {
		t.Fatalf("expected unlimited key to remain active, got %d", *result.Remaining)
	}
	if !result.Usage.BalanceUnlimited || result.Usage.BalanceUnit != "USD" || result.Usage.BalanceUsed != 0.017 {
		t.Fatalf("unexpected unlimited openrouter usage: %#v", result.Usage)
	}
	if result.Usage.PlanType != "OpenRouter Free Tier" {
		t.Fatalf("unexpected plan type: %q", result.Usage.PlanType)
	}
}

func TestValidatorUsesTokenRouterRoutingRules(t *testing.T) {
	tests := []struct {
		name    string
		baseURL func(string) string
	}{
		{
			name:    "base without version",
			baseURL: func(serverURL string) string { return serverURL },
		},
		{
			name:    "base with version",
			baseURL: func(serverURL string) string { return serverURL + "/v1" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/routing-rules" {
					t.Fatalf("unexpected path: %s", r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer tr_test_token" {
					t.Fatalf("unexpected Authorization header: %q", r.Header.Get("Authorization"))
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"rules":[]}`))
			}))
			defer upstream.Close()

			validator, err := NewValidator(config.Config{
				TokenRouterBaseURL: tt.baseURL(upstream.URL),
			})
			if err != nil {
				t.Fatal(err)
			}

			result, err := validator.Validate(context.Background(), token.Token{
				Provider:       token.ProviderTokenRouter,
				CredentialType: token.CredentialTypeAPIKey,
				TokenValue:     "tr_test_token",
			})
			if err != nil {
				t.Fatal(err)
			}
			if !result.OK {
				t.Fatalf("expected tokenrouter validation to pass: %#v", result)
			}
		})
	}
}

func TestValidatorUsesZoModelsAvailable(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models/available" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer zo_sk_test_token" {
			t.Fatalf("unexpected Authorization header: %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer upstream.Close()

	validator, err := NewValidator(config.Config{
		ZoBaseURL: upstream.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := validator.Validate(context.Background(), token.Token{
		Provider:       token.ProviderZo,
		CredentialType: token.CredentialTypeAPIKey,
		TokenValue:     "zo_sk_test_token",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("expected zo validation to pass: %#v", result)
	}
}

func TestValidatorUsesSub2APIUsageEndpoint(t *testing.T) {
	tests := []struct {
		name    string
		baseURL func(string) string
	}{
		{
			name:    "base without version",
			baseURL: func(serverURL string) string { return serverURL },
		},
		{
			name:    "base with version",
			baseURL: func(serverURL string) string { return serverURL + "/v1" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/usage" {
					t.Fatalf("unexpected path: %s", r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer sub2api-api-key-token" {
					t.Fatalf("unexpected Authorization header: %q", r.Header.Get("Authorization"))
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{
					"mode": "quota_limited",
					"isValid": true,
					"status": "active",
					"quota": {"limit": 100, "used": 12.5, "remaining": 87.5, "unit": "USD"},
					"usage": {
						"today": {"requests": 2, "total_tokens": 10, "actual_cost": 0.12},
						"total": {"requests": 9, "total_tokens": 80, "actual_cost": 1.25}
					},
					"model_stats": []
				}`))
			}))
			defer upstream.Close()

			validator, err := NewValidator(config.Config{
				Sub2APIBaseURL: tt.baseURL(upstream.URL),
			})
			if err != nil {
				t.Fatal(err)
			}

			result, err := validator.Validate(context.Background(), token.Token{
				Provider:       token.ProviderSub2API,
				CredentialType: token.CredentialTypeAPIKey,
				BaseURL:        tt.baseURL(upstream.URL),
				TokenValue:     "sub2api-api-key-token",
			})
			if err != nil {
				t.Fatal(err)
			}
			if !result.OK {
				t.Fatalf("expected sub2api validation to pass: %#v", result)
			}
			if result.Usage == nil {
				t.Fatal("expected sub2api usage details")
			}
			if result.Usage.Source != token.ProviderSub2API || result.Usage.BalanceUnit != "USD" {
				t.Fatalf("unexpected sub2api usage source/unit: %#v", result.Usage)
			}
			if result.Usage.BalanceTotal != 100 || result.Usage.BalanceUsed != 12.5 || result.Usage.BalanceRemaining != 87.5 {
				t.Fatalf("unexpected sub2api quota usage: %#v", result.Usage)
			}
			if result.Remaining == nil || *result.Remaining != 88 {
				t.Fatalf("expected result remaining 88, got %#v", result.Remaining)
			}
		})
	}
}

func TestValidatorParsesSub2APIWalletUsage(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/usage" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"mode": "unrestricted",
			"isValid": true,
			"planName": "Wallet",
			"remaining": 23.4,
			"unit": "USD",
			"balance": 23.4
		}`))
	}))
	defer upstream.Close()

	validator, err := NewValidator(config.Config{
		Sub2APIBaseURL: upstream.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := validator.Validate(context.Background(), token.Token{
		Provider:       token.ProviderSub2API,
		CredentialType: token.CredentialTypeAPIKey,
		BaseURL:        upstream.URL,
		TokenValue:     "sub2api-api-key-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Usage == nil || result.Usage.BalanceRemaining != 23.4 || result.Usage.BalanceUnit != "USD" {
		t.Fatalf("unexpected sub2api wallet usage: %#v", result.Usage)
	}
	if result.Remaining == nil || *result.Remaining != 100 {
		t.Fatalf("expected wallet balance to map to remaining 100, got %#v", result.Remaining)
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

func TestValidatorParsesCodexFreeUsageWithoutPrimaryWindow(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer codex-access-token" {
			t.Fatalf("unexpected Authorization header: %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"plan_type": "free",
			"rate_limit": {
				"limit_reached": false,
				"secondary_window": {"used_percent": 18, "reset_at": 1760500000}
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
	if result.Usage.PrimaryRemainingPercent != 0 {
		t.Fatalf("expected no primary quota window, got %#v", result.Usage)
	}
	if result.Usage.SecondaryRemainingPercent != 82 {
		t.Fatalf("expected 82 secondary remaining, got %d", result.Usage.SecondaryRemainingPercent)
	}
	if result.Remaining == nil || *result.Remaining != 82 {
		t.Fatalf("expected result remaining 82 from weekly quota, got %#v", result.Remaining)
	}
}

func TestValidatorMapsCodexFreePrimaryWindowToWeeklyQuota(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"plan_type": "free",
			"rate_limit": {
				"limit_reached": false,
				"primary_window": {"used_percent": 3, "reset_at": 1779000000}
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
		TokenValue:     `{"tokens":{"access_token":"codex-access-token"}}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Usage == nil {
		t.Fatal("expected usage details")
	}
	if result.Usage.PrimaryRemainingPercent != 0 || result.Usage.PrimaryResetAt != 0 {
		t.Fatalf("expected free plan to avoid 5h quota fields, got %#v", result.Usage)
	}
	if result.Usage.SecondaryRemainingPercent != 97 || result.Usage.SecondaryResetAt != 1779000000 {
		t.Fatalf("expected free primary window to map to weekly quota, got %#v", result.Usage)
	}
	if result.Remaining == nil || *result.Remaining != 97 {
		t.Fatalf("expected result remaining 97 from weekly quota, got %#v", result.Remaining)
	}
}
