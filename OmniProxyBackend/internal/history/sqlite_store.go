package history

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	mu   sync.Mutex
	path string
	db   *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	store := &SQLiteStore{path: path, db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) Load() ([]Entry, error) {
	return s.List(Filter{}, 0)
}

func (s *SQLiteStore) Save(entries []Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM request_history`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM billing_daily_usage`); err != nil {
		return err
	}
	historyStmt, err := tx.Prepare(insertHistorySQL)
	if err != nil {
		return err
	}
	defer historyStmt.Close()

	usageStmt, err := tx.Prepare(upsertDailyUsageSQL)
	if err != nil {
		return err
	}
	defer usageStmt.Close()

	for _, entry := range entries {
		if err := insertEntry(historyStmt, entry); err != nil {
			return err
		}
		if err := upsertDailyUsage(usageStmt, entry); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) Append(entry Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(insertHistorySQL, entryValues(entry)...); err != nil {
		return err
	}
	if err := upsertDailyUsageTx(tx, entry); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteStore) List(filter Filter, limit int) ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 {
		limit = defaultMaxEntries
	}
	query, args := historyListQuery(filter, limit)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEntries(rows)
}

func (s *SQLiteStore) Prune(max int) error {
	if max <= 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
DELETE FROM request_history
WHERE id NOT IN (
  SELECT id FROM request_history ORDER BY id DESC LIMIT ?
)`, max)
	return err
}

func (s *SQLiteStore) DailyUsage(date string, limit int) ([]DailyUsage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 {
		limit = defaultMaxEntries
	}
	rows, err := s.db.Query(`
SELECT date, provider, protocol, client_key, client_name, model, request_count, input_tokens, output_tokens, total_tokens, updated_at
FROM billing_daily_usage
WHERE date = ?
ORDER BY total_tokens DESC, request_count DESC, model COLLATE NOCASE
LIMIT ?`, strings.TrimSpace(date), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDailyUsage(rows)
}

func (s *SQLiteStore) DailyUsageDates(limit int) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 {
		limit = 30
	}
	rows, err := s.db.Query(`
SELECT DISTINCT date
FROM billing_daily_usage
ORDER BY date DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dates := []string{}
	for rows.Next() {
		var date string
		if err := rows.Scan(&date); err != nil {
			return nil, err
		}
		dates = append(dates, date)
	}
	return dates, rows.Err()
}

func (s *SQLiteStore) PruneBeforeDate(cutoffDate string) error {
	cutoffDate = strings.TrimSpace(cutoffDate)
	if cutoffDate == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM request_history WHERE substr(time, 1, 10) < ?`, cutoffDate); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM billing_daily_usage WHERE date < ?`, cutoffDate); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteStore) ClearDailyUsage() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM billing_daily_usage`)
	return err
}

func (s *SQLiteStore) ClearRequestHistory() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM request_history`)
	return err
}

func (s *SQLiteStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Close()
}

func (s *SQLiteStore) init() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		return err
	}
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS request_history (
  id INTEGER PRIMARY KEY,
  time TEXT NOT NULL,
  level TEXT NOT NULL,
  method TEXT,
  path TEXT,
  provider TEXT,
  protocol TEXT,
  client_key TEXT,
  client_name TEXT,
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
);
`)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS billing_daily_usage (
  date TEXT NOT NULL,
  provider TEXT NOT NULL DEFAULT '',
  protocol TEXT NOT NULL DEFAULT '',
  client_key TEXT NOT NULL DEFAULT '',
  client_name TEXT NOT NULL DEFAULT '',
  model TEXT NOT NULL DEFAULT '',
  request_count INTEGER NOT NULL DEFAULT 0,
  input_tokens INTEGER NOT NULL DEFAULT 0,
  output_tokens INTEGER NOT NULL DEFAULT 0,
  total_tokens INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (date, provider, protocol, client_key, model)
);
`); err != nil {
		return err
	}
	if err := s.ensureColumn("client_key", "ALTER TABLE request_history ADD COLUMN client_key TEXT"); err != nil {
		return err
	}
	if err := s.ensureColumn("client_name", "ALTER TABLE request_history ADD COLUMN client_name TEXT"); err != nil {
		return err
	}
	_, err = s.db.Exec(`
CREATE INDEX IF NOT EXISTS idx_request_history_provider ON request_history(provider);
CREATE INDEX IF NOT EXISTS idx_request_history_client_key ON request_history(client_key);
CREATE INDEX IF NOT EXISTS idx_request_history_level ON request_history(level);
CREATE INDEX IF NOT EXISTS idx_request_history_status ON request_history(status);
CREATE INDEX IF NOT EXISTS idx_request_history_model ON request_history(model);
CREATE INDEX IF NOT EXISTS idx_request_history_token_name ON request_history(token_name);
CREATE INDEX IF NOT EXISTS idx_request_history_time ON request_history(time);
CREATE INDEX IF NOT EXISTS idx_billing_daily_usage_date ON billing_daily_usage(date);
CREATE INDEX IF NOT EXISTS idx_billing_daily_usage_model ON billing_daily_usage(model);
`)
	if err != nil {
		return err
	}
	return s.rebuildDailyUsageIfEmptyLocked()
}

