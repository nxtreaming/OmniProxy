package history

import "strings"

func (s *SQLiteStore) RelabelTokenNames(names map[string]string) error {
	if len(names) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for id, name := range names {
		id = strings.TrimSpace(id)
		name = strings.TrimSpace(name)
		if id == "" || name == "" {
			continue
		}
		if _, err := tx.Exec(`UPDATE request_history SET token_name = ? WHERE token_id = ? AND COALESCE(token_name, '') != ?`, name, id, name); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE request_daily_summary SET token_name = ? WHERE token_id = ? AND COALESCE(token_name, '') != ?`, name, id, name); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE billing_daily_usage SET token_name = ? WHERE token_id = ? AND COALESCE(token_name, '') != ?`, name, id, name); err != nil {
			return err
		}
	}
	return tx.Commit()
}
