import { computed } from 'vue'
import { providers, tabs } from '../constants/app'
import { claudeModelOptions, claudeModelSelectionLimit, codexModelOptions, codexModelSelectionLimit } from '../constants/claudeModels'
import { formatNumber, localDateKey } from '../utils/format'
import { aggregateAPIBalanceSummaries } from '../utils/quota'
import { isCooling, normalizeBillingDailyRows, showQuotaWindows } from '../utils/tokenDisplay'

const quotaOverviewPageSize = 4

export function claudeModelLabel(modelId) {
  return claudeModelOptions.find((option) => option.id === modelId)?.label || modelId
}

export function codexModelLabel(modelId) {
  return codexModelOptions.find((option) => option.id === modelId)?.label || modelId
}

export function createAppDerivedState(state, tokenHelpers) {
  const enabledTokens = computed(() => state.tokens.value.filter((item) => !item.disabled))
  const disabledTokens = computed(() => state.tokens.value.filter((item) => item.disabled))
  const activeTokens = computed(() => enabledTokens.value.filter((item) => item.status === 'active'))
  const lowTokens = computed(() => enabledTokens.value.filter((item) => item.status === 'low'))
  const exhaustedTokens = computed(() =>
    enabledTokens.value.filter((item) => item.status === 'exhausted' && !isCooling(item)),
  )
  const invalidTokens = computed(() => enabledTokens.value.filter((item) => item.status === 'invalid'))
  const coolingTokens = computed(() => enabledTokens.value.filter((item) => isCooling(item)))
  const healthWatchThreshold = computed(() => clampHealthThreshold(state.config.healthWatchThreshold, 80))
  const healthRiskThreshold = computed(() =>
    Math.min(healthWatchThreshold.value - 1, clampHealthThreshold(state.config.healthRiskThreshold, 50)),
  )
  const watchTokens = computed(() =>
    enabledTokens.value.filter((item) => {
      const score = Number(item.healthScore)
      return score >= healthRiskThreshold.value && score < healthWatchThreshold.value
    }),
  )
  const riskTokens = computed(() =>
    enabledTokens.value.filter((item) => Number(item.healthScore) < healthRiskThreshold.value),
  )
  const activeTokenIds = computed(() => new Set(state.activeRequests.value.map((item) => item.tokenId).filter(Boolean)))
  const activeProviderInfo = computed(
    () => providers.find((item) => item.key === state.activeProvider.value) || providers[0],
  )
  const activeProviderTokens = computed(() => tokenHelpers.providerTokens(state.activeProvider.value))
  const activeProviderEnabledCount = computed(
    () => activeProviderTokens.value.filter((item) => !item.disabled).length,
  )
  const activeProviderAPIBalanceSummaries = computed(() =>
    aggregateAPIBalanceSummaries(activeProviderTokens.value),
  )
  const openRouterTokens = computed(() => tokenHelpers.providerTokens('openrouter'))
  const currentTabLabel = computed(() => tabs.find((tab) => tab.key === state.activeTab.value)?.label || '控制台')
  const proxyEndpoint = computed(() => `127.0.0.1:${state.proxyStatus.port || state.config.proxyPort}`)
  const appThemeLabel = computed(() => (state.isDark.value ? '浅色模式' : '深色模式'))
  const selectedClaudeModelLabels = computed(() =>
    state.selectedClaudeModels.value.map((model) => claudeModelLabel(model)).filter(Boolean),
  )
  const selectedCodexModelLabels = computed(() =>
    state.selectedCodexModels.value.map((model) => codexModelLabel(model)).filter(Boolean),
  )
  const canConfigureCodexModels = computed(
    () => state.selectedCodexModels.value.length > 0 && state.selectedCodexModels.value.length <= codexModelSelectionLimit,
  )
  const canConfigureClaudeModels = computed(
    () => state.selectedClaudeModels.value.length > 0 && state.selectedClaudeModels.value.length <= claudeModelSelectionLimit,
  )
  const currentFirstUseGuideStep = computed(
    () => state.firstUseGuideSteps[state.firstUseGuideStepIndex.value] || state.firstUseGuideSteps[0],
  )
  const isMacOSPlatform = computed(() => String(state.appInfo.platform || '').toLowerCase().startsWith('darwin/'))
  const subscriptionOverviewTokens = computed(() => state.tokens.value.filter((item) => showQuotaWindows(item)))
  const apiOverviewTokens = computed(() => state.tokens.value.filter((item) => !showQuotaWindows(item)))
  const subscriptionOverviewPageCount = computed(() =>
    quotaOverviewPageCount(subscriptionOverviewTokens.value.length),
  )
  const apiOverviewPageCount = computed(() => quotaOverviewPageCount(apiOverviewTokens.value.length))
  const pagedSubscriptionOverviewTokens = computed(() =>
    quotaOverviewPageItems(subscriptionOverviewTokens.value, state.subscriptionQuotaPage.value),
  )
  const pagedApiOverviewTokens = computed(() =>
    quotaOverviewPageItems(apiOverviewTokens.value, state.apiQuotaPage.value),
  )
  const subscriptionQuotaPageText = computed(() =>
    quotaOverviewPageText(
      state.subscriptionQuotaPage.value,
      subscriptionOverviewPageCount.value,
      subscriptionOverviewTokens.value.length,
    ),
  )
  const apiQuotaPageText = computed(() =>
    quotaOverviewPageText(state.apiQuotaPage.value, apiOverviewPageCount.value, apiOverviewTokens.value.length),
  )
  const totalProxyRequests = computed(() => Number(state.billingSummary.value?.requestCount || 0))
  const totalProxyTokens = computed(() => Number(state.billingSummary.value?.totalTokens || 0))
  const totalProxyInputTokens = computed(() => Number(state.billingSummary.value?.inputTokens || 0))
  const totalProxyOutputTokens = computed(() => Number(state.billingSummary.value?.outputTokens || 0))
  const dailyUsageRows = computed(() => normalizeBillingDailyRows(state.billingSummary.value?.dailyRows || []))
  const todayProxyTokens = computed(
    () => dailyUsageRows.value.find((row) => row.date === localDateKey())?.totalTokens || 0,
  )
  const todayProxyRequests = computed(
    () => dailyUsageRows.value.find((row) => row.date === localDateKey())?.requestCount || 0,
  )
  const recentDailyUsageRows = computed(() => dailyUsageRows.value.slice(0, 14).reverse())
  const dashboardTrendRows = computed(() => recentDailyUsageRows.value.slice(-7))
  const dashboardDailyUsageRows = computed(() => dailyUsageRows.value.slice(0, 5))
  const usageTrendMax = computed(() =>
    Math.max(1, ...dashboardTrendRows.value.map((row) => Number(row.totalTokens || 0))),
  )
  const requestTrendMax = computed(() =>
    Math.max(1, ...dashboardTrendRows.value.map((row) => Number(row.requestCount || 0))),
  )
  const trendGridColumns = computed(
    () => `repeat(${Math.max(1, recentDailyUsageRows.value.length)}, minmax(0, 1fr))`,
  )
  const toolUsageRows = computed(() =>
    tokenHelpers.buildToolUsageRows(state.activeRequests.value, state.requestHistory.value),
  )
  const dashboardSignals = computed(() => [
    {
      label: '代理端口',
      value: state.proxyStatus.port || state.config.proxyPort,
      meta: state.proxyStatus.running ? '在线' : '待启动',
    },
    {
      label: '账号池',
      value: state.tokens.value.length,
      meta: `${activeTokens.value.length} 可用`,
    },
    {
      label: '实时连接',
      value: state.activeRequests.value.length,
      meta: `${activeTokenIds.value.size} 个账号占用`,
    },
    {
      label: '今日请求',
      value: formatNumber(todayProxyRequests.value),
      meta: `${formatNumber(todayProxyTokens.value)} Token`,
    },
  ])
  const isCodexForm = computed(() => state.form.provider === 'openai' && state.form.credentialType === 'codex_auth_json')
  const isAutoNameForm = computed(
    () =>
      isCodexForm.value ||
      (state.form.provider === 'anthropic' && state.form.credentialType === 'claude_oauth_json'),
  )

  function isTokenActiveNow(item) {
    return activeTokenIds.value.has(item.id)
  }

  function trendHeight(row) {
    const value = Number(row.totalTokens || 0)
    if (value <= 0) return '4%'
    return `${Math.max(8, Math.round((value / usageTrendMax.value) * 100))}%`
  }

  function requestTrendHeight(row) {
    const value = Number(row.requestCount || 0)
    if (value <= 0) return '4%'
    return `${Math.max(8, Math.round((value / requestTrendMax.value) * 100))}%`
  }

  function trendWidth(row) {
    const value = Number(row.totalTokens || 0)
    if (value <= 0) return '2%'
    return `${Math.max(6, Math.round((value / usageTrendMax.value) * 100))}%`
  }

  function requestTrendWidth(row) {
    const value = Number(row.requestCount || 0)
    if (value <= 0) return '2%'
    return `${Math.max(6, Math.round((value / requestTrendMax.value) * 100))}%`
  }

  function quotaOverviewPageCount(total) {
    return Math.max(1, Math.ceil(Number(total || 0) / quotaOverviewPageSize))
  }

  function clampQuotaOverviewPage(page, pageCount) {
    const maxPage = Math.max(0, Number(pageCount || 1) - 1)
    const nextPage = Number(page || 0)
    if (!Number.isFinite(nextPage) || nextPage < 0) return 0
    if (nextPage > maxPage) return maxPage
    return nextPage
  }

  function quotaOverviewPageItems(items, page) {
    const safeItems = Array.isArray(items) ? items : []
    const safePage = clampQuotaOverviewPage(page, quotaOverviewPageCount(safeItems.length))
    const start = safePage * quotaOverviewPageSize
    return safeItems.slice(start, start + quotaOverviewPageSize)
  }

  function quotaOverviewPageText(page, pageCount, total) {
    if (!total) return '0 / 0'
    return `${clampQuotaOverviewPage(page, pageCount) + 1} / ${pageCount}`
  }

  function quotaOverviewRangeText(page, total) {
    if (!total) return ''
    const start = clampQuotaOverviewPage(page, quotaOverviewPageCount(total)) * quotaOverviewPageSize + 1
    const end = Math.min(total, start + quotaOverviewPageSize - 1)
    return `${start}-${end} / ${total}`
  }

  function clampHealthThreshold(value, fallback) {
    const parsed = Number(value || fallback)
    if (!Number.isFinite(parsed)) return fallback
    return Math.min(100, Math.max(1, Math.round(parsed)))
  }

  function changeQuotaOverviewPage(type, direction) {
    const target = type === 'subscription' ? state.subscriptionQuotaPage : state.apiQuotaPage
    const count = type === 'subscription' ? subscriptionOverviewPageCount.value : apiOverviewPageCount.value
    target.value = clampQuotaOverviewPage(target.value + direction, count)
  }

  return {
    enabledTokens,
    disabledTokens,
    activeTokens,
    lowTokens,
    exhaustedTokens,
    invalidTokens,
    coolingTokens,
    watchTokens,
    riskTokens,
    activeTokenIds,
    activeProviderInfo,
    activeProviderTokens,
    activeProviderEnabledCount,
    activeProviderAPIBalanceSummaries,
    openRouterTokens,
    currentTabLabel,
    proxyEndpoint,
    appThemeLabel,
    selectedClaudeModelLabels,
    selectedCodexModelLabels,
    canConfigureCodexModels,
    canConfigureClaudeModels,
    currentFirstUseGuideStep,
    isMacOSPlatform,
    dashboardSignals,
    subscriptionOverviewTokens,
    apiOverviewTokens,
    subscriptionOverviewPageCount,
    apiOverviewPageCount,
    pagedSubscriptionOverviewTokens,
    pagedApiOverviewTokens,
    subscriptionQuotaPageText,
    apiQuotaPageText,
    totalProxyRequests,
    totalProxyTokens,
    totalProxyInputTokens,
    totalProxyOutputTokens,
    dailyUsageRows,
    todayProxyTokens,
    todayProxyRequests,
    recentDailyUsageRows,
    dashboardTrendRows,
    dashboardDailyUsageRows,
    usageTrendMax,
    requestTrendMax,
    trendGridColumns,
    toolUsageRows,
    isCodexForm,
    isAutoNameForm,
    isTokenActiveNow,
    trendHeight,
    requestTrendHeight,
    trendWidth,
    requestTrendWidth,
    quotaOverviewPageCount,
    clampQuotaOverviewPage,
    quotaOverviewPageItems,
    quotaOverviewPageText,
    quotaOverviewRangeText,
    changeQuotaOverviewPage,
  }
}