func (s *SQLiteStore) ensureColumn(name string, statement string) error {
	rows, err := s.db.Query(`PRAGMA table_info(request_history)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			columnName string
			columnType string
			notNull    int
			defaultVal sql.NullString
			pk         int
		)
		if err := rows.Scan(&cid, &columnName, &columnType, &notNull, &defaultVal, &pk); err != nil {
			return err
		}
		if strings.EqualFold(columnName, name) {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.Exec(statement)
	return err
}

func (s *SQLiteStore) rebuildDailyUsageIfEmptyLocked() error {
	var historyCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM request_history`).Scan(&historyCount); err != nil {
		return err
	}
	if historyCount == 0 {
		return nil
	}
	var usageCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM billing_daily_usage`).Scan(&usageCount); err != nil {
		return err
	}
	if usageCount > 0 {
		return nil
	}
	_, err := s.db.Exec(`
INSERT INTO billing_daily_usage (
  date, provider, protocol, client_key, client_name, model,
  request_count, input_tokens, output_tokens, total_tokens, updated_at
)
SELECT
  substr(time, 1, 10) AS date,
  COALESCE(provider, '') AS provider,
  COALESCE(protocol, '') AS protocol,
  COALESCE(client_key, '') AS client_key,
  COALESCE(MAX(NULLIF(client_name, '')), '') AS client_name,
  TRIM(model) AS model,
  COUNT(*) AS request_count,
  SUM(
    CASE
      WHEN COALESCE(input_tokens, 0) > 0 THEN input_tokens
      WHEN COALESCE(total_tokens, 0) > COALESCE(output_tokens, 0) THEN COALESCE(total_tokens, 0) - COALESCE(output_tokens, 0)
      ELSE 0
    END
  ) AS input_tokens,
  SUM(COALESCE(output_tokens, 0)) AS output_tokens,
  SUM(
    CASE
      WHEN COALESCE(total_tokens, 0) > 0 THEN total_tokens
      ELSE COALESCE(input_tokens, 0) + COALESCE(output_tokens, 0)
    END
  ) AS total_tokens,
  MAX(time) AS updated_at
FROM request_history
WHERE TRIM(COALESCE(model, '')) != ''
  AND (
    COALESCE(total_tokens, 0) > 0 OR
    COALESCE(input_tokens, 0) > 0 OR
    COALESCE(output_tokens, 0) > 0
  )
  AND UPPER(TRIM(COALESCE(method, ''))) != 'CHECK'
  AND LOWER(TRIM(COALESCE(path, ''))) NOT LIKE '/maintenance/%'
  AND LOWER(TRIM(COALESCE(protocol, ''))) NOT IN ('health-check', 'quota-refresh', 'token-validation')
GROUP BY date, provider, protocol, client_key, model`)
	return err
}

const insertHistorySQL = `
INSERT OR REPLACE INTO request_history (
  id, time, level, method, path, provider, protocol, model, status, duration_ms,
  client_key, client_name, token_id, token_name, input_tokens, output_tokens, total_tokens, cooldown_triggered,
  retry_chain, message
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const upsertDailyUsageSQL = `
INSERT INTO billing_daily_usage (
  date, provider, protocol, client_key, client_name, model,
  request_count, input_tokens, output_tokens, total_tokens, updated_at
) VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?, ?, ?)
ON CONFLICT(date, provider, protocol, client_key, model) DO UPDATE SET
  client_name = CASE
    WHEN excluded.client_name != '' THEN excluded.client_name
    ELSE billing_daily_usage.client_name
  END,
  request_count = billing_daily_usage.request_count + excluded.request_count,
  input_tokens = billing_daily_usage.input_tokens + excluded.input_tokens,
  output_tokens = billing_daily_usage.output_tokens + excluded.output_tokens,
  total_tokens = billing_daily_usage.total_tokens + excluded.total_tokens,
  updated_at = excluded.updated_at`

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

func historyListQuery(filter Filter, limit int) (string, []any) {
	where := make([]string, 0, 6)
	args := make([]any, 0, 8)

	if filter.Provider != "" && filter.Provider != "all" {
		where = append(where, `provider = ? COLLATE NOCASE`)
		args = append(args, filter.Provider)
	}
	if filter.Client != "" && filter.Client != "all" {
		where = append(where, `client_key = ? COLLATE NOCASE`)
		args = append(args, filter.Client)
	}
	if filter.Level != "" && filter.Level != "all" {
		where = append(where, `level = ? COLLATE NOCASE`)
		args = append(args, filter.Level)
	}
	if filter.Status != "" && filter.Status != "all" {
		statusWhere, statusArgs := statusSQL(filter.Status)
		where = append(where, statusWhere)
		args = append(args, statusArgs...)
	}
	if strings.TrimSpace(filter.Model) != "" {
		where = append(where, `model LIKE ? COLLATE NOCASE`)
		args = append(args, like(filter.Model))
	}
	if strings.TrimSpace(filter.Token) != "" {
		where = append(where, `token_name LIKE ? COLLATE NOCASE`)
		args = append(args, like(filter.Token))
	}
	if strings.TrimSpace(filter.Search) != "" {
		where = append(where, `(
method LIKE ? COLLATE NOCASE OR
path LIKE ? COLLATE NOCASE OR
provider LIKE ? COLLATE NOCASE OR
protocol LIKE ? COLLATE NOCASE OR
client_key LIKE ? COLLATE NOCASE OR
client_name LIKE ? COLLATE NOCASE OR
model LIKE ? COLLATE NOCASE OR
token_name LIKE ? COLLATE NOCASE OR
message LIKE ? COLLATE NOCASE OR
CAST(status AS TEXT) LIKE ?
)`)
		search := like(filter.Search)
		args = append(args, search, search, search, search, search, search, search, search, search, search)
	}

	var builder strings.Builder
	builder.WriteString(`SELECT id, time, level, method, path, provider, protocol, client_key, client_name, model, status, duration_ms, token_id, token_name, input_tokens, output_tokens, total_tokens, cooldown_triggered, retry_chain, message FROM request_history`)
	if len(where) > 0 {
		builder.WriteString(` WHERE `)
		builder.WriteString(strings.Join(where, ` AND `))
	}
	builder.WriteString(` ORDER BY id DESC LIMIT ?`)
	args = append(args, limit)
	return builder.String(), args
}

func statusSQL(filter string) (string, []any) {
	switch strings.ToLower(strings.TrimSpace(filter)) {
	case "success":
		return `status >= 200 AND status < 400`, nil
	case "error":
		return `(status = 0 OR status >= 400)`, nil
	default:
		parsed, err := strconv.Atoi(filter)
		if err != nil {
			return `1 = 0`, nil
		}
		return `status = ?`, []any{parsed}
	}
}

func scanEntries(rows *sql.Rows) ([]Entry, error) {
	entries := []Entry{}
	for rows.Next() {
		var entry Entry
		var encodedTime string
		var method sql.NullString
		var path sql.NullString
		var provider sql.NullString
		var protocol sql.NullString
		var clientKey sql.NullString
		var clientName sql.NullString
		var model sql.NullString
		var tokenID sql.NullString
		var tokenName sql.NullString
		var retryChain sql.NullString
		var status sql.NullInt64
		var duration sql.NullInt64
		var inputTokens sql.NullInt64
		var outputTokens sql.NullInt64
		var totalTokens sql.NullInt64
		var cooldownTriggered sql.NullInt64
		if err := rows.Scan(
			&entry.ID,
			&encodedTime,
			&entry.Level,
			&method,
			&path,
			&provider,
			&protocol,
			&clientKey,
			&clientName,
			&model,
			&status,
			&duration,
			&tokenID,
			&tokenName,
			&inputTokens,
			&outputTokens,
			&totalTokens,
			&cooldownTriggered,
			&retryChain,
			&entry.Message,
		); err != nil {
			return nil, err
		}
		parsedTime, err := time.Parse(time.RFC3339Nano, encodedTime)
		if err != nil {
			return nil, fmt.Errorf("parse request history time: %w", err)
		}
		entry.Time = parsedTime
		entry.Method = method.String
		entry.Path = path.String
		entry.Provider = provider.String
		entry.Protocol = protocol.String
		entry.ClientKey = clientKey.String
		entry.ClientName = clientName.String
		entry.Model = model.String
		entry.Status = int(status.Int64)
		entry.Duration = duration.Int64
		entry.TokenID = tokenID.String
		entry.TokenName = tokenName.String
		entry.InputTokens = int(inputTokens.Int64)
		entry.OutputTokens = int(outputTokens.Int64)
		entry.TotalTokens = int(totalTokens.Int64)
		entry.CooldownTriggered = cooldownTriggered.Int64 != 0
		if retryChain.String != "" {
			_ = json.Unmarshal([]byte(retryChain.String), &entry.RetryChain)
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func scanDailyUsage(rows *sql.Rows) ([]DailyUsage, error) {
	out := []DailyUsage{}
	for rows.Next() {
		var row DailyUsage
		var updatedAt string
		if err := rows.Scan(
			&row.Date,
			&row.Provider,
			&row.Protocol,
			&row.ClientKey,
			&row.ClientName,
			&row.Model,
			&row.RequestCount,
			&row.InputTokens,
			&row.OutputTokens,
			&row.TotalTokens,
			&updatedAt,
		); err != nil {
			return nil, err
		}
		if parsed, err := time.Parse(time.RFC3339Nano, updatedAt); err == nil {
			row.UpdatedAt = parsed
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func like(value string) string {
	return "%" + strings.TrimSpace(value) + "%"
}

func maxInt(value int, minimum int) int {
	if value < minimum {
		return minimum
	}
	return value
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
