import { strict as assert } from 'node:assert'
import test from 'node:test'

import { applyLoadedConfig } from './appDataMerge.js'

test('applyLoadedConfig preserves unsaved gateway route edits while refreshing other config', () => {
  const config = {
    proxyPort: 3000,
    gatewayRoutes: {
      codex: { provider: 'openai', credentialType: 'codex_auth_json', model: 'gpt-5.5' },
    },
    modelRoutes: {
      'deepseek-v4-pro': { provider: 'deepseek', credentialType: '', model: 'deepseek-v4-pro' },
    },
  }

  applyLoadedConfig(config, {
    proxyPort: 3100,
    gatewayRoutes: {
      codex: { provider: 'openai', credentialType: 'api_key', model: 'gpt-5.4' },
    },
    modelRoutes: {
      'deepseek-v4-pro': { provider: 'prem', credentialType: 'api_key', model: 'deepseek-v4-pro' },
    },
  }, true)

  assert.equal(config.proxyPort, 3100)
  assert.deepEqual(config.gatewayRoutes, {
    codex: { provider: 'openai', credentialType: 'codex_auth_json', model: 'gpt-5.5' },
  })
  assert.deepEqual(config.modelRoutes, {
    'deepseek-v4-pro': { provider: 'deepseek', credentialType: '', model: 'deepseek-v4-pro' },
  })
})

test('applyLoadedConfig replaces gateway routes when there are no unsaved edits', () => {
  const config = {
    gatewayRoutes: {
      codex: { provider: 'openai', credentialType: 'codex_auth_json', model: 'gpt-5.5' },
    },
  }

  applyLoadedConfig(config, {
    gatewayRoutes: {
      codex: { provider: 'openai', credentialType: '', model: 'gpt-5.5' },
    },
  })

  assert.deepEqual(config.gatewayRoutes, {
    codex: { provider: 'openai', credentialType: '', model: 'gpt-5.5' },
  })
})

test('applyLoadedConfig keeps loaded routes when dirty state has no local route draft', () => {
  const config = {
    proxyPort: 3000,
  }

  applyLoadedConfig(config, {
    proxyPort: 3100,
    gatewayRoutes: {
      codex: { provider: 'openai', credentialType: '', model: 'gpt-5.5' },
    },
  }, true)

  assert.deepEqual(config.gatewayRoutes, {
    codex: { provider: 'openai', credentialType: '', model: 'gpt-5.5' },
  })
})
