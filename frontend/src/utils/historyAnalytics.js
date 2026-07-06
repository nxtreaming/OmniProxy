export function normalizeHistorySummary(summary) {
  if (!summary) return null
  const total = Number(summary.total || 0)
  const failed = Number(summary.failed || 0)
  const totalTokens = Number(summary.totalTokens || 0)
  const averageDuration = Number(summary.averageDuration || 0)
  return {
    total,
    failed,
    failureRate: total ? Math.round((failed / total) * 100) : 0,
    totalTokens,
    averageDuration,
  }
}

export function normalizeDailyRows(rows) {
  return [...(rows || [])].map((row) => ({
    date: row.date,
    requestCount: Number(row.requestCount || 0),
    failedCount: Number(row.failedCount || 0),
    totalTokens: Number(row.totalTokens || 0),
  }))
}

export function normalizeRanks(rows, fallbackLabel, labelFn = (value) => value) {
  return [...(rows || [])].map((row) => {
    const rawLabel = String(row.label || '').trim()
    return {
      label: labelFn(rawLabel || fallbackLabel),
      count: Number(row.count || 0),
      totalTokens: Number(row.totalTokens || 0),
      failedCount: Number(row.failedCount || 0),
    }
  })
}

export function normalizeClientRankLabel(label) {
  return label === '__status_check__' ? '状态检查' : label
}

export function normalizeFailureReasonLabel(label) {
  return label
    .replaceAll('__no_status__', '无状态码')
    .replaceAll('__reason_sep__', ' · ')
}

export function filterHistory(items, currentFilters) {
  const search = currentFilters.search.trim().toLowerCase()
  const model = currentFilters.model.trim().toLowerCase()
  const tokenName = currentFilters.token.trim().toLowerCase()
  const tokenId = String(currentFilters.tokenId || 'all').trim()
  return items
    .filter((item) => currentFilters.provider === 'all' || item.provider === currentFilters.provider)
    .filter((item) => currentFilters.client === 'all' || item.clientKey === currentFilters.client)
    .filter((item) => currentFilters.level === 'all' || item.level === currentFilters.level)
    .filter((item) => currentFilters.status === 'all' || historyStatusMatches(item, currentFilters.status))
    .filter((item) => !model || String(item.model || '').toLowerCase().includes(model))
    .filter((item) => tokenId === 'all' || String(item.tokenId || '') === tokenId)
    .filter((item) => !tokenName || `${item.tokenName || ''} ${item.tokenId || ''}`.toLowerCase().includes(tokenName))
    .filter((item) => {
      if (!search) return true
      return [
        item.method,
        item.path,
        item.provider,
        item.protocol,
        item.clientKey,
        item.clientName,
        item.model,
        item.tokenId,
        item.tokenName,
        item.message,
        String(item.status || ''),
      ]
        .join(' ')
        .toLowerCase()
        .includes(search)
    })
}

export function summarizeHistory(items) {
  const total = items.length
  const failed = items.filter(isFailedHistory).length
  const totalTokens = items.reduce((sum, item) => sum + Number(item.totalTokens || 0), 0)
  const totalDuration = items.reduce((sum, item) => sum + Number(item.durationMs || 0), 0)
  return {
    total,
    failed,
    failureRate: total ? Math.round((failed / total) * 100) : 0,
    totalTokens,
    averageDuration: total ? Math.round(totalDuration / total) : 0,
  }
}

export function aggregateHistoryByDay(items) {
  const byDay = new Map()
  for (const entry of items) {
    const day = String(entry.time || '').slice(0, 10) || 'unknown'
    const current = byDay.get(day) || { date: day, requestCount: 0, failedCount: 0, totalTokens: 0 }
    current.requestCount += 1
    if (isFailedHistory(entry)) current.failedCount += 1
    current.totalTokens += Number(entry.totalTokens || 0)
    byDay.set(day, current)
  }
  return [...byDay.values()].sort((a, b) => a.date.localeCompare(b.date))
}

