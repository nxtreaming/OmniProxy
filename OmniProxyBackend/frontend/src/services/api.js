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

export function getProxyStatus() {
  return useWailsBindings() ? DesktopApp.ProxyStatus() : request('/proxy/status')
}

export function startProxy() {
  return useWailsBindings() ? DesktopApp.StartProxy() : request('/proxy/start', { method: 'POST' })
}

export function stopProxy() {
  return useWailsBindings() ? DesktopApp.StopProxy() : request('/proxy/stop', { method: 'POST' })
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
