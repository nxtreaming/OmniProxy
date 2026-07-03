package history

import (
	"database/sql"
	"encoding/json"
	_ "modernc.org/sqlite"
	"strings"
	"time"
)

func insertEntry(stmt *sql.Stmt, entry Entry) error {
	_, err := stmt.Exec(entryValues(entry)...)
	return err
}

func upsertDailyUsage(stmt *sql.Stmt, entry Entry) error {
	values, ok := dailyUsageValues(entry)
	if !ok {
		return nil
	}
	_, err := stmt.Exec(values...)
	return err
}

func upsertDailyUsageTx(tx *sql.Tx, entry Entry) error {
	values, ok := dailyUsageValues(entry)
	if !ok {
		return nil
	}
	_, err := tx.Exec(upsertDailyUsageSQL, values...)
	return err
}

func upsertBillingLifetime(stmt *sql.Stmt, entry Entry) error {
	values, ok := billingLifetimeValues(entry)
	if !ok {
		return nil
	}
	_, err := stmt.Exec(values...)
	return err
}

func upsertBillingLifetimeTx(tx *sql.Tx, entry Entry) error {
	values, ok := billingLifetimeValues(entry)
	if !ok {
		return nil
	}
	_, err := tx.Exec(upsertBillingLifetimeSQL, values...)
	return err
}

func upsertRequestDailySummary(stmt *sql.Stmt, entry Entry) error {
	_, err := stmt.Exec(requestDailySummaryValues(entry)...)
	return err
}

func upsertRequestDailySummaryTx(tx *sql.Tx, entry Entry) error {
	_, err := tx.Exec(upsertRequestDailySummarySQL, requestDailySummaryValues(entry)...)
	return err
}

func entryValues(entry Entry) []any {
	retryChain, _ := json.Marshal(entry.RetryChain)
	return []any{
		entryIDValue(entry.ID),
		entry.Time.Format(time.RFC3339Nano),
		entry.Level,
		entry.Method,
		entry.Path,
		entry.Provider,
		entry.Protocol,
		entry.Model,
		entry.Status,
		entry.Duration,
		entry.ClientKey,
		entry.ClientName,
		entry.TokenID,
		entry.TokenName,
		entry.InputTokens,
		entry.OutputTokens,
		entry.TotalTokens,
		boolInt(entry.CooldownTriggered),
		string(retryChain),
		entry.Message,
	}
}

func requestDailySummaryValues(entry Entry) []any {
	_, _, total := tokenCounts(entry)
	return []any{
		entry.Time.Local().Format("2006-01-02"),
		strings.TrimSpace(entry.Provider),
		strings.TrimSpace(entry.Protocol),
		strings.TrimSpace(entry.ClientKey),
		strings.TrimSpace(entry.ClientName),
		requestSummaryClientLabel(entry),
		strings.TrimSpace(entry.Level),
		entry.Status,
		strings.TrimSpace(entry.Model),
		strings.TrimSpace(entry.TokenName),
		boolInt(failedEntry(entry)),
		total,
		maxInt64(entry.Duration, 0),
		time.Now().Format(time.RFC3339Nano),
	}
}

func requestSummaryClientLabel(entry Entry) string {
	if strings.EqualFold(strings.TrimSpace(entry.Method), "CHECK") {
		return "__status_check__"
	}
	if strings.EqualFold(strings.TrimSpace(entry.Protocol), "health-check") {
		return "__status_check__"
	}
	if strings.Contains(strings.ToLower(strings.TrimSpace(entry.Path)), "/maintenance/token-health-check") {
		return "__status_check__"
	}
	if name := strings.TrimSpace(entry.ClientName); name != "" {
		return name
	}
	return strings.TrimSpace(entry.ClientKey)
}

func entryIDValue(id int64) any {
	if id <= 0 {
		return nil
	}
	return id
}

func dailyUsageValues(entry Entry) ([]any, bool) {
	if !dailyUsageCandidate(entry) {
		return nil, false
	}
	input, output, total := tokenCounts(entry)
	return []any{
		entry.Time.Local().Format("2006-01-02"),
		strings.TrimSpace(entry.Provider),
		strings.TrimSpace(entry.Protocol),
		strings.TrimSpace(entry.ClientKey),
		strings.TrimSpace(entry.ClientName),
		strings.TrimSpace(entry.Model),
		input,
		output,
		total,
		time.Now().Format(time.RFC3339Nano),
	}, true
}

func billingLifetimeValues(entry Entry) ([]any, bool) {
	if !dailyUsageCandidate(entry) {
		return nil, false
	}
	input, output, total := tokenCounts(entry)
	return []any{
		input,
		output,
		total,
		time.Now().Format(time.RFC3339Nano),
	}, true
}

func dailyUsageCandidate(entry Entry) bool {
	if strings.TrimSpace(entry.Model) == "" {
		return false
	}
	_, _, total := tokenCounts(entry)
	if total <= 0 {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(entry.Method), "CHECK") {
		return false
	}
	path := strings.ToLower(strings.TrimSpace(entry.Path))
	if strings.HasPrefix(path, "/maintenance/") {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(entry.Protocol)) {
	case "health-check", "quota-refresh", "token-validation":
		return false
	default:
		return true
	}
}

func tokenCounts(entry Entry) (int, int, int) {
	input := maxInt(entry.InputTokens, 0)
	output := maxInt(entry.OutputTokens, 0)
	total := maxInt(entry.TotalTokens, 0)
	if total <= 0 {
		total = input + output
	}
	if input <= 0 && total > output {
		input = total - output
	}
	if output <= 0 && total > input {
		output = total - input
	}
	return input, output, total
}
