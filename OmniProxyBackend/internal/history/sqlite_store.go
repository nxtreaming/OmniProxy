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
	stmt, err := tx.Prepare(insertHistorySQL)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, entry := range entries {
		if err := insertEntry(stmt, entry); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) Append(entry Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(insertHistorySQL, entryValues(entry)...)
	return err
}

func (s *SQLiteStore) List(filter Filter, limit int) ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 {
		limit = 5000
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
CREATE INDEX IF NOT EXISTS idx_request_history_provider ON request_history(provider);
CREATE INDEX IF NOT EXISTS idx_request_history_level ON request_history(level);
CREATE INDEX IF NOT EXISTS idx_request_history_status ON request_history(status);
CREATE INDEX IF NOT EXISTS idx_request_history_model ON request_history(model);
CREATE INDEX IF NOT EXISTS idx_request_history_token_name ON request_history(token_name);
CREATE INDEX IF NOT EXISTS idx_request_history_time ON request_history(time);
`)
	return err
}

const insertHistorySQL = `
INSERT OR REPLACE INTO request_history (
  id, time, level, method, path, provider, protocol, model, status, duration_ms,
  token_id, token_name, input_tokens, output_tokens, total_tokens, cooldown_triggered,
  retry_chain, message
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

func insertEntry(stmt *sql.Stmt, entry Entry) error {
	_, err := stmt.Exec(entryValues(entry)...)
	return err
}

func entryValues(entry Entry) []any {
	retryChain, _ := json.Marshal(entry.RetryChain)
	return []any{
		entry.ID,
		entry.Time.Format(time.RFC3339Nano),
		entry.Level,
		entry.Method,
		entry.Path,
		entry.Provider,
		entry.Protocol,
		entry.Model,
		entry.Status,
		entry.Duration,
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

func historyListQuery(filter Filter, limit int) (string, []any) {
	where := make([]string, 0, 6)
	args := make([]any, 0, 8)

	if filter.Provider != "" && filter.Provider != "all" {
		where = append(where, `provider = ? COLLATE NOCASE`)
		args = append(args, filter.Provider)
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
model LIKE ? COLLATE NOCASE OR
token_name LIKE ? COLLATE NOCASE OR
message LIKE ? COLLATE NOCASE OR
CAST(status AS TEXT) LIKE ?
)`)
		search := like(filter.Search)
		args = append(args, search, search, search, search, search, search, search, search)
	}

	var builder strings.Builder
	builder.WriteString(`SELECT id, time, level, method, path, provider, protocol, model, status, duration_ms, token_id, token_name, input_tokens, output_tokens, total_tokens, cooldown_triggered, retry_chain, message FROM request_history`)
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
		var cooldownTriggered int
		var retryChain string
		if err := rows.Scan(
			&entry.ID,
			&encodedTime,
			&entry.Level,
			&entry.Method,
			&entry.Path,
			&entry.Provider,
			&entry.Protocol,
			&entry.Model,
			&entry.Status,
			&entry.Duration,
			&entry.TokenID,
			&entry.TokenName,
			&entry.InputTokens,
			&entry.OutputTokens,
			&entry.TotalTokens,
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
		entry.CooldownTriggered = cooldownTriggered != 0
		if retryChain != "" {
			_ = json.Unmarshal([]byte(retryChain), &entry.RetryChain)
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func like(value string) string {
	return "%" + strings.TrimSpace(value) + "%"
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
