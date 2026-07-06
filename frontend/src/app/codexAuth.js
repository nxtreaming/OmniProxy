export function codexEmailFromAuthJSON(text) {
  return codexIdentityFromAuthJSON(text).email
}

export function codexIdentityFromAuthJSON(text) {
  let data
  try {
    data = JSON.parse(text)
  } catch {
    throw new Error('不是有效 JSON')
  }

  const type = codexStringField(data?.type)
  if (type && type.toLowerCase() !== 'codex') {
    throw new Error('不是 Codex auth JSON')
  }
  if (!codexAuthSecretFromData(data)) {
    throw new Error('缺少可用的 access_token 或 id_token')
  }

  const directEmail = codexStringField(data?.email)
  const idToken = codexIDTokenFromData(data)
  const directAccountId = codexStringField(data?.tokens?.account_id) || codexStringField(data?.account_id)
  let payload = null
  if ((!directEmail || !directAccountId) && idToken) {
    const parts = idToken.split('.')
    if (parts.length !== 3) {
      if (!directEmail) throw new Error('id_token 格式不正确')
    } else {
      try {
        payload = JSON.parse(decodeBase64URL(parts[1]))
      } catch {
        if (!directEmail) throw new Error('无法解析 id_token')
      }
    }
  }

  const email = directEmail || payload?.['https://api.openai.com/profile']?.email || payload?.email
  if (typeof email !== 'string' || !email.trim()) {
    throw new Error('缺少 email，且无法从 id_token 解析邮箱')
  }

  const accountId = directAccountId ||
    payload?.['https://api.openai.com/auth']?.chatgpt_account_id ||
    payload?.account_id ||
    ''
  return {
    email: email.trim(),
    accountId: codexStringField(accountId),
  }
}

function codexIDTokenFromData(data) {
  return codexStringField(data?.tokens?.id_token) || codexStringField(data?.id_token)
}

function codexAuthSecretFromData(data) {
  return (
    codexStringField(data?.tokens?.access_token) ||
    codexStringField(data?.access_token) ||
    codexStringField(data?.OPENAI_API_KEY) ||
    codexIDTokenFromData(data)
  )
}

function codexStringField(value) {
  return typeof value === 'string' ? value.trim() : ''
}

function decodeBase64URL(value) {
  const normalized = value.replace(/-/g, '+').replace(/_/g, '/')
  const padded = normalized.padEnd(normalized.length + ((4 - (normalized.length % 4)) % 4), '=')
  const binary = globalThis.atob(padded)
  const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0))
  return new TextDecoder().decode(bytes)
}
