package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"omniproxy/internal/config"
	"omniproxy/internal/token"
)

func TestCodexResetCreditsFetchAndConsume(t *testing.T) {
	const redeemRequestID = "4ea571da-8102-4e72-b4a9-f1591f0b3d31"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer access-secret" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("ChatGPT-Account-Id"); got != "acct-123" {
			t.Errorf("ChatGPT-Account-Id = %q", got)
		}
		if got := r.Header.Get("OpenAI-Beta"); got != "codex-1" {
			t.Errorf("OpenAI-Beta = %q", got)
		}
		if got := r.Header.Get("Originator"); got != "Codex Desktop" {
			t.Errorf("Originator = %q", got)
		}

		switch r.URL.Path {
		case "/backend-api/wham/rate-limit-reset-credits":
			if r.Method != http.MethodGet {
				t.Errorf("fetch method = %s", r.Method)
			}
			_, _ = w.Write([]byte(`{
                    "available_count": 1,
                    "credits": [
                      {"id":"available-1","status":"available","reset_type":"codex_rate_limits","granted_at":"2029-12-01T00:00:00Z","expires_at":"2030-01-01T00:00:00Z"},
                      {"id":"used-1","status":"redeemed","reset_type":"codex_rate_limits","granted_at":1893456000000,"expires_at":1896134400000,"redeemed_at":"2029-12-15T12:00:00Z"}
                    ]
                }`))
		case "/backend-api/wham/rate-limit-reset-credits/consume":
			if r.Method != http.MethodPost {
				t.Errorf("consume method = %s", r.Method)
			}
			var payload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode consume body: %v", err)
			}
			if got := payload["redeem_request_id"]; got != redeemRequestID {
				t.Errorf("redeem_request_id = %q", got)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := config.Default()
	cfg.CodexUsageEndpoint = server.URL + "/backend-api/wham/usage"
	validator, err := NewValidator(cfg)
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	selected := token.Token{
		Provider:       token.ProviderOpenAI,
		CredentialType: token.CredentialTypeCodexAuthJSON,
		TokenValue:     `{"tokens":{"access_token":"access-secret","account_id":"acct-123"}}`,
	}

	snapshot, err := validator.FetchCodexResetCredits(context.Background(), selected)
	if err != nil {
		t.Fatalf("FetchCodexResetCredits: %v", err)
	}
	if snapshot.AvailableCount == nil || *snapshot.AvailableCount != 1 {
		t.Fatalf("AvailableCount = %v", snapshot.AvailableCount)
	}
	if len(snapshot.Credits) != 2 {
		t.Fatalf("credits = %d", len(snapshot.Credits))
	}
	if snapshot.Credits[0].GrantedAt != 1890777600 || snapshot.Credits[0].ExpiresAt != 1893456000 {
		t.Fatalf("available timestamps = %+v", snapshot.Credits[0])
	}
	if snapshot.Credits[1].RedeemedAt != 1892030400 {
		t.Fatalf("redeemed timestamp = %d", snapshot.Credits[1].RedeemedAt)
	}
	if snapshot.NextExpiresAt != 1893456000 {
		t.Fatalf("NextExpiresAt = %d", snapshot.NextExpiresAt)
	}

	if err := validator.ConsumeCodexResetCredit(context.Background(), selected, redeemRequestID); err != nil {
		t.Fatalf("ConsumeCodexResetCredit: %v", err)
	}
}

func TestParseCodexResetCreditsSupportsNestedCamelCasePayload(t *testing.T) {
	snapshot, err := parseCodexResetCredits([]byte(`{
        "data": {
          "availableCount": "1",
          "credits": [
            {"creditId":"credit-1","state":"available","resetType":"codex_rate_limits","expiresAt":"2030-02-01T00:00:00Z"}
          ]
        }
    }`))
	if err != nil {
		t.Fatalf("parseCodexResetCredits: %v", err)
	}
	if snapshot.AvailableCount == nil || *snapshot.AvailableCount != 1 {
		t.Fatalf("AvailableCount = %v", snapshot.AvailableCount)
	}
	if len(snapshot.Credits) != 1 || snapshot.Credits[0].ID != "credit-1" {
		t.Fatalf("credits = %+v", snapshot.Credits)
	}
}

func TestParseCodexUsageIncludesResetCreditsSummary(t *testing.T) {
	usage, ok := parseCodexUsage([]byte(`{
        "plan_type":"plus",
        "rate_limit":{"primary_window":{"used_percent":25,"reset_at":1893456000}},
        "rate_limit_reset_credits":{"available_count":2}
    }`))
	if !ok {
		t.Fatal("parseCodexUsage returned false")
	}
	if usage.CodexResetCreditsAvailable == nil || *usage.CodexResetCreditsAvailable != 2 {
		t.Fatalf("CodexResetCreditsAvailable = %v", usage.CodexResetCreditsAvailable)
	}
}

func TestCodexResetCreditsHTTPErrorPreservesAuthStatus(t *testing.T) {
	err := codexResetCreditsStatusError(http.StatusUnauthorized, []byte(`{"detail":{"code":"token_expired"}}`))
	if !CodexResetCreditsAuthFailed(err) {
		t.Fatalf("expected auth failure, got %v", err)
	}
	if err.Error() != "Codex 额度刷新卡接口返回 401：token_expired" {
		t.Fatalf("error = %q", err.Error())
	}
}
