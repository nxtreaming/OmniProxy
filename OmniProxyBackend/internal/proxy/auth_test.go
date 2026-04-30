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

func TestApplyAuthClearsIncomingCodexAccountID(t *testing.T) {
	header := http.Header{}
	header.Set("Authorization", "Bearer caller-token")
	header.Set("ChatGPT-Account-Id", "caller-account")
	selected := token.Token{
		Provider:       token.ProviderOpenAI,
		CredentialType: token.CredentialTypeCodexAuthJSON,
		TokenValue: `{
			"auth_mode": "chatgpt",
			"tokens": {
				"access_token": "selected-access-token",
				"id_token": "selected-id-token"
			}
		}`,
	}

	if err := applyAuth(header, selected); err != nil {
		t.Fatal(err)
	}
	if got := header.Get("Authorization"); got != "Bearer selected-access-token" {
		t.Fatalf("unexpected Authorization header: %q", got)
	}
	if got := header.Get("ChatGPT-Account-Id"); got != "" {
		t.Fatalf("incoming ChatGPT-Account-Id should be cleared, got %q", got)
	}
}

func TestApplyAuthUsesTopLevelCodexAccountID(t *testing.T) {
	header := http.Header{}
	selected := token.Token{
		Provider:       token.ProviderOpenAI,
		CredentialType: token.CredentialTypeCodexAuthJSON,
		TokenValue: `{
			"auth_mode": "chatgpt",
			"account_id": "top-level-account",
			"tokens": {
				"access_token": "selected-access-token",
				"id_token": "selected-id-token"
			}
		}`,
	}

	if err := applyAuth(header, selected); err != nil {
		t.Fatal(err)
	}
	if got := header.Get("ChatGPT-Account-Id"); got != "top-level-account" {
		t.Fatalf("unexpected ChatGPT-Account-Id header: %q", got)
	}
}
