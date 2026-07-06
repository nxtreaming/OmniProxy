import assert from 'node:assert/strict'
import { test } from 'node:test'

import {
  buildQuotaWorkspaceGroups,
  quotaWorkspaceLabel,
  quotaWorkspaceTitle,
} from './quotaWorkspaceGroups.js'

function codexToken(id, name, accountId, selected = false) {
  return {
    id,
    name,
    accountId,
    selected,
    provider: 'openai',
    credentialType: 'codex_auth_json',
  }
}

test('buildQuotaWorkspaceGroups groups same-email Codex workspaces', () => {
  const tokens = [
    codexToken('a', 'coder@example.com', 'workspace-a'),
    codexToken('b', 'coder@example.com', 'workspace-b'),
    codexToken('c', 'other@example.com', 'workspace-c'),
  ]

  const groups = buildQuotaWorkspaceGroups(tokens)

  assert.equal(groups.length, 2)
  assert.equal(groups[0].isWorkspaceGroup, true)
  assert.equal(groups[0].tokens.length, 2)
  assert.equal(groups[0].current.id, 'a')
  assert.equal(groups[1].isWorkspaceGroup, false)
  assert.equal(groups[1].current.id, 'c')
})

test('buildQuotaWorkspaceGroups keeps API keys and different emails separate', () => {
  const tokens = [
    { id: 'api-1', name: 'shared', provider: 'openai', credentialType: 'api_key' },
    { id: 'api-2', name: 'shared', provider: 'openai', credentialType: 'api_key' },
    codexToken('codex-1', 'coder+one@example.com', 'workspace-one'),
    codexToken('codex-2', 'coder+two@example.com', 'workspace-two'),
  ]

  const groups = buildQuotaWorkspaceGroups(tokens)

  assert.equal(groups.length, 4)
  assert.deepEqual(groups.map((group) => group.current.id), ['api-1', 'api-2', 'codex-1', 'codex-2'])
})

test('buildQuotaWorkspaceGroups uses selected workspace unless page state overrides it', () => {
  const tokens = [
    codexToken('a', 'coder@example.com', 'workspace-a'),
    codexToken('b', 'coder@example.com', 'workspace-b', true),
  ]

  assert.equal(buildQuotaWorkspaceGroups(tokens)[0].current.id, 'b')
  assert.equal(buildQuotaWorkspaceGroups(tokens, { 'codex:coder@example.com': 0 })[0].current.id, 'a')
})

test('quota workspace labels surface account_id safely', () => {
  const token = codexToken('a', 'coder@example.com', 'acct-1234567890abcdef')

  assert.equal(quotaWorkspaceLabel(token), 'acct-123...abcdef')
  assert.equal(quotaWorkspaceTitle(token), 'account_id: acct-1234567890abcdef')
  assert.equal(quotaWorkspaceLabel(codexToken('b', 'coder@example.com', '')), '工作区 1')
})
