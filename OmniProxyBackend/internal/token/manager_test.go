package token

import (
	"encoding/base64"
	"encoding/json"
	"errors"
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

func TestManagerSkipsDisabledTokens(t *testing.T) {
	manager, err := NewManager(storage.NewJSONStore[[]Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}

	disabled, err := manager.Add(UpsertRequest{Name: "disabled", Provider: "openai", TokenValue: "sk-disabled-token"})
	if err != nil {
		t.Fatal(err)
	}
	enabled, err := manager.Add(UpsertRequest{Name: "enabled", Provider: "openai", TokenValue: "sk-enabled-token"})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := manager.SetDisabled(disabled.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if !updated.Disabled {
		t.Fatal("expected token to be disabled")
	}

	selected, err := manager.Acquire(ProviderOpenAI, map[string]bool{enabled.ID: true})
	if err != ErrNoActiveToken {
		t.Fatalf("expected no active token when only disabled account remains, got selected=%#v err=%v", selected, err)
	}
	selected, err = manager.Acquire(ProviderOpenAI, nil)
	if err != nil {
		t.Fatal(err)
	}
	if selected.ID != enabled.ID {
		t.Fatalf("expected enabled token, got %s", selected.Name)
	}

	candidates := manager.HealthCheckCandidates(time.Now(), time.Millisecond, time.Millisecond)
	if len(candidates) != 1 || candidates[0].ID != enabled.ID {
		t.Fatalf("expected only enabled health check candidate, got %#v", candidates)
	}
}

func TestManagerSelectedProviderAccountsRestrictSchedulingWithoutDisablingOthers(t *testing.T) {
	manager, err := NewManager(storage.NewJSONStore[[]Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}

	target, err := manager.Add(UpsertRequest{Name: "target", Provider: "openai", TokenValue: "sk-target-token"})
	if err != nil {
		t.Fatal(err)
	}
	otherOpenAI, err := manager.Add(UpsertRequest{Name: "other", Provider: "openai", TokenValue: "sk-other-token"})
	if err != nil {
		t.Fatal(err)
	}
	unselectedOpenAI, err := manager.Add(UpsertRequest{Name: "unselected", Provider: "openai", TokenValue: "sk-unselected-token"})
	if err != nil {
		t.Fatal(err)
	}
	otherProvider, err := manager.Add(UpsertRequest{Name: "anthropic", Provider: "anthropic", TokenValue: "sk-ant-token"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.SetDisabled(otherProvider.ID, true); err != nil {
		t.Fatal(err)
	}

	if _, err := manager.SetSelected(target.ID, true); err != nil {
		t.Fatal(err)
	}
	items, err := manager.SetSelected(otherOpenAI.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]Token{}
	for _, item := range items {
		byID[item.ID] = item
	}
	if !byID[target.ID].Selected || !byID[otherOpenAI.ID].Selected {
		t.Fatal("expected selected OpenAI tokens to be marked selected")
	}
	if byID[unselectedOpenAI.ID].Selected {
		t.Fatal("expected unselected OpenAI token to remain outside the selection set")
	}
	if byID[unselectedOpenAI.ID].Disabled {
		t.Fatal("expected unselected OpenAI token to keep enabled state")
	}
	if !byID[otherProvider.ID].Disabled {
		t.Fatal("expected other provider token state to be unchanged")
	}

	selected, err := manager.Acquire(ProviderOpenAI, map[string]bool{target.ID: true})
	if err != nil {
		t.Fatal(err)
	}
	if selected.ID != otherOpenAI.ID {
		t.Fatalf("expected scheduling to stay inside selected accounts, got %s", selected.Name)
	}
	if _, err := manager.Acquire(ProviderOpenAI, map[string]bool{target.ID: true, otherOpenAI.ID: true}); !errors.Is(err, ErrNoActiveToken) {
		t.Fatalf("expected selected provider not to fall back to unselected accounts, got %v", err)
	}

	items, err = manager.ClearProviderSelectionForToken(target.ID)
	if err != nil {
		t.Fatal(err)
	}
	byID = map[string]Token{}
	for _, item := range items {
		byID[item.ID] = item
	}
	if byID[target.ID].Selected || byID[otherOpenAI.ID].Selected || byID[unselectedOpenAI.ID].Selected {
		t.Fatal("expected OpenAI provider selection to be cleared")
	}
	if byID[target.ID].Disabled || byID[otherOpenAI.ID].Disabled || byID[unselectedOpenAI.ID].Disabled {
		t.Fatal("expected OpenAI disabled states to remain enabled after clearing selection")
	}
	if !byID[otherProvider.ID].Disabled {
		t.Fatal("expected other provider token state to remain unchanged after clearing selection")
	}
}

func TestManagerDisablingSelectedTokenClearsSelection(t *testing.T) {
	manager, err := NewManager(storage.NewJSONStore[[]Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}

	target, err := manager.Add(UpsertRequest{Name: "target", Provider: "openai", TokenValue: "sk-target-token"})
	if err != nil {
		t.Fatal(err)
	}
	backup, err := manager.Add(UpsertRequest{Name: "backup", Provider: "openai", TokenValue: "sk-backup-token"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.SetSelected(target.ID, true); err != nil {
		t.Fatal(err)
	}
	updated, err := manager.SetDisabled(target.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Selected {
		t.Fatal("expected disabling a selected token to clear its selection")
	}

	selected, err := manager.Acquire(ProviderOpenAI, nil)
	if err != nil {
		t.Fatal(err)
	}
	if selected.ID != backup.ID {
		t.Fatalf("expected backup token after disabling pinned token, got %s", selected.Name)
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
	plan, err := manager.Add(UpsertRequest{
		Name:           "plan",
		Provider:       ProviderXiaomi,
		CredentialType: CredentialTypeMimoTokenPlan,
		TokenValue:     "tp-xiaomi-token-plan",
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Region != MimoRegionCN {
		t.Fatalf("expected default MiMo Token Plan region cn, got %q", plan.Region)
	}
	planSGP, err := manager.Add(UpsertRequest{
		Name:           "plan-sgp",
		Provider:       ProviderXiaomi,
		CredentialType: CredentialTypeMimoTokenPlan,
		Region:         MimoRegionSGP,
		TokenValue:     "tp-xiaomi-token-plan-sgp",
	})
	if err != nil {
		t.Fatal(err)
	}
	if planSGP.Region != MimoRegionSGP {
		t.Fatalf("expected MiMo Token Plan region sgp, got %q", planSGP.Region)
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
	if _, err := manager.Add(UpsertRequest{
		Name:           "bad-plan-region",
		Provider:       ProviderXiaomi,
		CredentialType: CredentialTypeMimoTokenPlan,
		Region:         "moon",
		TokenValue:     "tp-xiaomi-token-plan-region",
	}); err == nil {
		t.Fatal("expected xiaomi token plan region error")
	}
}

func TestManagerAllowsZhipuCodingPlanCredential(t *testing.T) {
	manager, err := NewManager(storage.NewJSONStore[[]Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	item, err := manager.Add(UpsertRequest{
		Name:           "glm-coding",
		Provider:       ProviderZhipu,
		CredentialType: CredentialTypeCodingPlan,
		TokenValue:     "zhipu-coding-plan-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	if item.CredentialType != CredentialTypeCodingPlan {
		t.Fatalf("expected coding plan credential type, got %q", item.CredentialType)
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

func TestManagerUpdateAllowsMimoTokenPlanRegionChangeWithoutTokenValue(t *testing.T) {
	manager, err := NewManager(storage.NewJSONStore[[]Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	item, err := manager.Add(UpsertRequest{
		Name:           "plan",
		Provider:       ProviderXiaomi,
		CredentialType: CredentialTypeMimoTokenPlan,
		Region:         MimoRegionCN,
		TokenValue:     "tp-xiaomi-token-plan",
	})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := manager.Update(item.ID, UpsertRequest{
		Name:           "plan",
		Provider:       ProviderXiaomi,
		CredentialType: CredentialTypeMimoTokenPlan,
		Region:         MimoRegionSGP,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Region != MimoRegionSGP || updated.TokenValue != "tp-xiaomi-token-plan" {
		t.Fatalf("expected region update with preserved token, got %#v", updated)
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

func TestManagerUsesFlatCodexEmailAsName(t *testing.T) {
	manager, err := NewManager(storage.NewJSONStore[[]Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}

	authJSON, err := json.Marshal(map[string]any{
		"type":         "codex",
		"email":        "flat@example.com",
		"access_token": "flat-access-token",
		"id_token":     codexJWTForTest(t, "flat@example.com", "flat-account"),
	})
	if err != nil {
		t.Fatal(err)
	}
	item, err := manager.Add(UpsertRequest{
		Name:           "",
		Provider:       ProviderOpenAI,
		CredentialType: CredentialTypeCodexAuthJSON,
		TokenValue:     string(authJSON),
	})
	if err != nil {
		t.Fatal(err)
	}
	if item.Name != "flat@example.com" {
		t.Fatalf("expected flat email as name, got %q", item.Name)
	}
}

func TestManagerUsesClaudeOAuthEmailAsName(t *testing.T) {
	manager, err := NewManager(storage.NewJSONStore[[]Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}

	item, err := manager.Add(UpsertRequest{
		Name:           "",
		Provider:       ProviderAnthropic,
		CredentialType: CredentialTypeClaudeOAuth,
		TokenValue:     `{"access_token":"claude-access-token","refresh_token":"claude-refresh-token","email":"claude@example.com","expired":"2026-05-01T08:09:54+08:00"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if item.Name != "claude@example.com" {
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

func TestManagerRecordsSubscriptionUsageFallsBackToSecondaryWindow(t *testing.T) {
	manager, err := NewManager(storage.NewJSONStore[[]Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	item, err := manager.Add(UpsertRequest{
		Name:           "free-codex",
		Provider:       ProviderOpenAI,
		CredentialType: CredentialTypeCodexAuthJSON,
		TokenValue:     codexAuthJSONForTest(t, "free@example.com"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := manager.RecordUsageInfo(item.ID, UsageInfo{
		Source:                     "codex",
		PlanType:                   "free",
		SecondaryUsedPercent:       18,
		SecondaryRemainingPercent:  82,
		SecondaryResetAt:           1777798105,
		SubscriptionQuotaAvailable: true,
	}); err != nil {
		t.Fatal(err)
	}

	updated, err := manager.Get(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Remaining != 82 || updated.Status != StatusActive {
		t.Fatalf("expected secondary quota to keep token active, got status=%s remaining=%d usage=%#v", updated.Status, updated.Remaining, updated.Usage)
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
