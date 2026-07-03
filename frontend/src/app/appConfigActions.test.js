import { strict as assert } from 'node:assert'
import test from 'node:test'

import { gatewayRoutesPayload } from './appConfigActions.js'

test('gatewayRoutesPayload preserves ordered fallback route settings', () => {
  const payload = gatewayRoutesPayload({
    openai: {
      provider: ' openai ',
      credentialType: ' api_key ',
      model: ' gpt-5.4 ',
      fallbacks: [
        { provider: ' deepseek ', credentialType: '', model: ' deepseek-v4-pro[1m] ' },
        { provider: ' zhipu ', credentialType: ' coding_plan ', model: '' },
        { provider: ' ', credentialType: 'api_key', model: 'ignored' },
      ],
    },
  })

  assert.deepEqual(payload.openai, {
    provider: 'openai',
    credentialType: 'api_key',
    model: 'gpt-5.4',
    fallbacks: [
      { provider: 'deepseek', credentialType: '', model: 'deepseek-v4-pro[1m]' },
      { provider: 'zhipu', credentialType: 'coding_plan', model: '' },
    ],
  })
})

test('gatewayRoutesPayload keeps legacy routes compatible when fallbacks are omitted', () => {
  const payload = gatewayRoutesPayload({
    openai: {
      provider: ' deepseek ',
      model: ' deepseek-v4-pro[1m] ',
    },
  })

  assert.deepEqual(payload.openai, {
    provider: 'deepseek',
    credentialType: '',
    model: 'deepseek-v4-pro[1m]',
    fallbacks: [],
  })
})

test('gatewayRoutesPayload skips malformed fallback entries', () => {
  const payload = gatewayRoutesPayload({
    openai: {
      provider: 'openai',
      fallbacks: [
        null,
        'deepseek',
        { provider: ' kimi ', credentialType: ' api_key ', model: ' kimi-for-coding ' },
      ],
    },
  })

  assert.deepEqual(payload.openai.fallbacks, [
    { provider: 'kimi', credentialType: 'api_key', model: 'kimi-for-coding' },
  ])
})
