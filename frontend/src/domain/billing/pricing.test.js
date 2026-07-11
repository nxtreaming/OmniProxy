import assert from 'node:assert/strict'
import test from 'node:test'

import { resolvePrice } from './pricing.js'

test('resolvePrice supports all GPT-5.6 tiers', () => {
  assert.deepEqual(
    ['gpt-5.6-sol', 'gpt-5.6-terra', 'gpt-5.6-luna'].map((model) => {
      const price = resolvePrice(model)
      return [price.label, price.input, price.output, price.cacheCreation, price.cacheRead]
    }),
    [
      ['OpenAI GPT-5.6 Sol', 5, 30, 6.25, 0.5],
      ['OpenAI GPT-5.6 Terra', 2.5, 15, 3.125, 0.25],
      ['OpenAI GPT-5.6 Luna', 1, 6, 1.25, 0.1],
    ],
  )
})
