import * as DesktopApp from '../../wailsjs/go/main/DesktopApp'

const API_BASE = import.meta.env.VITE_OMNIPROXY_API || 'http://127.0.0.1:3890/api'

function useWailsBindings() {
  return (
    !import.meta.env.VITE_OMNIPROXY_API &&
    typeof window !== 'undefined' &&
    Boolean(window.go?.main?.DesktopApp)
  )
}

async function request(path, options = {}) {
  const response = await fetch(`${API_BASE}${path}`, {
    headers: {
      'Content-Type': 'application/json',
      ...(options.headers || {}),
    },
    ...options,
  })

  if (response.status === 204) {
    return null
  }

  const data = await response.json().catch(() => null)
  if (!response.ok) {
    throw new Error(data?.error || `请求失败：${response.status}`)
  }
  return data
}

function historyFilter(filters = {}) {
  return {
    provider: filters.provider || 'all',
    level: filters.level || 'all',
    status: filters.status || 'all',
    model: filters.model || '',
    token: filters.token || '',
    search: filters.search || '',
    limit: Number(filters.limit || 5000),
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

export function validateToken(id) {
  if (useWailsBindings()) {
    return DesktopApp.ValidateToken(id)
  }
  return request(`/tokens/${id}/validate`, {
    method: 'POST',
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

function historyCSV(entries) {
  const header = [
    '时间',
    '级别',
    '方法',
    '路径',
    '路由厂商',
    '协议',
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

export function startProxy() {
  return useWailsBindings() ? DesktopApp.StartProxy() : request('/proxy/start', { method: 'POST' })
}

export function stopProxy() {
  return useWailsBindings() ? DesktopApp.StopProxy() : request('/proxy/stop', { method: 'POST' })
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
