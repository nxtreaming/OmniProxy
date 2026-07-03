package history

import (
	"database/sql"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	if _, err := tx.Exec(`DELETE FROM billing_lifetime_summary`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM request_daily_summary`); err != nil {
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
	lifetimeStmt, err := tx.Prepare(upsertBillingLifetimeSQL)
	if err != nil {
		return err
	}
	defer lifetimeStmt.Close()
	summaryStmt, err := tx.Prepare(upsertRequestDailySummarySQL)
	if err != nil {
		return err
	}
	defer summaryStmt.Close()

	for _, entry := range entries {
		if err := insertEntry(historyStmt, entry); err != nil {
			return err
		}
		if err := upsertDailyUsage(usageStmt, entry); err != nil {
			return err
		}
		if err := upsertBillingLifetime(lifetimeStmt, entry); err != nil {
			return err
		}
		if err := upsertRequestDailySummary(summaryStmt, entry); err != nil {
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
	if err := upsertBillingLifetimeTx(tx, entry); err != nil {
		return err
	}
	if err := upsertRequestDailySummaryTx(tx, entry); err != nil {
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
