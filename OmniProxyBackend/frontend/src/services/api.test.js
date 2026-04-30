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
