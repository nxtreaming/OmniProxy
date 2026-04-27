package proxy

import (
	"net/http"
	"testing"

	"OmniProxyBackend/internal/token"
)

func TestApplyAuthUsesCodexAuthJSONAccessTokenAndAccountID(t *testing.T) {
	header := http.Header{}
	selected := token.Token{
		Provider:       token.ProviderOpenAI,
		CredentialType: token.CredentialTypeCodexAuthJSON,
		TokenValue: `{
			"auth_mode": "chatgpt",
			"OPENAI_API_KEY": null,
			"tokens": {
				"access_token": "codex-access-token",
				"account_id": "account-123",
				"id_token": "codex-id-token"
			},
			"last_refresh": "2026-04-27T00:00:00Z"
		}`,
	}

	if err := applyAuth(header, selected); err != nil {
		t.Fatal(err)
	}
	if got := header.Get("Authorization"); got != "Bearer codex-access-token" {
		t.Fatalf("unexpected Authorization header: %q", got)
	}
	if got := header.Get("ChatGPT-Account-Id"); got != "account-123" {
		t.Fatalf("unexpected ChatGPT-Account-Id header: %q", got)
	}
}
