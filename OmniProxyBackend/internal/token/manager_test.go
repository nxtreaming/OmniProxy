package token

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestManagerAcquireAvoidsInFlightQueueToken(t *testing.T) {
	manager, err := NewManager(storage.NewJSONStore[[]Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}

	backup, err := manager.Add(UpsertRequest{Name: "backup", Provider: "openai", TokenValue: "sk-backup-token"})
	if err != nil {
		t.Fatal(err)
	}
	primary, err := manager.Add(UpsertRequest{Name: "primary", Provider: "openai", TokenValue: "sk-primary-token"})
	if err != nil {
		t.Fatal(err)
	}

	selected, err := manager.Acquire(ProviderOpenAI, nil)
	if err != nil {
		t.Fatal(err)
	}
	if selected.ID != primary.ID {
		t.Fatalf("expected primary token first, got %s", selected.Name)
	}

	selected, err = manager.Acquire(ProviderOpenAI, nil)
	if err != nil {
		t.Fatal(err)
	}
	if selected.ID != backup.ID {
		t.Fatalf("expected busy primary to be skipped, got %s", selected.Name)
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
	if _, err := manager.Add(UpsertRequest{Name: "work", Provider: ProviderZhipu, TokenValue: "zhipu-api-key-token"}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(UpsertRequest{Name: "work", Provider: ProviderMiniMax, TokenValue: "minimax-api-key-token"}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(UpsertRequest{Name: "work", Provider: ProviderGemini, TokenValue: "gemini-api-key-token"}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(UpsertRequest{Name: "work", Provider: ProviderCustom, TokenValue: "custom-api-key-token"}); err != nil {
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

func TestManagerUpdatePreservesTokenValueWhenBlank(t *testing.T) {
	manager, err := NewManager(storage.NewJSONStore[[]Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	item, err := manager.Add(UpsertRequest{Name: "primary", Provider: ProviderOpenAI, TokenValue: "sk-primary-token"})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := manager.Update(item.ID, UpsertRequest{Name: "renamed", Provider: ProviderOpenAI, CredentialType: CredentialTypeAPIKey})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "renamed" {
		t.Fatalf("expected updated name, got %q", updated.Name)
	}
	if updated.TokenValue != "sk-primary-token" {
		t.Fatalf("expected token value to be preserved, got %q", updated.TokenValue)
	}
}

func TestManagerUpdateRequiresTokenValueWhenCredentialTypeChanges(t *testing.T) {
	manager, err := NewManager(storage.NewJSONStore[[]Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	item, err := manager.Add(UpsertRequest{Name: "primary", Provider: ProviderOpenAI, TokenValue: "sk-primary-token"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = manager.Update(item.ID, UpsertRequest{Name: "primary", Provider: ProviderOpenAI, CredentialType: CredentialTypeCodexAuthJSON})
	if err == nil {
		t.Fatal("expected credential type change without token value to fail")
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

func TestManagerRecordsBalanceUsageStatus(t *testing.T) {
	manager, err := NewManager(storage.NewJSONStore[[]Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	item, err := manager.Add(UpsertRequest{Name: "deepseek", Provider: ProviderDeepSeek, TokenValue: "sk-deepseek-token"})
	if err != nil {
		t.Fatal(err)
	}

	if err := manager.RecordUsageInfo(item.ID, UsageInfo{
		Source:           ProviderDeepSeek,
		BalanceRemaining: 0,
		BalanceUnit:      "CNY",
		Message:          "DeepSeek balance",
	}); err != nil {
		t.Fatal(err)
	}
	updated, err := manager.Get(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != StatusExhausted || updated.Remaining != 0 {
		t.Fatalf("expected zero balance to exhaust token, got status=%s remaining=%d", updated.Status, updated.Remaining)
	}

	if err := manager.RecordUsageInfo(item.ID, UsageInfo{
		Source:           ProviderDeepSeek,
		BalanceRemaining: 12.5,
		BalanceUnit:      "CNY",
		Message:          "DeepSeek balance",
	}); err != nil {
		t.Fatal(err)
	}
	updated, err = manager.Get(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != StatusActive || updated.Remaining != 100 {
		t.Fatalf("expected positive balance to restore active token, got status=%s remaining=%d", updated.Status, updated.Remaining)
	}
}

func TestManagerBatchesUsagePersistenceUntilFlush(t *testing.T) {
	store := &countingTokenStore{}
	manager, err := NewManager(store, 15)
	if err != nil {
		t.Fatal(err)
	}
	manager.persistDelay = time.Hour

	item, err := manager.Add(UpsertRequest{Name: "metered", Provider: ProviderOpenAI, TokenValue: "sk-metered-token"})
	if err != nil {
		t.Fatal(err)
	}
	if store.saves != 1 {
		t.Fatalf("expected add to persist immediately, got %d saves", store.saves)
	}

	if err := manager.RecordProxyUsage(item.ID, TokenConsumption{TotalTokens: 10}); err != nil {
		t.Fatal(err)
	}
	if store.saves != 1 {
		t.Fatalf("expected usage update to be deferred, got %d saves", store.saves)
	}

	if err := manager.Flush(); err != nil {
		t.Fatal(err)
	}
	if store.saves != 2 {
		t.Fatalf("expected flush to persist deferred usage, got %d saves", store.saves)
	}
	if store.tokens[0].Stats.TotalTokens != 10 {
		t.Fatalf("expected flushed stats, got %#v", store.tokens[0].Stats)
	}
}

func TestSecureStoreProtectsTokenValuesAndMigratesPlaintext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	rawStore := storage.NewJSONStore[[]Token](path)
	store := NewSecureStore(rawStore)

	manager, err := NewManager(store, 15)
	if err != nil {
		t.Fatal(err)
	}
	item, err := manager.Add(UpsertRequest{Name: "primary", Provider: ProviderOpenAI, TokenValue: "sk-secure-token"})
	if err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "sk-secure-token") {
		t.Fatalf("stored token file leaked plaintext secret: %s", string(raw))
	}

	reloaded, err := NewManager(store, 15)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := reloaded.Get(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.TokenValue != "sk-secure-token" {
		t.Fatalf("expected decrypted token value, got %q", loaded.TokenValue)
	}

	if err := rawStore.Save([]Token{{
		ID:             "legacy",
		Name:           "legacy",
		Provider:       ProviderOpenAI,
		CredentialType: CredentialTypeAPIKey,
		TokenValue:     "sk-legacy-token",
		Remaining:      100,
		Status:         StatusActive,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}}); err != nil {
		t.Fatal(err)
	}
	migrated, err := NewManager(store, 15)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := migrated.Get("legacy")
	if err != nil {
		t.Fatal(err)
	}
	if legacy.TokenValue != "sk-legacy-token" {
		t.Fatalf("expected migrated legacy token to decrypt, got %q", legacy.TokenValue)
	}
	raw, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "sk-legacy-token") {
		t.Fatalf("legacy token was not migrated to protected storage: %s", string(raw))
	}
}

func TestManagerHealthCooldownCandidatesAndRecovery(t *testing.T) {
	manager, err := NewManager(storage.NewJSONStore[[]Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	item, err := manager.Add(UpsertRequest{Name: "primary", Provider: ProviderOpenAI, TokenValue: "sk-primary-token"})
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	cooldownUntil := now.Add(time.Hour)
	if err := manager.MarkExhaustedUntil(item.ID, "upstream returned 429", &cooldownUntil); err != nil {
		t.Fatal(err)
	}
	if candidates := manager.HealthCheckCandidates(now, time.Minute, time.Minute); len(candidates) != 0 {
		t.Fatalf("expected active cooldown to skip health check, got %#v", candidates)
	}

	afterCooldown := cooldownUntil.Add(time.Second)
	candidates := manager.HealthCheckCandidates(afterCooldown, time.Minute, time.Minute)
	if len(candidates) != 1 || candidates[0].ID != item.ID {
		t.Fatalf("expected expired cooldown to be checked, got %#v", candidates)
	}

	if err := manager.RecordUsage(item.ID, 80); err != nil {
		t.Fatal(err)
	}
	if err := manager.RecordHealthCheck(item.ID, true, 200, "OK", nil); err != nil {
		t.Fatal(err)
	}
	updated, err := manager.Get(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != StatusActive || updated.CooldownUntil != nil || updated.Health.ConsecutiveErrors != 0 {
		t.Fatalf("expected health recovery to clear cooldown and restore active status, got %#v", updated)
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

type countingTokenStore struct {
	tokens []Token
	saves  int
}

func (s *countingTokenStore) Load() ([]Token, error) {
	out := make([]Token, len(s.tokens))
	copy(out, s.tokens)
	return out, nil
}

func (s *countingTokenStore) Save(tokens []Token) error {
	s.saves++
	s.tokens = make([]Token, len(tokens))
	copy(s.tokens, tokens)
	return nil
}
