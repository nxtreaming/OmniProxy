import test from 'node:test'
import assert from 'node:assert/strict'

import {
  codexWeeklyQuotaEstimateMeta,
  codexWeeklyQuotaEstimateText,
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
      { date: '2026-05-27', requestCount: 0, inputTokens: 4, outputTokens: 6, totalTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0 },
      { date: '2026-05-26', requestCount: 2, inputTokens: 0, outputTokens: 0, totalTokens: 30, cacheCreationTokens: 0, cacheReadTokens: 0 },
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

test('Codex team plan shows the 5h quota label', () => {
  assert.equal(
    quotaPrimaryLabel({
      provider: 'openai',
      credentialType: 'codex_auth_json',
      usage: { planType: 'team' },
    }),
    '5h额度',
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

test('Codex weekly quota estimate uses current weekly tokens and remaining percent', () => {
  const resetAt = Math.floor(Date.parse('2026-06-18T00:00:00+08:00') / 1000)
  const token = {
    provider: 'openai',
    credentialType: 'codex_auth_json',
    usage: {
      subscriptionQuotaAvailable: true,
      secondaryRemainingPercent: 80,
      secondaryResetAt: resetAt,
    },
    stats: {
      inputTokens: 900000,
      outputTokens: 900000,
      totalTokens: 1800000,
      daily: [
        { date: '2026-06-01', inputTokens: 900000, outputTokens: 900000, totalTokens: 1800000 },
        { date: '2026-06-12', inputTokens: 100000, outputTokens: 10000, totalTokens: 110000 },
      ],
    },
  }

  assert.equal(codexWeeklyQuotaEstimateText(token), '$2.00 / 周')
  assert.equal(codexWeeklyQuotaEstimateMeta(token), '按 110,000 Token、已用成本 $0.4000 和已用 20% 估算 · OpenAI GPT-5.5')
})

test('Codex weekly quota estimate prices cache tokens like sub2api', () => {
  const resetAt = Math.floor(Date.parse('2026-06-18T00:00:00+08:00') / 1000)
  const token = {
    provider: 'openai',
    credentialType: 'codex_auth_json',
    usage: {
      subscriptionQuotaAvailable: true,
      secondaryRemainingPercent: 80,
      secondaryResetAt: resetAt,
    },
    stats: {
      daily: [
        {
          date: '2026-06-12',
          inputTokens: 120000,
          outputTokens: 10000,
          totalTokens: 130000,
          cacheCreationTokens: 5000,
          cacheReadTokens: 100000,
        },
      ],
    },
  }

  assert.equal(codexWeeklyQuotaEstimateText(token), '$1.19 / 周')
  assert.equal(codexWeeklyQuotaEstimateMeta(token), '按 130,000 Token、已用成本 $0.2375 和已用 20% 估算 · OpenAI GPT-5.5')
})

test('Codex weekly quota estimate stays hidden without consumed weekly quota', () => {
  assert.equal(
    codexWeeklyQuotaEstimateText({
      provider: 'openai',
      credentialType: 'codex_auth_json',
      usage: {
        subscriptionQuotaAvailable: true,
        secondaryRemainingPercent: 100,
      },
      stats: {
        totalTokens: 1000,
      },
    }),
    '',
  )
})

test('cooling tokens report warning status class', () => {
  assert.equal(displayStatusClass({ cooldownUntil: new Date(Date.now() + 60000).toISOString() }), 'warning')
})