export function buildHistoryDailyWindow(rows, days) {
  if (!rows.length) return []

  const byDay = new Map(rows.map((row) => [row.date, row]))
  const endDate = parseDateKey(rows[rows.length - 1]?.date) || new Date()
  const startDate = addDays(endDate, -(days - 1))

  return Array.from({ length: days }, (_, index) => {
    const date = formatDateKey(addDays(startDate, index))
    return byDay.get(date) || { date, requestCount: 0, failedCount: 0, totalTokens: 0 }
  })
}

export function rankHistory(items, labelFn, mode) {
  const groups = new Map()
  for (const entry of items) {
    const label = labelFn(entry)
    const current = groups.get(label) || { label, count: 0, totalTokens: 0, failedCount: 0 }
    current.count += 1
    current.totalTokens += Number(entry.totalTokens || 0)
    if (isFailedHistory(entry)) current.failedCount += 1
    groups.set(label, current)
  }
  const metric = mode === 'totalTokens' ? 'totalTokens' : 'count'
  return [...groups.values()].sort((a, b) => b[metric] - a[metric] || b.count - a.count)
}

export function historyClientOptions(items) {
  const byKey = new Map()
  for (const entry of items || []) {
    const key = String(entry.clientKey || '').trim()
    if (!key) continue
    if (!byKey.has(key)) {
      byKey.set(key, clientLabel(entry))
    }
  }
  return [...byKey.entries()]
    .map(([key, label]) => ({ key, label }))
    .sort((a, b) => a.label.localeCompare(b.label))
}

export function historyWorkspaceOptions(items) {
  const byID = new Map()
  for (const entry of items || []) {
    const key = String(entry.tokenId || '').trim()
    if (!key || byID.has(key)) continue
    byID.set(key, historyWorkspaceLabel(entry))
  }
  return [...byID.entries()]
    .map(([key, label]) => ({ key, label }))
    .sort((a, b) => a.label.localeCompare(b.label))
}

export function historyWorkspaceLabel(entry) {
  return entry?.tokenName || entry?.tokenId || '未记录账号'
}

export function clientLabel(entry) {
  if (isStatusCheckEntry(entry)) return '状态检查'
  return entry?.clientName || entry?.clientKey || '未记录工具'
}

export function buildModelPieSegments(rows, colors) {
  const ranked = rows.filter((item) => Number(item.totalTokens || 0) > 0)
  const top = ranked.slice(0, 5)
  const rest = ranked.slice(5)
  const restTotal = rest.reduce((sum, item) => sum + Number(item.totalTokens || 0), 0)
  const restCount = rest.reduce((sum, item) => sum + Number(item.count || 0), 0)
  const segments = restTotal > 0
    ? [...top, { label: '其他模型', count: restCount, totalTokens: restTotal, failedCount: 0 }]
    : top
  const total = segments.reduce((sum, item) => sum + Number(item.totalTokens || 0), 0)

  return segments.map((item, index) => ({
    ...item,
    color: colors[index % colors.length],
    percent: total ? Math.round((Number(item.totalTokens || 0) / total) * 1000) / 10 : 0,
  }))
}

export function failureReasonLabel(entry) {
  const status = entry.status ? `${entry.status}` : '无状态码'
  const message = String(entry.message || '').trim()
  if (!message) return status
  return `${status} · ${message}`
}

export function historyStatusMatches(entry, status) {
  if (status === 'success') {
    return entry.status >= 200 && entry.status < 400
  }
  if (status === 'error') {
    return !entry.status || entry.status >= 400
  }
  return String(entry.status || '') === status
}

export function isFailedHistory(entry) {
  return entry?.level === 'error' || entry?.level === 'warn' || Number(entry?.status || 0) >= 400
}

function isStatusCheckEntry(entry) {
  return (
    entry?.method === 'CHECK' ||
    entry?.protocol === 'health-check' ||
    String(entry?.path || '').includes('/maintenance/token-health-check')
  )
}

function parseDateKey(value) {
  const match = String(value || '').match(/^(\d{4})-(\d{2})-(\d{2})$/)
  if (!match) return null
  return new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]))
}

function addDays(date, amount) {
  const next = new Date(date)
  next.setDate(next.getDate() + amount)
  return next
}

function formatDateKey(date) {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}
