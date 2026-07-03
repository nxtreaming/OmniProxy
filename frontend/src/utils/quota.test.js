import { strict as assert } from 'node:assert'
import test from 'node:test'

import { aggregateAPIBalanceSummaries } from './quota.js'

test('aggregateAPIBalanceSummaries groups API key balances by currency', () => {
  const summaries = aggregateAPIBalanceSummaries([
    {
      credentialType: 'api_key',
      usage: { balanceUnit: 'CNY', balanceRemaining: 10.14, balanceTotal: 20, balanceUsed: 9.86 },
    },
    {
      credentialType: 'api_key',
      usage: { balanceUnit: 'USD', balanceRemaining: 0.24, balanceTotal: 1, balanceUsed: 0.76 },
    },
    {
      credentialType: 'api_key',
      usage: { balanceUnit: 'usd', balanceRemaining: 7.18, balanceTotal: 10, balanceUsed: 2.82 },
    },
    {
      id: 'multi-currency-key',
      credentialType: 'api_key',
      usage: {
        balanceUnit: 'CNY',
        balanceRemaining: 999,
        balancePackages: [
          { unit: 'CNY', balanceRemaining: 3 },
          { unit: 'USD', balanceRemaining: 2 },
        ],
      },
    },
    {
      credentialType: 'codex_auth_json',
      usage: { balanceUnit: 'USD', balanceRemaining: 100 },
    },
    {
      credentialType: 'api_key',
      usage: { balanceRemaining: 5 },
    },
  ])

  assert.deepEqual(summaries, [
    { unit: 'CNY', count: 2, remaining: 13.14, total: 20, used: 9.86 },
    { unit: 'USD', count: 3, remaining: 9.42, total: 11, used: 3.58 },
  ])
})
