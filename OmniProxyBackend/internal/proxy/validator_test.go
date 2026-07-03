package proxy

import (
	"math"
	"net/http"
	"net/http/httptest"
	"omniproxy/internal/config"
	"omniproxy/internal/token"
	"strings"
	"testing"
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
