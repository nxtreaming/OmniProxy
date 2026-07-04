import { strict as assert } from 'node:assert'
import test from 'node:test'

function jsonResponse(status, payload) {
  return {
    ok: status >= 200 && status < 300,
    status,
    async json() {
      return payload
    },
  }
}

test('HTTP API fallback fetches and sends the control token', async () => {
  globalThis.__OMNIPROXY_CONTROL_TOKEN__ = 'test-control-token'
  const calls = []
  globalThis.fetch = async (url, options = {}) => {
    calls.push({ url: String(url), options })
    if (String(url).endsWith('/logs')) {
      assert.equal(options.headers['X-OmniProxy-Control-Token'], 'test-control-token')
      return jsonResponse(200, [])
    }
    throw new Error(`unexpected fetch: ${url}`)
  }

  const { getLogs } = await import(`./api.js?control-token-test=${Date.now()}`)
  assert.deepEqual(await getLogs(), [])
  assert.deepEqual(
    calls.map((call) => call.url),
    ['http://127.0.0.1:3890/api/logs'],
  )
  delete globalThis.__OMNIPROXY_CONTROL_TOKEN__
})

test('HTTP API fallback refreshes token auth through the control API', async () => {
  globalThis.__OMNIPROXY_CONTROL_TOKEN__ = 'refresh-control-token'
  const calls = []
  globalThis.fetch = async (url, options = {}) => {
    calls.push({ url: String(url), options })
    if (String(url).endsWith('/tokens/openai-token/refresh')) {
      assert.equal(options.method, 'POST')
      assert.equal(options.headers['X-OmniProxy-Control-Token'], 'refresh-control-token')
      return jsonResponse(200, { id: 'openai-token', name: 'coder@example.com' })
    }
    throw new Error(`unexpected fetch: ${url}`)
  }

  const { refreshTokenAuth } = await import(`./api.js?refresh-token-test=${Date.now()}`)
  assert.deepEqual(await refreshTokenAuth('openai-token'), {
    id: 'openai-token',
    name: 'coder@example.com',
  })
  assert.deepEqual(
    calls.map((call) => call.url),
    ['http://127.0.0.1:3890/api/tokens/openai-token/refresh'],
  )
  delete globalThis.__OMNIPROXY_CONTROL_TOKEN__
})

test('HTTP API fallback imports API keys through the control API', async () => {
  globalThis.__OMNIPROXY_CONTROL_TOKEN__ = 'import-control-token'
  const payload = { provider: 'openai', tokenText: 'sk-test-token-value' }
  const calls = []
  globalThis.fetch = async (url, options = {}) => {
    calls.push({ url: String(url), options })
    if (String(url).endsWith('/tokens/import-api-keys')) {
      assert.equal(options.method, 'POST')
      assert.equal(options.headers['X-OmniProxy-Control-Token'], 'import-control-token')
      assert.deepEqual(JSON.parse(options.body), payload)
      return jsonResponse(201, { createdCount: 1, skipped: [] })
    }
    throw new Error(`unexpected fetch: ${url}`)
  }

  const { importAPIKeys } = await import(`./api.js?import-api-keys-test=${Date.now()}`)
  assert.deepEqual(await importAPIKeys(payload), {
    createdCount: 1,
    skipped: [],
  })
  assert.deepEqual(
    calls.map((call) => call.url),
    ['http://127.0.0.1:3890/api/tokens/import-api-keys'],
  )
  delete globalThis.__OMNIPROXY_CONTROL_TOKEN__
})

test('HTTP API fallback fetches history summary with filters', async () => {
  globalThis.__OMNIPROXY_CONTROL_TOKEN__ = 'summary-control-token'
  const calls = []
  globalThis.fetch = async (url, options = {}) => {
    calls.push({ url: String(url), options })
    const parsed = new URL(String(url))
    if (parsed.pathname.endsWith('/history/summary')) {
      assert.equal(options.headers['X-OmniProxy-Control-Token'], 'summary-control-token')
      assert.equal(parsed.searchParams.get('provider'), 'openai')
      assert.equal(parsed.searchParams.get('model'), 'gpt-5.5')
      assert.equal(parsed.searchParams.get('days'), '14')
      return jsonResponse(200, { total: 42, dailyRows: [] })
    }
    throw new Error(`unexpected fetch: ${url}`)
  }

  const { getHistorySummary } = await import(`./api.js?history-summary-test=${Date.now()}`)
  assert.deepEqual(await getHistorySummary({ provider: 'openai', model: 'gpt-5.5' }, 14), {
    total: 42,
    dailyRows: [],
  })
  assert.deepEqual(
    calls.map((call) => new URL(call.url).pathname),
    ['/api/history/summary'],
  )
  delete globalThis.__OMNIPROXY_CONTROL_TOKEN__
})

test('HTTP API fallback fetches billing summary', async () => {
  globalThis.__OMNIPROXY_CONTROL_TOKEN__ = 'billing-summary-token'
  const calls = []
  globalThis.fetch = async (url, options = {}) => {
    calls.push({ url: String(url), options })
    const parsed = new URL(String(url))
    if (parsed.pathname.endsWith('/billing/summary')) {
      assert.equal(options.headers['X-OmniProxy-Control-Token'], 'billing-summary-token')
      assert.equal(parsed.searchParams.get('days'), '30')
      return jsonResponse(200, { requestCount: 3, totalTokens: 2048, dailyRows: [] })
    }
    throw new Error(`unexpected fetch: ${url}`)
  }

  const { getBillingSummary } = await import(`./api.js?billing-summary-test=${Date.now()}`)
  assert.deepEqual(await getBillingSummary(30), {
    requestCount: 3,
    totalTokens: 2048,
    dailyRows: [],
  })
  assert.deepEqual(
    calls.map((call) => new URL(call.url).pathname),
    ['/api/billing/summary'],
  )
  delete globalThis.__OMNIPROXY_CONTROL_TOKEN__
})

test('HTTP API fallback fetches update diagnostics', async () => {
  globalThis.__OMNIPROXY_CONTROL_TOKEN__ = 'update-diagnostics-token'
  const calls = []
  globalThis.fetch = async (url, options = {}) => {
    calls.push({ url: String(url), options })
    if (String(url).endsWith('/update/diagnostics')) {
      assert.equal(options.headers['X-OmniProxy-Control-Token'], 'update-diagnostics-token')
      return jsonResponse(200, {
        directory: 'C:\\Temp\\OmniProxy\\updates',
        status: { state: 'idle' },
        logTail: 'ready',
      })
    }
    throw new Error(`unexpected fetch: ${url}`)
  }

  const { getUpdateDiagnostics } = await import(`./api.js?update-diagnostics-test=${Date.now()}`)
  assert.deepEqual(await getUpdateDiagnostics(), {
    directory: 'C:\\Temp\\OmniProxy\\updates',
    status: { state: 'idle' },
    logTail: 'ready',
  })
  assert.deepEqual(
    calls.map((call) => call.url),
    ['http://127.0.0.1:3890/api/update/diagnostics'],
  )
  delete globalThis.__OMNIPROXY_CONTROL_TOKEN__
})
