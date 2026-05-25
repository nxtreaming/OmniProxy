import * as DesktopApp from '../../wailsjs/go/main/DesktopApp.js'

const API_BASE = import.meta.env?.VITE_OMNIPROXY_API || 'http://127.0.0.1:3890/api'
const STATIC_CONTROL_TOKEN =
  import.meta.env?.VITE_OMNIPROXY_CONTROL_TOKEN || globalThis.__OMNIPROXY_CONTROL_TOKEN__ || ''
const CONTROL_TOKEN_HEADER = 'X-OmniProxy-Control-Token'
let controlTokenPromise = null

function useWailsBindings() {
  return (
    !import.meta.env?.VITE_OMNIPROXY_API &&
    typeof window !== 'undefined' &&
    Boolean(window.go?.main?.DesktopApp)
  )
}

async function request(path, options = {}) {
  const headers = {
    'Content-Type': 'application/json',
    ...(options.headers || {}),
  }
  const token = await getHTTPControlToken()
  if (token) {
    headers[CONTROL_TOKEN_HEADER] = token
  }

  const response = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
  })

  if (response.status === 401 && token) {
    controlTokenPromise = null
    const retryToken = await getHTTPControlToken()
    if (retryToken && retryToken !== token) {
      return request(path, options)
    }
  }

  if (response.status === 204) {
    return null
  }

  const data = await response.json().catch(() => null)
  if (!response.ok) {
    throw new Error(data?.error || `请求失败：${response.status}`)
  }
  return data
}

async function getHTTPControlToken() {
  if (useWailsBindings()) {
    return ''
  }
  if (STATIC_CONTROL_TOKEN) {
    return STATIC_CONTROL_TOKEN
  }
  if (!controlTokenPromise) {
    controlTokenPromise = fetch(`${API_BASE}/control-token`, {
      headers: {
        Accept: 'application/json',
      },
    })
      .then(async (response) => {
        const data = await response.json().catch(() => null)
        if (!response.ok) {
          throw new Error(data?.error || `控制令牌获取失败：${response.status}`)
        }
        return data?.token || ''
      })
      .catch((error) => {
        controlTokenPromise = null
        throw error
      })
  }
  return controlTokenPromise
}

function historyFilter(filters = {}) {
  return {
    provider: filters.provider || 'all',
    client: filters.client || 'all',
    level: filters.level || 'all',
    status: filters.status || 'all',
    model: filters.model || '',
    token: filters.token || '',
    search: filters.search || '',
    limit: Number(filters.limit || 10000),
  }
}

function historyQuery(filters = {}) {
  const params = new URLSearchParams()
  const normalized = historyFilter(filters)
  Object.entries(normalized).forEach(([key, value]) => {
    if (value === '' || value === undefined || value === null) return
    params.set(key, String(value))
  })
  return params.toString()
}

export function getTokens() {
  return useWailsBindings() ? DesktopApp.Tokens() : request('/tokens')
}

