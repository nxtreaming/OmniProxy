import test from 'node:test'
import assert from 'node:assert/strict'

import {
  displayStatusClass,
  normalizeBillingDailyRows,
  openRouterQuotaRemaining,
  quotaPrimaryLabel,
  showPrimaryQuotaWindow,
  showSecondaryQuotaWindow,
  validationSuccessMessage,
  weeklyLimitReached,
} from './tokenDisplay.js'

test('normalizeBillingDailyRows sorts and normalizes usage rows', () => {
  assert.deepEqual(
    normalizeBillingDailyRows([
      { date: '2026-05-26', requestCount: '2', totalTokens: '30' },
      { date: '', requestCount: 10 },
      { date: '2026-05-27', inputTokens: '4', outputTokens: '6' },
    ]),
    [
      { date: '2026-05-27', requestCount: 0, inputTokens: 4, outputTokens: 6, totalTokens: 0 },
      { date: '2026-05-26', requestCount: 2, inputTokens: 0, outputTokens: 0, totalTokens: 30 },
    ],
  )
})

test('OpenRouter quota display uses refreshed balance data', () => {
  const token = {
    provider: 'openrouter',
    usage: {
      balanceRemaining: 12.5,
      balanceTotal: 20,
      balanceUnit: 'USD',
    },
  }

  assert.equal(openRouterQuotaRemaining(token), '12.50 USD')
  assert.equal(
    validationSuccessMessage({ provider: 'openrouter' }, { usage: { balanceRemaining: 1, balanceUsed: 2, balanceUnit: 'USD' } }),
    'OpenRouter 额度已刷新：剩余 1.00 USD，已用 2.00 USD',
  )
})

test('subscription quota helpers keep Codex free plan window rules', () => {
  const primaryToken = {
    provider: 'openai',
    credentialType: 'codex_auth_json',
    usage: {
      planType: 'free',
      subscriptionQuotaAvailable: true,
      primaryRemainingPercent: 80,
    },
  }
  const weeklyToken = {
    provider: 'openai',
    credentialType: 'codex_auth_json',
    usage: {
      planType: 'free',
      subscriptionQuotaAvailable: true,
      secondaryRemainingPercent: 0,
      secondaryUsedPercent: 100,
    },
  }

  assert.equal(showPrimaryQuotaWindow(primaryToken), true)
  assert.equal(showSecondaryQuotaWindow(primaryToken), false)
  assert.equal(showPrimaryQuotaWindow(weeklyToken), false)
  assert.equal(showSecondaryQuotaWindow(weeklyToken), true)
  assert.equal(weeklyLimitReached(weeklyToken), true)
})

test('Codex team plan shows the monthly quota label', () => {
  assert.equal(
    quotaPrimaryLabel({
      provider: 'openai',
      credentialType: 'codex_auth_json',
      usage: { planType: 'team' },
    }),
    '本月额度',
  )
  assert.equal(
    quotaPrimaryLabel({
      provider: 'openai',
      credentialType: 'codex_auth_json',
      usage: { planType: 'plus' },
    }),
    '5h额度',
  )
})

test('cooling tokens report warning status class', () => {
  assert.equal(displayStatusClass({ cooldownUntil: new Date(Date.now() + 60000).toISOString() }), 'warning')
})
