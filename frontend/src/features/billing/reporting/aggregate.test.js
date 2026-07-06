import assert from 'node:assert/strict'
import { test } from 'node:test'

import { buildBillingRows } from './aggregate.js'

test('buildBillingRows groups daily usage by workspace without losing priced cost', () => {
  const rows = buildBillingRows([], '2026-05-01', [
    {
      date: '2026-05-01',
      model: 'gpt-5.5',
      tokenId: 'workspace-a',
      tokenName: 'coder@example.com (a)',
      requestCount: 1,
      inputTokens: 1_000_000,
      outputTokens: 0,
      totalTokens: 1_000_000,
    },
    {
      date: '2026-05-01',
      model: 'gpt-5.5',
      tokenId: 'workspace-b',
      tokenName: 'coder@example.com (b)',
      requestCount: 1,
      inputTokens: 2_000_000,
      outputTokens: 0,
      totalTokens: 2_000_000,
    },
  ], 'workspace')

  assert.equal(rows.length, 2)
  assert.deepEqual(rows.map((row) => row.model), ['coder@example.com (b)', 'coder@example.com (a)'])
  assert.equal(rows[0].groupMode, 'workspace')
  assert.equal(rows[0].models[0], 'gpt-5.5')
  assert.equal(rows[0].billable, true)
  assert.ok(rows[0].cost > rows[1].cost)
})

test('buildBillingRows keeps model grouping as the default', () => {
  const rows = buildBillingRows([], '2026-05-01', [
    {
      date: '2026-05-01',
      model: 'gpt-5.5',
      tokenId: 'workspace-a',
      tokenName: 'coder@example.com (a)',
      requestCount: 1,
      inputTokens: 1_000,
      outputTokens: 2_000,
      totalTokens: 3_000,
    },
    {
      date: '2026-05-01',
      model: 'gpt-5.5',
      tokenId: 'workspace-b',
      tokenName: 'coder@example.com (b)',
      requestCount: 1,
      inputTokens: 3_000,
      outputTokens: 4_000,
      totalTokens: 7_000,
    },
  ])

  assert.equal(rows.length, 1)
  assert.equal(rows[0].model, 'gpt-5.5')
  assert.equal(rows[0].requestCount, 2)
  assert.equal(rows[0].totalTokens, 10_000)
})