export function createToken(payload) {
  if (useWailsBindings()) {
    return DesktopApp.CreateToken(payload)
  }
  return request('/tokens', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export function importAPIKeys(payload) {
  if (useWailsBindings() && DesktopApp.ImportAPIKeys) {
    return DesktopApp.ImportAPIKeys(payload)
  }
  return request('/tokens/import-api-keys', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export function updateToken(id, payload) {
  if (useWailsBindings()) {
    return DesktopApp.UpdateToken(id, payload)
  }
  return request(`/tokens/${id}`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  })
}

export function deleteToken(id) {
  if (useWailsBindings()) {
    return DesktopApp.DeleteToken(id)
  }
  return request(`/tokens/${id}`, {
    method: 'DELETE',
  })
}

export function setTokenDisabled(id, disabled) {
  if (useWailsBindings()) {
    return DesktopApp.SetTokenDisabled(id, disabled)
  }
  return request(`/tokens/${id}/disabled`, {
    method: 'PUT',
    body: JSON.stringify({ disabled }),
  })
}

export function useOnlyToken(id) {
  if (useWailsBindings()) {
    return DesktopApp.UseOnlyToken(id)
  }
  return request(`/tokens/${id}/exclusive`, {
    method: 'PUT',
  })
}

export function cancelUseOnlyToken(id) {
  if (useWailsBindings()) {
    return DesktopApp.CancelUseOnlyToken(id)
  }
  return request(`/tokens/${id}/exclusive`, {
    method: 'DELETE',
  })
}

export function setTokenSelected(id, selected) {
  if (useWailsBindings()) {
    return DesktopApp.SetTokenSelected(id, selected)
  }
  return request(`/tokens/${id}/selected`, {
    method: 'PUT',
    body: JSON.stringify({ selected }),
  })
}

export function validateToken(id) {
  if (useWailsBindings()) {
    return DesktopApp.ValidateToken(id)
  }
  return request(`/tokens/${id}/validate`, {
    method: 'POST',
  })
}

export function refreshTokenAuth(id) {
  if (useWailsBindings() && DesktopApp.RefreshTokenAuth) {
    return DesktopApp.RefreshTokenAuth(id)
  }
  return request(`/tokens/${id}/refresh`, {
    method: 'POST',
  })
}

export function getOpenRouterModels(refresh = false) {
  if (useWailsBindings() && DesktopApp.OpenRouterModels) {
    return DesktopApp.OpenRouterModels(Boolean(refresh))
  }
  const params = new URLSearchParams()
  if (refresh) params.set('refresh', 'true')
  return request(`/openrouter/models?${params.toString()}`)
}

export function sendOpenRouterChat(payload) {
  if (useWailsBindings() && DesktopApp.OpenRouterChat) {
    return DesktopApp.OpenRouterChat(payload)
  }
  return request('/openrouter/chat', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export function getConfig() {
  return useWailsBindings() ? DesktopApp.Config() : request('/config')
}

export function saveConfig(payload) {
  if (useWailsBindings()) {
    return DesktopApp.SaveConfig(payload)
  }
  return request('/config', {
    method: 'PUT',
    body: JSON.stringify(payload),
  })
}

export function getLogs() {
  return useWailsBindings() ? DesktopApp.Logs() : request('/logs')
}

export function getHistory(filters = {}) {
  if (useWailsBindings() && DesktopApp.RequestHistory) {
    return DesktopApp.RequestHistory(historyFilter(filters))
  }
  return request(`/history?${historyQuery(filters)}`)
}

export function getHistorySummary(filters = {}, days = 14) {
  const normalizedDays = Number(days || 14)
  if (useWailsBindings() && DesktopApp.RequestHistorySummary) {
    return DesktopApp.RequestHistorySummary(historyFilter(filters), normalizedDays)
  }
  const params = new URLSearchParams(historyQuery(filters))
  params.set('days', String(normalizedDays))
  return request(`/history/summary?${params.toString()}`)
}

export function getBillingUsage(date) {
  const value = String(date || '').trim()
  if (useWailsBindings() && DesktopApp.BillingUsage) {
    return DesktopApp.BillingUsage(value)
  }
  const params = new URLSearchParams()
  if (value) params.set('date', value)
  return request(`/billing/usage?${params.toString()}`)
}

export function getBillingDates(limit = 30) {
  const normalizedLimit = Number(limit || 30)
  if (useWailsBindings() && DesktopApp.BillingDates) {
    return DesktopApp.BillingDates(normalizedLimit)
  }
  const params = new URLSearchParams()
  params.set('limit', String(normalizedLimit))
  return request(`/billing/dates?${params.toString()}`)
}

export function getBillingSummary(days = 30) {
  const normalizedDays = Number(days || 30)
  if (useWailsBindings() && DesktopApp.BillingSummary) {
    return DesktopApp.BillingSummary(normalizedDays)
  }
  const params = new URLSearchParams()
  params.set('days', String(normalizedDays))
  return request(`/billing/summary?${params.toString()}`)
}

export function clearBillingUsage() {
  if (useWailsBindings() && DesktopApp.ClearBillingUsage) {
    return DesktopApp.ClearBillingUsage()
  }
  return request('/billing/clear', {
    method: 'DELETE',
  })
}

export function clearRequestHistory() {
  if (useWailsBindings() && DesktopApp.ClearRequestHistory) {
    return DesktopApp.ClearRequestHistory()
  }
  return request('/history/clear', {
    method: 'DELETE',
  })
}

export function getDataDirectory() {
  if (useWailsBindings() && DesktopApp.DataDirectory) {
    return DesktopApp.DataDirectory()
  }
  return request('/data-directory')
}

export function chooseDataDirectory(migrate = true) {
  if (useWailsBindings() && DesktopApp.ChooseDataDirectory) {
    return DesktopApp.ChooseDataDirectory(Boolean(migrate))
  }
  return Promise.reject(new Error('更改数据目录需要在桌面客户端中操作'))
}

export function getTaskAutomationBrowserProfiles(browser = 'default') {
  if (useWailsBindings() && DesktopApp.TaskAutomationBrowserProfiles) {
    return DesktopApp.TaskAutomationBrowserProfiles(String(browser || 'default'))
  }
  return Promise.resolve([])
}

export async function exportHistory(format, filters = {}, entries = []) {
  if (useWailsBindings() && DesktopApp.ExportRequestHistory) {
    return DesktopApp.ExportRequestHistory(format, historyFilter(filters))
  }
  const data = format === 'json' ? JSON.stringify(entries, null, 2) : historyCSV(entries)
  const type = format === 'json' ? 'application/json' : 'text/csv;charset=utf-8'
  const blob = new Blob([format === 'csv' ? '\uFEFF' : '', data], { type })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `omniproxy-request-history-${new Date().toISOString().replace(/[:.]/g, '-')}.${format}`
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
  return link.download
}

export function exportTokens() {
  return useWailsBindings() && DesktopApp.ExportTokens
    ? DesktopApp.ExportTokens()
    : Promise.reject(new Error('导出账号池需要在桌面客户端中操作'))
}

export function exportCodexAuthFiles() {
  return useWailsBindings() && DesktopApp.ExportCodexAuthFiles
    ? DesktopApp.ExportCodexAuthFiles()
    : Promise.reject(new Error('导出 Codex auth 文件需要在桌面客户端中操作'))
}

function historyCSV(entries) {
  const header = [
    '时间',
    '级别',
    '方法',
    '路径',
    '路由厂商',
    '协议',
    '编程工具',
    '模型',
    '状态码',
    '耗时(ms)',
    '账号',
    '输入Token',
    '输出Token',
    '总Token',
    '触发冷却',
    '错误摘要',
    '重试链路',
  ]
  const rows = entries.map((entry) => [
    entry.time || '',
    entry.level || '',
    entry.method || '',
    entry.path || '',
    entry.provider || '',
    entry.protocol || '',
    entry.clientName || entry.clientKey || '',
    entry.model || '',
    entry.status || '',
    entry.durationMs || 0,
    entry.tokenName || '',
    entry.inputTokens || 0,
    entry.outputTokens || 0,
    entry.totalTokens || 0,
    entry.cooldownTriggered ? '是' : '否',
    entry.message || '',
    retryChainText(entry.retryChain || []),
  ])
  return [header, ...rows].map((row) => row.map(csvCell).join(',')).join('\n')
}

function retryChainText(chain) {
  return chain
    .map((attempt) => {
      const parts = [`#${attempt.attempt || '-'}`, attempt.provider || '-', attempt.tokenName || '-']
      if (attempt.status) parts.push(String(attempt.status))
      parts.push(`${attempt.durationMs || 0}ms`)
      if (attempt.cooldownTriggered) parts.push('冷却')
      if (attempt.message) parts.push(attempt.message)
      return parts.join(' ')
    })
    .join(' | ')
}

function csvCell(value) {
  const text = String(value ?? '')
  if (/[",\n\r]/.test(text)) {
    return `"${text.replace(/"/g, '""')}"`
  }
  return text
}

export function getProxyStatus() {
  return useWailsBindings() ? DesktopApp.ProxyStatus() : request('/proxy/status')
}

export function getActiveRequests() {
  return useWailsBindings() && DesktopApp.ActiveProxyRequests
    ? DesktopApp.ActiveProxyRequests()
    : request('/proxy/active-requests')
}

export function startProxy() {
  return useWailsBindings() ? DesktopApp.StartProxy() : request('/proxy/start', { method: 'POST' })
}

export function stopProxy() {
  return useWailsBindings() ? DesktopApp.StopProxy() : request('/proxy/stop', { method: 'POST' })
}

export function checkForUpdates() {
  return useWailsBindings() && DesktopApp.CheckForUpdates
    ? DesktopApp.CheckForUpdates()
    : request('/update/check')
}

export function downloadUpdate(payload) {
  const desktopApp = typeof window !== 'undefined' ? window.go?.main?.DesktopApp : null
  return useWailsBindings() && desktopApp?.DownloadUpdate
    ? desktopApp.DownloadUpdate(payload)
    : request('/update/download', { method: 'POST', body: JSON.stringify(payload) })
}

export function getUpdateDownloadStatus() {
  const desktopApp = typeof window !== 'undefined' ? window.go?.main?.DesktopApp : null
  return useWailsBindings() && desktopApp?.UpdateDownloadStatus
    ? desktopApp.UpdateDownloadStatus()
    : request('/update/download/status')
}

export function installDownloadedUpdate() {
  const desktopApp = typeof window !== 'undefined' ? window.go?.main?.DesktopApp : null
  return useWailsBindings() && desktopApp?.InstallDownloadedUpdate
    ? desktopApp.InstallDownloadedUpdate()
    : request('/update/install', { method: 'POST' })
}

export function getAppInfo() {
  return useWailsBindings() && DesktopApp.AppInfo
    ? DesktopApp.AppInfo()
    : request('/app/info')
}

export function openExternalURL(url) {
  if (!url) return
  if (useWailsBindings() && window.runtime?.BrowserOpenURL) {
    window.runtime.BrowserOpenURL(url)
    return
  }
  window.open(url, '_blank', 'noopener,noreferrer')
}

export function getAutoStartStatus() {
  return useWailsBindings() && DesktopApp.AutoStartStatus
    ? DesktopApp.AutoStartStatus()
    : Promise.resolve({ enabled: false })
}

export function setAutoStart(enabled) {
  return useWailsBindings() && DesktopApp.SetAutoStart
    ? DesktopApp.SetAutoStart(enabled)
    : Promise.resolve({ enabled: false })
}

export function configureCodex() {
  return useWailsBindings() ? DesktopApp.ConfigureCodex() : request('/codex/configure', { method: 'POST' })
}

export function configureCodexSub2API() {
  return useWailsBindings()
    ? DesktopApp.ConfigureCodexSub2API()
    : request('/codex/sub2api/configure', { method: 'POST' })
}

export function configureCodexZo() {
  return useWailsBindings() && DesktopApp.ConfigureCodexZo
    ? DesktopApp.ConfigureCodexZo()
    : request('/codex/zo/configure', { method: 'POST' })
}

export function restoreCodex() {
  return useWailsBindings() ? DesktopApp.RestoreCodex() : request('/codex/restore', { method: 'POST' })
}

export function configureMimoClaude() {
  return useWailsBindings() ? DesktopApp.ConfigureMimoClaude() : request('/mimo/claude/configure', { method: 'POST' })
}

export function restoreMimoClaude() {
  return useWailsBindings() ? DesktopApp.RestoreMimoClaude() : request('/mimo/claude/restore', { method: 'POST' })
}

export function configureDeepSeekClaude() {
  return useWailsBindings()
    ? DesktopApp.ConfigureDeepSeekClaude()
    : request('/deepseek/claude/configure', { method: 'POST' })
}

export function restoreDeepSeekClaude() {
  return useWailsBindings()
    ? DesktopApp.RestoreDeepSeekClaude()
    : request('/deepseek/claude/restore', { method: 'POST' })
}

export function configureKimiClaude() {
  return useWailsBindings()
    ? DesktopApp.ConfigureKimiClaude()
    : request('/kimi/claude/configure', { method: 'POST' })
}

export function restoreKimiClaude() {
  return useWailsBindings()
    ? DesktopApp.RestoreKimiClaude()
    : request('/kimi/claude/restore', { method: 'POST' })
}

export function configureZhipuClaude() {
  return useWailsBindings()
    ? DesktopApp.ConfigureZhipuClaude()
    : request('/zhipu/claude/configure', { method: 'POST' })
}

export function configureZoClaude() {
  return useWailsBindings() && DesktopApp.ConfigureZoClaude
    ? DesktopApp.ConfigureZoClaude()
    : request('/zo/claude/configure', { method: 'POST' })
}

export function configureClaudeModels(models) {
  const payload = { models: Array.isArray(models) ? models : [] }
  return useWailsBindings() && DesktopApp.ConfigureClaudeModels
    ? DesktopApp.ConfigureClaudeModels(payload)
    : request('/claude/models/configure', { method: 'POST', body: JSON.stringify(payload) })
}

export function configureClaudeDesktopModels(models) {
  const payload = { models: Array.isArray(models) ? models : [] }
  return useWailsBindings() && DesktopApp.ConfigureClaudeDesktopModels
    ? DesktopApp.ConfigureClaudeDesktopModels(payload)
    : request('/claude/desktop/models/configure', { method: 'POST', body: JSON.stringify(payload) })
}

export function restoreClaudeDesktop() {
  return useWailsBindings() && DesktopApp.RestoreClaudeDesktop
    ? DesktopApp.RestoreClaudeDesktop()
    : request('/claude/desktop/restore', { method: 'POST' })
}

export function restoreZhipuClaude() {
  return useWailsBindings()
    ? DesktopApp.RestoreZhipuClaude()
    : request('/zhipu/claude/restore', { method: 'POST' })
}

export function configureDeepSeekTUI() {
  return useWailsBindings()
    ? DesktopApp.ConfigureDeepSeekTUI()
    : request('/deepseek-tui/configure', { method: 'POST' })
}

export function restoreDeepSeekTUI() {
  return useWailsBindings()
    ? DesktopApp.RestoreDeepSeekTUI()
    : request('/deepseek-tui/restore', { method: 'POST' })
}

export function configureGemini() {
  return useWailsBindings()
    ? DesktopApp.ConfigureGemini()
    : request('/gemini/configure', { method: 'POST' })
}

export function restoreGemini() {
  return useWailsBindings()
    ? DesktopApp.RestoreGemini()
    : request('/gemini/restore', { method: 'POST' })
}

export function configureOpenCode() {
  return useWailsBindings()
    ? DesktopApp.ConfigureOpenCode()
    : request('/opencode/configure', { method: 'POST' })
}

export function restoreOpenCode() {
  return useWailsBindings()
    ? DesktopApp.RestoreOpenCode()
    : request('/opencode/restore', { method: 'POST' })
}

export function configurePi() {
  return useWailsBindings()
    ? DesktopApp.ConfigurePi()
    : request('/pi/configure', { method: 'POST' })
}

export function restorePi() {
  return useWailsBindings()
    ? DesktopApp.RestorePi()
    : request('/pi/restore', { method: 'POST' })
}
