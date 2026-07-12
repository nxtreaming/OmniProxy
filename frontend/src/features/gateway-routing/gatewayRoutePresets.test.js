import assert from 'node:assert/strict'
import test from 'node:test'

import { inferGatewayProviderForModel, routeDefinitions, routeStrategyChain } from './gatewayRoutePresets.js'

test('routeDefinitions build stable local gateway endpoints', () => {
  const endpoints = Object.fromEntries(routeDefinitions.map((route) => [route.key, route.endpoint(3899)]))

  assert.equal(endpoints.codex, 'http://127.0.0.1:3899/codex/v1')
  assert.equal(endpoints.claude, 'http://127.0.0.1:3899/anthropic-router')
  assert.equal(endpoints.openai, 'http://127.0.0.1:3899/opencode-router/v1')
  assert.equal(endpoints.gemini, 'http://127.0.0.1:3899/gemini')
})

test('routeDefinitions expose GPT-5.6 role-aware defaults and family presets', () => {
  const codex = routeDefinitions.find((route) => route.key === 'codex')
  const openai = routeDefinitions.find((route) => route.key === 'openai')

  assert.equal(codex.fallback.model, 'gpt-5.6-sol')
  assert.equal(openai.fallback.model, 'gpt-5.6-terra')
  for (const model of ['gpt-5.6-sol', 'gpt-5.6-terra', 'gpt-5.6-luna']) {
    assert.ok(codex.modelPresets.includes(model))
    assert.ok(openai.modelPresets.includes(model))
  }
})

test('routeDefinitions expose Forge only for documented Chat and Messages protocols', () => {
  const codex = routeDefinitions.find((route) => route.key === 'codex')
  const claude = routeDefinitions.find((route) => route.key === 'claude')
  const openai = routeDefinitions.find((route) => route.key === 'openai')

  assert.equal(codex.providers.includes('forge'), false)
  assert.equal(claude.providers.includes('forge'), true)
  assert.equal(openai.providers.includes('forge'), true)
})

test('inferGatewayProviderForModel keeps provider inference stable', () => {
  const cases = [
    ['claude-sonnet-4-6', 'zo'],
    ['claude-opus-4-7', 'zo'],
    ['claude-sonnet-4-5', 'anthropic'],
    ['deepseek-v4-pro', 'deepseek'],
    ['kimi-for-coding', 'kimi'],
    ['mimo-v2.5-pro', 'xiaomi'],
    ['glm-5.1', 'zhipu'],
    ['MiniMax-M2.7', 'minimax'],
    ['gemini-3-pro-preview', 'gemini'],
    ['auto:balance', 'tokenrouter'],
    ['openai/gpt-5.4', 'openrouter'],
    ['custom-model', 'custom'],
    ['gpt-5.6-sol', 'openai'],
    ['gpt-5.4', 'openai'],
  ]

  for (const [model, provider] of cases) {
    assert.equal(inferGatewayProviderForModel(model), provider)
  }
})

test('routeStrategyChain orders known providers and preserves unknown providers', () => {
  assert.deepEqual(
    routeStrategyChain(['openai', 'deepseek', 'prem', 'local-gateway'], 'cost'),
    ['deepseek', 'prem', 'openai', 'local-gateway'],
  )
  assert.deepEqual(routeStrategyChain(['prem', 'openai', 'zo'], 'speed'), ['prem', 'zo', 'openai'])
})
