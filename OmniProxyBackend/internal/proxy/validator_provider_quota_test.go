package proxy

import (
	"net/http"
	"net/http/httptest"
	"omniproxy/internal/config"
	"omniproxy/internal/token"
	"testing"
)

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
