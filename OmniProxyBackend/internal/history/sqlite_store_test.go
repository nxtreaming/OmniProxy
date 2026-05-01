package history

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
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
		ClientKey:   "codex",
		ClientName:  "Codex",
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
		ClientKey:         "opencode",
		ClientName:        "OpenCode",
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
		Time:       time.Now(),
		Level:      "error",
		Method:     "POST",
		Path:       "/anthropic-router/v1/messages",
		Provider:   "deepseek",
		Protocol:   "anthropic",
		ClientKey:  "claude",
		ClientName: "Claude Code",
		Model:      "deepseek-v4",
		Status:     502,
		TokenName:  "deepseek-main",
		Message:    "proxy failed",
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
	claude := recorder.List(Filter{Client: "claude"})
	if len(claude) != 1 || claude[0].ClientName != "Claude Code" {
		t.Fatalf("expected persisted client metadata, got %#v", claude)
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

func TestSQLiteStoreDailyUsageSummarizesBillableEntries(t *testing.T) {
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "request_history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	recorder, err := NewRecorder(store, 10)
	if err != nil {
		t.Fatal(err)
	}
	usageDate := time.Date(2026, 5, 1, 10, 30, 0, 0, time.Local)
	recorder.Add(Entry{
		Time:         usageDate,
		Level:        "info",
		Method:       "POST",
		Path:         "/v1/chat/completions",
		Provider:     "openai",
		Protocol:     "openai",
		ClientKey:    "codex",
		ClientName:   "Codex",
		Model:        "gpt-5.5",
		Status:       200,
		InputTokens:  100,
		OutputTokens: 50,
		TotalTokens:  150,
		Message:      "request proxied",
	})
	recorder.Add(Entry{
		Time:        usageDate.Add(time.Minute),
		Level:       "info",
		Method:      "CHECK",
		Path:        "/maintenance/current-token-quota-refresh",
		Provider:    "openai",
		Protocol:    "quota-refresh",
		Model:       "codex_auth_json",
		Status:      200,
		TotalTokens: 999,
		Message:     "quota refresh completed",
	})
	recorder.Add(Entry{
		Time:        usageDate.Add(2 * time.Minute),
		Level:       "info",
		Method:      "POST",
		Path:        "/v1/chat/completions",
		Provider:    "openai",
		Status:      200,
		TotalTokens: 88,
		Message:     "missing model",
	})

	usage, err := store.DailyUsage("2026-05-01", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(usage) != 1 {
		t.Fatalf("expected one billable usage row, got %#v", usage)
	}
	if usage[0].Model != "gpt-5.5" || usage[0].RequestCount != 1 || usage[0].InputTokens != 100 || usage[0].OutputTokens != 50 || usage[0].TotalTokens != 150 {
		t.Fatalf("unexpected usage row: %#v", usage[0])
	}
	dates, err := store.DailyUsageDates(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(dates) != 1 || dates[0] != "2026-05-01" {
		t.Fatalf("expected usage date to be listed, got %#v", dates)
	}
}

func TestRecorderRetentionPrunesHistoryAndDailyUsageByDate(t *testing.T) {
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "request_history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	recorder, err := NewRecorder(store, 10)
	if err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().AddDate(0, 0, -20)
	recentTime := time.Now()
	recorder.Add(Entry{
		Time:        oldTime,
		Level:       "info",
		Method:      "POST",
		Path:        "/v1/chat/completions",
		Provider:    "openai",
		Model:       "gpt-5.4",
		Status:      200,
		TotalTokens: 100,
		Message:     "old",
	})
	recorder.Add(Entry{
		Time:        recentTime,
		Level:       "info",
		Method:      "POST",
		Path:        "/v1/chat/completions",
		Provider:    "openai",
		Model:       "gpt-5.5",
		Status:      200,
		TotalTokens: 200,
		Message:     "recent",
	})
	if err := recorder.SetRetentionDays(14); err != nil {
		t.Fatal(err)
	}

	entries := recorder.List(Filter{})
	if len(entries) != 1 || entries[0].Model != "gpt-5.5" {
		t.Fatalf("expected only recent history after retention, got %#v", entries)
	}
	dates, err := store.DailyUsageDates(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(dates) != 1 || dates[0] != recentTime.Format("2006-01-02") {
		t.Fatalf("expected only recent usage date after retention, got %#v", dates)
	}
}

func TestSQLiteStoreMigratesExistingRowsWithoutClientColumns(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "request_history.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
CREATE TABLE request_history (
  id INTEGER PRIMARY KEY,
  time TEXT NOT NULL,
  level TEXT NOT NULL,
  method TEXT,
  path TEXT,
  provider TEXT,
  protocol TEXT,
  model TEXT,
  status INTEGER,
  duration_ms INTEGER,
  token_id TEXT,
  token_name TEXT,
  input_tokens INTEGER,
  output_tokens INTEGER,
  total_tokens INTEGER,
  cooldown_triggered INTEGER,
  retry_chain TEXT,
  message TEXT NOT NULL
)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO request_history (id, time, level, provider, protocol, model, status, retry_chain, message) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		1, time.Now().Format(time.RFC3339Nano), "info", "openai", "openai", "gpt-5.5", 200, "[]", "legacy request"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	loaded, err := store.List(Filter{}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].ClientKey != "" || loaded[0].Message != "legacy request" {
		t.Fatalf("expected legacy row to load with empty client fields, got %#v", loaded)
	}
}
