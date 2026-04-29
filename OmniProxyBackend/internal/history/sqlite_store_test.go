package history

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteStoreListFiltersAndPrunes(t *testing.T) {
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "request_history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	recorder, err := NewRecorder(store, 2)
	if err != nil {
		t.Fatal(err)
	}
	recorder.Add(Entry{
		Time:        time.Now().Add(-2 * time.Minute),
		Level:       "info",
		Method:      "POST",
		Path:        "/v1/chat/completions",
		Provider:    "openai",
		Model:       "gpt-5.5",
		Status:      200,
		TokenName:   "primary",
		TotalTokens: 42,
		Message:     "request proxied",
	})
	recorder.Add(Entry{
		Time:              time.Now().Add(-time.Minute),
		Level:             "warn",
		Method:            "POST",
		Path:              "/v1/chat/completions",
		Provider:          "openai",
		Model:             "gpt-5.5",
		Status:            429,
		TokenName:         "backup",
		CooldownTriggered: true,
		RetryChain: []RetryAttempt{{
			Attempt:           1,
			Provider:          "openai",
			Status:            429,
			TokenName:         "backup",
			CooldownTriggered: true,
		}},
		Message: "upstream returned 429",
	})
	recorder.Add(Entry{
		Time:      time.Now(),
		Level:     "error",
		Method:    "POST",
		Path:      "/anthropic-router/v1/messages",
		Provider:  "deepseek",
		Protocol:  "anthropic",
		Model:     "deepseek-v4",
		Status:    502,
		TokenName: "deepseek-main",
		Message:   "proxy failed",
	})

	if all := recorder.List(Filter{}); len(all) != 2 {
		t.Fatalf("expected prune to keep latest 2 entries, got %d: %#v", len(all), all)
	}
	errors := recorder.List(Filter{Status: "error"})
	if len(errors) != 2 {
		t.Fatalf("expected two error entries, got %#v", errors)
	}
	deepseek := recorder.List(Filter{Provider: "deepseek", Search: "anthropic"})
	if len(deepseek) != 1 || deepseek[0].Protocol != "anthropic" {
		t.Fatalf("expected filtered deepseek anthropic entry, got %#v", deepseek)
	}
	openai := recorder.List(Filter{Model: "gpt-5.5"})
	if len(openai) != 1 || openai[0].Model != "gpt-5.5" {
		t.Fatalf("expected persisted model metadata, got %#v", openai)
	}
	cooldown := recorder.List(Filter{Status: "429"})
	if len(cooldown) != 1 || !cooldown[0].CooldownTriggered || len(cooldown[0].RetryChain) != 1 {
		t.Fatalf("expected persisted retry chain and cooldown state, got %#v", cooldown)
	}
}

func TestSQLiteStoreSaveImportsLegacyEntries(t *testing.T) {
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "request_history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	entries := []Entry{{
		ID:       12,
		Time:     time.Now(),
		Level:    "info",
		Provider: "openai",
		Status:   200,
		Message:  "legacy",
	}}
	if err := store.Save(entries); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.List(Filter{Status: "success"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].ID != 12 || loaded[0].Message != "legacy" {
		t.Fatalf("expected imported legacy entry, got %#v", loaded)
	}
}

func TestSQLiteStoreSaveAssignsIDsForLegacyEntriesWithoutIDs(t *testing.T) {
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "request_history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	entries := []Entry{
		{
			Time:    time.Now().Add(-time.Minute),
			Level:   "info",
			Status:  200,
			Message: "first legacy entry",
		},
		{
			Time:    time.Now(),
			Level:   "warn",
			Status:  429,
			Message: "second legacy entry",
		},
	}
	if err := store.Save(entries); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.List(Filter{}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected both legacy entries, got %#v", loaded)
	}
	if loaded[0].ID == 0 || loaded[1].ID == 0 || loaded[0].ID == loaded[1].ID {
		t.Fatalf("expected generated unique IDs, got %#v", loaded)
	}
}
