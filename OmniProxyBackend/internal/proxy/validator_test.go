package proxy

import (
	"net/http"
	"net/http/httptest"
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
