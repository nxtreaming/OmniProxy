package history

import (
	_ "modernc.org/sqlite"
)

const insertHistorySQL = `
INSERT OR REPLACE INTO request_history (
  id, time, level, method, path, provider, protocol, model, status, duration_ms,
  client_key, client_name, token_id, token_name, input_tokens, output_tokens, total_tokens, cooldown_triggered,
  retry_chain, message
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const failedHistorySQL = `(LOWER(COALESCE(level, '')) IN ('error', 'warn') OR COALESCE(status, 0) >= 400)`

const providerRankLabelSQL = `COALESCE(NULLIF(provider, ''), '')`

const clientRankLabelSQL = `CASE
  WHEN UPPER(TRIM(COALESCE(method, ''))) = 'CHECK'
    OR LOWER(TRIM(COALESCE(protocol, ''))) = 'health-check'
    OR LOWER(TRIM(COALESCE(path, ''))) LIKE '%/maintenance/token-health-check%'
  THEN '__status_check__'
  ELSE COALESCE(NULLIF(client_name, ''), NULLIF(client_key, ''), '')
END`

const summaryClientRankLabelSQL = `COALESCE(NULLIF(client_label, ''), NULLIF(client_name, ''), NULLIF(client_key, ''), '')`

const modelRankLabelSQL = `COALESCE(NULLIF(model, ''), NULLIF(protocol, ''), '')`

const tokenRankLabelSQL = `COALESCE(NULLIF(token_name, ''), '')`

const failureStatusLabelSQL = `CASE
  WHEN COALESCE(status, 0) = 0 THEN '__no_status__'
  ELSE CAST(status AS TEXT)
END`

const failureReasonRankLabelSQL = `CASE
  WHEN TRIM(COALESCE(message, '')) = '' THEN ` + failureStatusLabelSQL + `
  ELSE ` + failureStatusLabelSQL + ` || '__reason_sep__' || message
END`

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

const upsertBillingLifetimeSQL = `
INSERT INTO billing_lifetime_summary (
  id, request_count, input_tokens, output_tokens, total_tokens, updated_at
) VALUES (1, 1, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  request_count = billing_lifetime_summary.request_count + excluded.request_count,
  input_tokens = billing_lifetime_summary.input_tokens + excluded.input_tokens,
  output_tokens = billing_lifetime_summary.output_tokens + excluded.output_tokens,
  total_tokens = billing_lifetime_summary.total_tokens + excluded.total_tokens,
  updated_at = excluded.updated_at`

const upsertRequestDailySummarySQL = `
INSERT INTO request_daily_summary (
  date, provider, protocol, client_key, client_name, client_label,
  level, status, model, token_name,
  request_count, failed_count, total_tokens, duration_ms, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?, ?, ?)
ON CONFLICT(date, provider, protocol, client_key, level, status, model, token_name) DO UPDATE SET
  client_name = CASE
    WHEN excluded.client_name != '' THEN excluded.client_name
    ELSE request_daily_summary.client_name
  END,
  client_label = CASE
    WHEN excluded.client_label != '' THEN excluded.client_label
    ELSE request_daily_summary.client_label
  END,
  request_count = request_daily_summary.request_count + excluded.request_count,
  failed_count = request_daily_summary.failed_count + excluded.failed_count,
  total_tokens = request_daily_summary.total_tokens + excluded.total_tokens,
  duration_ms = request_daily_summary.duration_ms + excluded.duration_ms,
  updated_at = excluded.updated_at`
