const API_BASE = import.meta.env.VITE_OMNIPROXY_API || 'http://127.0.0.1:3890/api'

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

export function getTokens() {
  return request('/tokens')
}

export function createToken(payload) {
  return request('/tokens', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export function updateToken(id, payload) {
  return request(`/tokens/${id}`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  })
}

export function deleteToken(id) {
  return request(`/tokens/${id}`, {
    method: 'DELETE',
  })
}

export function validateToken(id) {
  return request(`/tokens/${id}/validate`, {
    method: 'POST',
  })
}

export function getConfig() {
  return request('/config')
}

export function saveConfig(payload) {
  return request('/config', {
    method: 'PUT',
    body: JSON.stringify(payload),
  })
}

export function getLogs() {
  return request('/logs')
}

export function getProxyStatus() {
  return request('/proxy/status')
}

export function startProxy() {
  return request('/proxy/start', { method: 'POST' })
}

export function stopProxy() {
  return request('/proxy/stop', { method: 'POST' })
}

export function configureCodex() {
  return request('/codex/configure', { method: 'POST' })
}

export function restoreCodex() {
  return request('/codex/restore', { method: 'POST' })
}

export function configureMimoClaude() {
  return request('/mimo/claude/configure', { method: 'POST' })
}

export function restoreMimoClaude() {
  return request('/mimo/claude/restore', { method: 'POST' })
}

export function configureDeepSeekClaude() {
  return request('/deepseek/claude/configure', { method: 'POST' })
}

export function restoreDeepSeekClaude() {
  return request('/deepseek/claude/restore', { method: 'POST' })
}

export function configureKimiClaude() {
  return request('/kimi/claude/configure', { method: 'POST' })
}

export function restoreKimiClaude() {
  return request('/kimi/claude/restore', { method: 'POST' })
}
