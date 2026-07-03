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
