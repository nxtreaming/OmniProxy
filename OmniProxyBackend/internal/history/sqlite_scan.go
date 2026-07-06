package history

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "modernc.org/sqlite"
	"strings"
	"time"
)

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
			&row.TokenID,
			&row.TokenName,
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
