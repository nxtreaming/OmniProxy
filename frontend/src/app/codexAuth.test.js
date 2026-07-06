import { strict as assert } from 'node:assert'
import test from 'node:test'

import { codexIdentityFromAuthJSON } from './codexAuth.js'

function jwtForTest(payload) {
  return `header.${Buffer.from(JSON.stringify(payload)).toString('base64url')}.signature`
}

test('codexIdentityFromAuthJSON reads direct email and account_id', () => {
  const identity = codexIdentityFromAuthJSON(JSON.stringify({
    type: 'codex',
    email: 'coder@example.com',
    access_token: 'codex-access-token',
    account_id: 'account-direct',
  }))

  assert.deepEqual(identity, {
    email: 'coder@example.com',
    accountId: 'account-direct',
  })
})

test('codexIdentityFromAuthJSON falls back to id_token claims', () => {
  const identity = codexIdentityFromAuthJSON(JSON.stringify({
    tokens: {
      access_token: 'codex-access-token',
      id_token: jwtForTest({
        email: 'claims@example.com',
        'https://api.openai.com/auth': {
          chatgpt_account_id: 'account-claims',
        },
      }),
    },
  }))

  assert.deepEqual(identity, {
    email: 'claims@example.com',
    accountId: 'account-claims',
  })
})
