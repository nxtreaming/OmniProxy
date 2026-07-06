package history

import (
	_ "modernc.org/sqlite"
	"strconv"
	"strings"
)

func historyListQuery(filter Filter, limit int) (string, []any) {
	where, args := historyWhere(filter)

	var builder strings.Builder
	builder.WriteString(`SELECT id, time, level, method, path, provider, protocol, client_key, client_name, model, status, duration_ms, token_id, token_name, input_tokens, output_tokens, total_tokens, cooldown_triggered, retry_chain, message FROM request_history`)
	builder.WriteString(historyWhereSQL(where))
	builder.WriteString(` ORDER BY id DESC LIMIT ?`)
	args = append(args, limit)
	return builder.String(), args
}

func historyWhere(filter Filter) ([]string, []any) {
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
	if strings.TrimSpace(filter.TokenID) != "" && filter.TokenID != "all" {
		where = append(where, `token_id = ? COLLATE NOCASE`)
		args = append(args, strings.TrimSpace(filter.TokenID))
	}
	if strings.TrimSpace(filter.Token) != "" {
		where = append(where, `(token_name LIKE ? COLLATE NOCASE OR token_id LIKE ? COLLATE NOCASE)`)
		tokenSearch := like(filter.Token)
		args = append(args, tokenSearch, tokenSearch)
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
token_id LIKE ? COLLATE NOCASE OR
message LIKE ? COLLATE NOCASE OR
CAST(status AS TEXT) LIKE ?
)`)
		search := like(filter.Search)
		args = append(args, search, search, search, search, search, search, search, search, search, search, search)
	}

	return where, args
}

func historyWhereSQL(where []string) string {
	if len(where) > 0 {
		return ` WHERE ` + strings.Join(where, ` AND `)
	}
	return ``
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
