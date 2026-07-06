import assert from 'node:assert/strict'
import { test } from 'node:test'

import {
  filterHistory,
  historyWorkspaceOptions,
  rankHistory,
  historyWorkspaceLabel,
} from './historyAnalytics.js'

test('filterHistory can isolate a workspace by token id', () => {
  const entries = [
    { tokenId: 'workspace-a', tokenName: 'coder@example.com (a)', provider: 'openai', clientKey: 'codex', level: 'info', status: 200 },
    { tokenId: 'workspace-b', tokenName: 'coder@example.com (b)', provider: 'openai', clientKey: 'codex', level: 'info', status: 200 },
  ]

  const result = filterHistory(entries, {
    provider: 'all',
    client: 'all',
    level: 'all',
    status: 'all',
    model: '',
    tokenId: 'workspace-b',
    token: '',
    search: '',
  })

  assert.deepEqual(result.map((entry) => entry.tokenId), ['workspace-b'])
})

test('historyWorkspaceOptions exposes token id choices with readable labels', () => {
  const entries = [
    { tokenId: 'workspace-b', tokenName: 'coder@example.com (b)' },
    { tokenId: 'workspace-a', tokenName: 'coder@example.com (a)' },
    { tokenId: 'workspace-a', tokenName: 'duplicate' },
  ]

  assert.deepEqual(historyWorkspaceOptions(entries), [
    { key: 'workspace-a', label: 'coder@example.com (a)' },
    { key: 'workspace-b', label: 'coder@example.com (b)' },
  ])
})

test('rankHistory can group by workspace label', () => {
  const ranks = rankHistory([
    { tokenId: 'workspace-a', tokenName: 'coder@example.com (a)', totalTokens: 100 },
    { tokenId: 'workspace-b', tokenName: 'coder@example.com (b)', totalTokens: 300 },
  ], historyWorkspaceLabel, 'totalTokens')

  assert.equal(ranks[0].label, 'coder@example.com (b)')
  assert.equal(ranks[0].totalTokens, 300)
})
