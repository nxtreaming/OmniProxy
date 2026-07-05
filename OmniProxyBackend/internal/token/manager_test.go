package token

import (
	"omniproxy/internal/storage"
	"path/filepath"
	"testing"
	"time"
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

func TestManagerSelectedProviderAccountsPreferSelectionWithoutDisablingOthers(t *testing.T) {
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
	selected, err = manager.Acquire(ProviderOpenAI, map[string]bool{target.ID: true, otherOpenAI.ID: true})
	if err != nil {
		t.Fatal(err)
	}
	if selected.ID != unselectedOpenAI.ID {
		t.Fatalf("expected active unselected account before low or unavailable selected accounts, got %s", selected.Name)
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
