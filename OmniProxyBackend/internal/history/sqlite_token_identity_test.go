package history

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteStoreDailyUsageSeparatesTokenIDRows(t *testing.T) {
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
	for _, entry := range []Entry{
		{
			Time:        usageDate,
			Level:       "info",
			Method:      "POST",
			Path:        "/v1/chat/completions",
			Provider:    "openai",
			Protocol:    "openai",
			ClientKey:   "codex",
			ClientName:  "Codex",
			Model:       "gpt-5.5",
			Status:      200,
			TokenID:     "workspace-a",
			TokenName:   "coder@example.com",
			TotalTokens: 100,
			Message:     "request proxied",
		},
		{
			Time:        usageDate.Add(time.Minute),
			Level:       "info",
			Method:      "POST",
			Path:        "/v1/chat/completions",
			Provider:    "openai",
			Protocol:    "openai",
			ClientKey:   "codex",
			ClientName:  "Codex",
			Model:       "gpt-5.5",
			Status:      200,
			TokenID:     "workspace-b",
			TokenName:   "coder@example.com",
			TotalTokens: 200,
			Message:     "request proxied",
		},
	} {
		recorder.Add(entry)
	}

	usage, err := store.DailyUsage("2026-05-01", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(usage) != 2 {
		t.Fatalf("expected same model to stay separated by token id, got %#v", usage)
	}
	if usage[0].TokenID != "workspace-b" || usage[0].TotalTokens != 200 {
		t.Fatalf("unexpected first usage row: %#v", usage[0])
	}
	if usage[1].TokenID != "workspace-a" || usage[1].TotalTokens != 100 {
		t.Fatalf("unexpected second usage row: %#v", usage[1])
	}
	summary := recorder.BillingSummary(30)
	if summary.RequestCount != 2 || summary.TotalTokens != 300 {
		t.Fatalf("expected billing summary to still total both workspaces, got %#v", summary)
	}
}

func TestSQLiteStoreRelabelTokenNamesUpdatesHistoryAndSummaries(t *testing.T) {
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "request_history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	recorder, err := NewRecorder(store, 10)
	if err != nil {
		t.Fatal(err)
	}
	baseTime := time.Date(2026, 5, 2, 10, 0, 0, 0, time.Local)
	recorder.Add(Entry{
		Time:        baseTime,
		Level:       "warn",
		Method:      "POST",
		Path:        "/v1/chat/completions",
		Provider:    "openai",
		ClientKey:   "codex",
		ClientName:  "Codex",
		Model:       "gpt-5.5",
		Status:      429,
		Duration:    30,
		TokenID:     "workspace-a",
		TokenName:   "coder@example.com",
		TotalTokens: 35,
		Message:     "rate limited",
	})

	updatedName := "coder@example.com (account_id: 12345678...abcdef)"
	if err := store.RelabelTokenNames(map[string]string{"workspace-a": updatedName}); err != nil {
		t.Fatal(err)
	}

	entries, err := store.List(Filter{}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].TokenName != updatedName {
		t.Fatalf("expected request history token name to be relabeled, got %#v", entries)
	}
	usage, err := store.DailyUsage("2026-05-02", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(usage) != 1 || usage[0].TokenName != updatedName {
		t.Fatalf("expected billing usage token name to be relabeled, got %#v", usage)
	}
	summary, err := store.Summary(Filter{}, 14)
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.TokenFailureRanks) != 1 || summary.TokenFailureRanks[0].Label != updatedName {
		t.Fatalf("expected request summary token rank to be relabeled, got %#v", summary.TokenFailureRanks)
	}
}

func TestSQLiteStoreSummaryFiltersAndRanksByTokenID(t *testing.T) {
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "request_history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	recorder, err := NewRecorder(store, 10)
	if err != nil {
		t.Fatal(err)
	}
	usageDate := time.Date(2026, 5, 3, 10, 0, 0, 0, time.Local)
	recorder.Add(Entry{
		Time:        usageDate,
		Level:       "info",
		Method:      "POST",
		Provider:    "openai",
		Protocol:    "openai",
		ClientKey:   "codex",
		ClientName:  "Codex",
		Model:       "gpt-5.5",
		Status:      200,
		TokenID:     "workspace-a",
		TokenName:   "coder@example.com (account_id: a)",
		TotalTokens: 100,
		Message:     "request proxied",
	})
	recorder.Add(Entry{
		Time:        usageDate.Add(time.Minute),
		Level:       "info",
		Method:      "POST",
		Provider:    "openai",
		Protocol:    "openai",
		ClientKey:   "codex",
		ClientName:  "Codex",
		Model:       "gpt-5.5",
		Status:      200,
		TokenID:     "workspace-b",
		TokenName:   "coder@example.com (account_id: b)",
		TotalTokens: 300,
		Message:     "request proxied",
	})

	summary := recorder.Summary(Filter{TokenID: "workspace-b"}, 14)
	if summary.Total != 1 || summary.TotalTokens != 300 {
		t.Fatalf("expected token_id filter to isolate workspace-b, got %#v", summary)
	}
	all := recorder.Summary(Filter{}, 14)
	if len(all.TokenRanks) != 2 || all.TokenRanks[0].Label != "coder@example.com (account_id: b)" || all.TokenRanks[0].TotalTokens != 300 {
		t.Fatalf("expected token ranks to preserve workspace identity, got %#v", all.TokenRanks)
	}
}

func TestSQLiteStoreRebuildSummariesFromRequestHistory(t *testing.T) {
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "request_history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	recorder, err := NewRecorder(store, 10)
	if err != nil {
		t.Fatal(err)
	}
	recorder.Add(Entry{
		Time:         time.Date(2026, 5, 4, 10, 0, 0, 0, time.Local),
		Level:        "info",
		Method:       "POST",
		Provider:     "openai",
		Protocol:     "openai",
		ClientKey:    "codex",
		ClientName:   "Codex",
		Model:        "gpt-5.5",
		Status:       200,
		TokenID:      "workspace-a",
		TokenName:    "coder@example.com (account_id: a)",
		InputTokens:  70,
		OutputTokens: 30,
		TotalTokens:  100,
		Message:      "request proxied",
	})
	if err := recorder.ClearDailyUsage(); err != nil {
		t.Fatal(err)
	}
	if summary := recorder.BillingSummary(30); summary.TotalTokens != 0 {
		t.Fatalf("expected cleared summary, got %#v", summary)
	}
	if err := recorder.RebuildSummaries(); err != nil {
		t.Fatal(err)
	}
	summary := recorder.BillingSummary(30)
	if summary.RequestCount != 1 || summary.InputTokens != 70 || summary.OutputTokens != 30 || summary.TotalTokens != 100 {
		t.Fatalf("expected billing summary to be rebuilt, got %#v", summary)
	}
	usage, err := store.DailyUsage("2026-05-04", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(usage) != 1 || usage[0].TokenID != "workspace-a" || usage[0].TotalTokens != 100 {
		t.Fatalf("expected daily usage to be rebuilt with token_id, got %#v", usage)
	}
}
