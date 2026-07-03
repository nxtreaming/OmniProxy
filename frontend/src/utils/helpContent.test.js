import assert from 'node:assert/strict'
import test from 'node:test'
import { buildThirdPartyEndpointGroups } from './helpContent.js'

test('buildThirdPartyEndpointGroups applies the local proxy port to all endpoint base URLs', () => {
  const groups = buildThirdPartyEndpointGroups(3001)
  const endpoints = groups.flatMap((group) => group.endpoints)

  assert.equal(groups.length, 4)
  assert.ok(endpoints.length > 20)
  assert.ok(endpoints.every((endpoint) => endpoint.baseUrl.startsWith('http://127.0.0.1:3001')))
})

test('buildThirdPartyEndpointGroups keeps key client routes stable', () => {
  const endpoints = buildThirdPartyEndpointGroups(3899).flatMap((group) => group.endpoints)

  assert.equal(endpoints.find((endpoint) => endpoint.name === 'OmniProxy OpenAI')?.baseUrl, 'http://127.0.0.1:3899/v1')
  assert.equal(
    endpoints.find((endpoint) => endpoint.name === 'Claude Router')?.baseUrl,
    'http://127.0.0.1:3899/anthropic-router',
  )
  assert.equal(endpoints.find((endpoint) => endpoint.name === 'Gemini Native')?.baseUrl, 'http://127.0.0.1:3899/gemini')
  assert.equal(
    endpoints.find((endpoint) => endpoint.name === 'Claude Desktop Gateway')?.apiKey,
    'omniproxy-claude-desktop',
  )
})
