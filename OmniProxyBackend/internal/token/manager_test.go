package token

import (
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"testing"

	"OmniProxyBackend/internal/storage"
)

func TestManagerAddPrependsAndRejectsDuplicateNames(t *testing.T) {
	manager, err := NewManager(storage.NewJSONStore[[]Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}

	first, err := manager.Add(UpsertRequest{Name: "primary", Provider: "openai", TokenValue: "sk-primary-token"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.Add(UpsertRequest{Name: "backup", Provider: "openai", TokenValue: "sk-backup-token"})
	if err != nil {
		t.Fatal(err)
	}

	items := manager.List()
	if len(items) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(items))
	}
	if items[0].ID != second.ID || items[1].ID != first.ID {
		t.Fatalf("new token was not prepended: %#v", items)
	}

	if _, err := manager.Add(UpsertRequest{Name: "PRIMARY", Provider: "openai", TokenValue: "sk-another-token"}); err != ErrDuplicateName {
		t.Fatalf("expected duplicate name error, got %v", err)
	}
}

func TestManagerAcquireFallsBackToLowTokens(t *testing.T) {
	manager, err := NewManager(storage.NewJSONStore[[]Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}

	active, err := manager.Add(UpsertRequest{Name: "active", Provider: "openai", TokenValue: "sk-active-token"})
	if err != nil {
		t.Fatal(err)
	}
	low, err := manager.Add(UpsertRequest{Name: "low", Provider: "openai", TokenValue: "sk-low-token"})
	if err != nil {
		t.Fatal(err)
	}

	if err := manager.RecordUsage(low.ID, 10); err != nil {
		t.Fatal(err)
	}
	selected, err := manager.Acquire(ProviderOpenAI, map[string]bool{active.ID: true})
	if err != nil {
		t.Fatal(err)
	}
	if selected.ID != low.ID {
		t.Fatalf("expected low token fallback, got %s", selected.ID)
	}
}

func TestManagerAcquireBalancedRotatesAcrossAccounts(t *testing.T) {
	manager, err := NewManager(storage.NewJSONStore[[]Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}

	first, err := manager.Add(UpsertRequest{Name: "first", Provider: "openai", TokenValue: "sk-first-token"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.Add(UpsertRequest{Name: "second", Provider: "openai", TokenValue: "sk-second-token"})
	if err != nil {
		t.Fatal(err)
	}

	selected, err := manager.AcquireBalancedMatching(ProviderOpenAI, CredentialTypeAPIKey, nil)
	if err != nil {
		t.Fatal(err)
	}
	if selected.ID != first.ID {
		t.Fatalf("expected older unused token first, got %s", selected.Name)
	}
	if err := manager.RecordProxyUsage(selected.ID, TokenConsumption{TotalTokens: 1}); err != nil {
		t.Fatal(err)
	}

	selected, err = manager.AcquireBalancedMatching(ProviderOpenAI, CredentialTypeAPIKey, nil)
	if err != nil {
		t.Fatal(err)
	}
	if selected.ID != second.ID {
		t.Fatalf("expected next unused token after one task, got %s", selected.Name)
	}
}

func TestManagerAcquireBalancedPrefersHigherRemainingQuota(t *testing.T) {
	manager, err := NewManager(storage.NewJSONStore[[]Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}

	lower, err := manager.Add(UpsertRequest{Name: "lower", Provider: "openai", TokenValue: "sk-lower-token"})
	if err != nil {
		t.Fatal(err)
	}
	higher, err := manager.Add(UpsertRequest{Name: "higher", Provider: "openai", TokenValue: "sk-higher-token"})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.RecordUsage(lower.ID, 40); err != nil {
		t.Fatal(err)
	}
	if err := manager.RecordUsage(higher.ID, 80); err != nil {
		t.Fatal(err)
	}

	selected, err := manager.AcquireBalancedMatching(ProviderOpenAI, CredentialTypeAPIKey, nil)
	if err != nil {
		t.Fatal(err)
	}
	if selected.ID != higher.ID {
		t.Fatalf("expected higher remaining token, got %s", selected.Name)
	}
}

func TestManagerAllowsSameNameAcrossProviders(t *testing.T) {
	manager, err := NewManager(storage.NewJSONStore[[]Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := manager.Add(UpsertRequest{Name: "work", Provider: ProviderOpenAI, TokenValue: "sk-openai-token"}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(UpsertRequest{Name: "work", Provider: ProviderAnthropic, TokenValue: "sk-anthropic-token"}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(UpsertRequest{Name: "work", Provider: ProviderKimi, TokenValue: "sk-kimi-token"}); err != nil {
		t.Fatal(err)
	}
}

func TestManagerValidatesXiaomiCredentialFormats(t *testing.T) {
	manager, err := NewManager(storage.NewJSONStore[[]Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := manager.Add(UpsertRequest{Name: "paygo", Provider: ProviderXiaomi, TokenValue: "sk-xiaomi-paygo"}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(UpsertRequest{
		Name:           "plan",
		Provider:       ProviderXiaomi,
		CredentialType: CredentialTypeMimoTokenPlan,
		TokenValue:     "tp-xiaomi-token-plan",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(UpsertRequest{Name: "bad-paygo", Provider: ProviderXiaomi, TokenValue: "tp-wrong-kind"}); err == nil {
		t.Fatal("expected xiaomi pay-as-you-go key format error")
	}
	if _, err := manager.Add(UpsertRequest{
		Name:           "bad-plan",
		Provider:       ProviderXiaomi,
		CredentialType: CredentialTypeMimoTokenPlan,
		TokenValue:     "sk-wrong-kind",
	}); err == nil {
		t.Fatal("expected xiaomi token plan key format error")
	}
}

func TestManagerUsesCodexEmailAsName(t *testing.T) {
	manager, err := NewManager(storage.NewJSONStore[[]Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}

	authJSON := codexAuthJSONForTest(t, "coder@example.com")
	item, err := manager.Add(UpsertRequest{
		Name:           "",
		Provider:       ProviderOpenAI,
		CredentialType: CredentialTypeCodexAuthJSON,
		TokenValue:     authJSON,
	})
	if err != nil {
		t.Fatal(err)
	}
	if item.Name != "coder@example.com" {
		t.Fatalf("expected email as name, got %q", item.Name)
	}
}

func TestManagerRecordsProxyUsageTotalsAndDailyStats(t *testing.T) {
	manager, err := NewManager(storage.NewJSONStore[[]Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	item, err := manager.Add(UpsertRequest{Name: "metered", Provider: ProviderOpenAI, TokenValue: "sk-metered-token"})
	if err != nil {
		t.Fatal(err)
	}

	if err := manager.RecordProxyUsage(item.ID, TokenConsumption{InputTokens: 100, OutputTokens: 40, TotalTokens: 140}); err != nil {
		t.Fatal(err)
	}
	if err := manager.RecordProxyUsage(item.ID, TokenConsumption{InputTokens: 10, OutputTokens: 5}); err != nil {
		t.Fatal(err)
	}

	updated, err := manager.Get(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Stats.RequestCount != 2 {
		t.Fatalf("expected 2 requests, got %d", updated.Stats.RequestCount)
	}
	if updated.Stats.TotalTokens != 155 {
		t.Fatalf("expected 155 total tokens, got %d", updated.Stats.TotalTokens)
	}
	if updated.Stats.InputTokens != 110 || updated.Stats.OutputTokens != 45 {
		t.Fatalf("unexpected input/output tokens: %#v", updated.Stats)
	}
	if len(updated.Stats.Daily) != 1 || updated.Stats.Daily[0].TotalTokens != 155 {
		t.Fatalf("unexpected daily stats: %#v", updated.Stats.Daily)
	}
}

func codexAuthJSONForTest(t *testing.T, email string) string {
	t.Helper()

	payload, err := json.Marshal(map[string]any{
		"https://api.openai.com/profile": map[string]any{
			"email": email,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	jwt := "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
	data, err := json.Marshal(map[string]any{
		"auth_mode": "chatgpt",
		"tokens": map[string]any{
			"id_token":      jwt,
			"access_token":  "codex-access-token",
			"refresh_token": "codex-refresh-token",
			"account_id":    "account-123",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
