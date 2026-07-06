package history

import (
	"database/sql"
	_ "modernc.org/sqlite"
	"strings"
)

func (s *SQLiteStore) BillingSummary(days int) (BillingSummary, error) {
	days = normalizeSummaryDays(days)
	s.mu.Lock()
	defer s.mu.Unlock()

	out := BillingSummary{DailyRows: []BillingDailySummary{}}
	err := s.db.QueryRow(`
SELECT request_count, input_tokens, output_tokens, total_tokens
FROM billing_lifetime_summary
WHERE id = 1`).Scan(&out.RequestCount, &out.InputTokens, &out.OutputTokens, &out.TotalTokens)
	if err != nil && err != sql.ErrNoRows {
		return BillingSummary{}, err
	}
	if err == sql.ErrNoRows {
		if err := s.db.QueryRow(`
SELECT
  COALESCE(SUM(request_count), 0),
  COALESCE(SUM(input_tokens), 0),
  COALESCE(SUM(output_tokens), 0),
  COALESCE(SUM(total_tokens), 0)
FROM billing_daily_usage`).Scan(&out.RequestCount, &out.InputTokens, &out.OutputTokens, &out.TotalTokens); err != nil {
			return BillingSummary{}, err
		}
	}

	rows, err := s.db.Query(`
SELECT
  date,
  COALESCE(SUM(request_count), 0) AS request_count,
  COALESCE(SUM(input_tokens), 0) AS input_tokens,
  COALESCE(SUM(output_tokens), 0) AS output_tokens,
  COALESCE(SUM(total_tokens), 0) AS total_tokens
FROM billing_daily_usage
GROUP BY date
ORDER BY date DESC
LIMIT ?`, days)
	if err != nil {
		return BillingSummary{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var row BillingDailySummary
		if err := rows.Scan(&row.Date, &row.RequestCount, &row.InputTokens, &row.OutputTokens, &row.TotalTokens); err != nil {
			return BillingSummary{}, err
		}
		out.DailyRows = append(out.DailyRows, row)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) Summary(filter Filter, days int) (Summary, error) {
	days = normalizeSummaryDays(days)
	if strings.TrimSpace(filter.Search) != "" {
		return s.summaryFromHistory(filter, days)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	where, args := historyWhere(filter)
	whereSQL := historyWhereSQL(where)
	out := Summary{
		DailyRows:          []DailySummary{},
		ProviderRanks:      []Rank{},
		ClientRanks:        []Rank{},
		TokenRanks:         []Rank{},
		ModelRanks:         []Rank{},
		TokenFailureRanks:  []Rank{},
		FailureReasonRanks: []Rank{},
	}

	var durationSum int64
	if err := s.db.QueryRow(`
SELECT
  COALESCE(SUM(request_count), 0) AS total,
  COALESCE(SUM(failed_count), 0) AS failed,
  COALESCE(SUM(COALESCE(total_tokens, 0)), 0) AS total_tokens,
  COALESCE(SUM(COALESCE(duration_ms, 0)), 0) AS duration_ms
FROM request_daily_summary`+whereSQL, args...).Scan(&out.Total, &out.Failed, &out.TotalTokens, &durationSum); err != nil {
		return Summary{}, err
	}
	if out.Total > 0 {
		out.FailureRate = int((int64(out.Failed)*100 + int64(out.Total)/2) / int64(out.Total))
		out.AverageDuration = (durationSum + int64(out.Total)/2) / int64(out.Total)
	}

	dailyRows, err := s.summaryDailyRows(whereSQL, args, days)
	if err != nil {
		return Summary{}, err
	}
	out.DailyRows = dailyRows

	if out.ProviderRanks, err = s.summaryRanks(providerRankLabelSQL, where, args, "count", 0, false); err != nil {
		return Summary{}, err
	}
	if out.ClientRanks, err = s.summaryRanks(summaryClientRankLabelSQL, where, args, "count", 0, false); err != nil {
		return Summary{}, err
	}
	if out.TokenRanks, err = s.summaryRanks(tokenRankLabelSQL, where, args, "total_tokens", 0, false); err != nil {
		return Summary{}, err
	}
	if out.ModelRanks, err = s.summaryRanks(modelRankLabelSQL, where, args, "total_tokens", 0, false); err != nil {
		return Summary{}, err
	}

	failedWhere := append([]string{}, where...)
	failedWhere = append(failedWhere, `failed_count > 0`)
	if out.TokenFailureRanks, err = s.summaryRanks(tokenRankLabelSQL, failedWhere, args, "count", 50, true); err != nil {
		return Summary{}, err
	}
	historyWhere, historyArgs := historyWhere(filter)
	historyFailedWhere := append([]string{}, historyWhere...)
	historyFailedWhere = append(historyFailedWhere, failedHistorySQL)
	if out.FailureReasonRanks, err = s.summaryRanksFromHistory(failureReasonRankLabelSQL, historyFailedWhere, historyArgs, "count", 50); err != nil {
		return Summary{}, err
	}
	return out, nil
}

func (s *SQLiteStore) summaryDailyRows(whereSQL string, args []any, days int) ([]DailySummary, error) {
	rows, err := s.db.Query(`
SELECT
  date,
  COALESCE(SUM(request_count), 0) AS request_count,
  COALESCE(SUM(failed_count), 0) AS failed_count,
  COALESCE(SUM(COALESCE(total_tokens, 0)), 0) AS total_tokens
FROM request_daily_summary`+whereSQL+`
GROUP BY date
ORDER BY date`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byDate := map[string]*DailySummary{}
	for rows.Next() {
		var row DailySummary
		if err := rows.Scan(&row.Date, &row.RequestCount, &row.FailedCount, &row.TotalTokens); err != nil {
			return nil, err
		}
		byDate[row.Date] = &row
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return dailySummaryWindow(byDate, days), nil
}

func (s *SQLiteStore) summaryRanks(labelSQL string, where []string, args []any, mode string, limit int, failedOnly bool) ([]Rank, error) {
	orderBy := `count DESC, total_tokens DESC, label COLLATE NOCASE`
	if mode == "total_tokens" {
		orderBy = `total_tokens DESC, count DESC, label COLLATE NOCASE`
	}
	countSQL := `COALESCE(SUM(request_count), 0)`
	if failedOnly {
		countSQL = `COALESCE(SUM(failed_count), 0)`
	}
	query := `
SELECT
  ` + labelSQL + ` AS label,
  ` + countSQL + ` AS count,
  COALESCE(SUM(COALESCE(total_tokens, 0)), 0) AS total_tokens,
  COALESCE(SUM(failed_count), 0) AS failed_count
FROM request_daily_summary` + historyWhereSQL(where) + `
GROUP BY label
ORDER BY ` + orderBy
	queryArgs := append([]any{}, args...)
	if limit > 0 {
		query += ` LIMIT ?`
		queryArgs = append(queryArgs, limit)
	}

	rows, err := s.db.Query(query, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Rank{}
	for rows.Next() {
		var row Rank
		if err := rows.Scan(&row.Label, &row.Count, &row.TotalTokens, &row.FailedCount); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) summaryFromHistory(filter Filter, days int) (Summary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	where, args := historyWhere(filter)
	whereSQL := historyWhereSQL(where)
	out := Summary{
		DailyRows:          []DailySummary{},
		ProviderRanks:      []Rank{},
		ClientRanks:        []Rank{},
		TokenRanks:         []Rank{},
		ModelRanks:         []Rank{},
		TokenFailureRanks:  []Rank{},
		FailureReasonRanks: []Rank{},
	}

	var durationSum int64
	if err := s.db.QueryRow(`
SELECT
  COUNT(*) AS total,
  COALESCE(SUM(CASE WHEN `+failedHistorySQL+` THEN 1 ELSE 0 END), 0) AS failed,
  COALESCE(SUM(COALESCE(total_tokens, 0)), 0) AS total_tokens,
  COALESCE(SUM(COALESCE(duration_ms, 0)), 0) AS duration_ms
FROM request_history`+whereSQL, args...).Scan(&out.Total, &out.Failed, &out.TotalTokens, &durationSum); err != nil {
		return Summary{}, err
	}
	if out.Total > 0 {
		out.FailureRate = int((int64(out.Failed)*100 + int64(out.Total)/2) / int64(out.Total))
		out.AverageDuration = (durationSum + int64(out.Total)/2) / int64(out.Total)
	}

	dailyRows, err := s.summaryDailyRowsFromHistory(whereSQL, args, days)
	if err != nil {
		return Summary{}, err
	}
	out.DailyRows = dailyRows

	if out.ProviderRanks, err = s.summaryRanksFromHistory(providerRankLabelSQL, where, args, "count", 0); err != nil {
		return Summary{}, err
	}
	if out.ClientRanks, err = s.summaryRanksFromHistory(clientRankLabelSQL, where, args, "count", 0); err != nil {
		return Summary{}, err
	}
	if out.TokenRanks, err = s.summaryRanksFromHistory(tokenRankLabelSQL, where, args, "total_tokens", 0); err != nil {
		return Summary{}, err
	}
	if out.ModelRanks, err = s.summaryRanksFromHistory(modelRankLabelSQL, where, args, "total_tokens", 0); err != nil {
		return Summary{}, err
	}

	failedWhere := append([]string{}, where...)
	failedWhere = append(failedWhere, failedHistorySQL)
	if out.TokenFailureRanks, err = s.summaryRanksFromHistory(tokenRankLabelSQL, failedWhere, args, "count", 50); err != nil {
		return Summary{}, err
	}
	if out.FailureReasonRanks, err = s.summaryRanksFromHistory(failureReasonRankLabelSQL, failedWhere, args, "count", 50); err != nil {
		return Summary{}, err
	}
	return out, nil
}

func (s *SQLiteStore) summaryDailyRowsFromHistory(whereSQL string, args []any, days int) ([]DailySummary, error) {
	rows, err := s.db.Query(`
SELECT
  substr(time, 1, 10) AS date,
  COUNT(*) AS request_count,
  COALESCE(SUM(CASE WHEN `+failedHistorySQL+` THEN 1 ELSE 0 END), 0) AS failed_count,
  COALESCE(SUM(COALESCE(total_tokens, 0)), 0) AS total_tokens
FROM request_history`+whereSQL+`
GROUP BY date
ORDER BY date`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byDate := map[string]*DailySummary{}
	for rows.Next() {
		var row DailySummary
		if err := rows.Scan(&row.Date, &row.RequestCount, &row.FailedCount, &row.TotalTokens); err != nil {
			return nil, err
		}
		byDate[row.Date] = &row
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return dailySummaryWindow(byDate, days), nil
}

func (s *SQLiteStore) summaryRanksFromHistory(labelSQL string, where []string, args []any, mode string, limit int) ([]Rank, error) {
	orderBy := `count DESC, total_tokens DESC, label COLLATE NOCASE`
	if mode == "total_tokens" {
		orderBy = `total_tokens DESC, count DESC, label COLLATE NOCASE`
	}
	query := `
SELECT
  ` + labelSQL + ` AS label,
  COUNT(*) AS count,
  COALESCE(SUM(COALESCE(total_tokens, 0)), 0) AS total_tokens,
  COALESCE(SUM(CASE WHEN ` + failedHistorySQL + ` THEN 1 ELSE 0 END), 0) AS failed_count
FROM request_history` + historyWhereSQL(where) + `
GROUP BY label
ORDER BY ` + orderBy
	queryArgs := append([]any{}, args...)
	if limit > 0 {
		query += ` LIMIT ?`
		queryArgs = append(queryArgs, limit)
	}

	rows, err := s.db.Query(query, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Rank{}
	for rows.Next() {
		var row Rank
		if err := rows.Scan(&row.Label, &row.Count, &row.TotalTokens, &row.FailedCount); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
