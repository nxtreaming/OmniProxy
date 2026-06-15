<script setup>
import { computed, defineAsyncComponent, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { ElMessageBox } from 'element-plus'
import DiagnosticDrawer from './components/DiagnosticDrawer.vue'
import appIconUrl from './assets/appicon.png'
import { credentialTypes, providers, tabs } from './constants/app'
import { claudeModelOptions, claudeModelSelectionLimit } from './constants/claudeModels'
import { knownClientTools } from './constants/clientTools'
import { useAppUpdate } from './composables/useAppUpdate'
import { useWorkspaceScroll } from './composables/useWorkspaceScroll'
import { formatDuration, formatNumber, formatTime, localDateKey } from './utils/format'
import { aggregateAPIBalanceSummaries } from './utils/quota'
import {
  apiQuotaDisplay,
  apiQuotaMeta,
  displayStatusClass,
  displayStatusLabel,
  displayStatusType,
  formatBalance,
  healthSummary,
  isCodexToken,
  isCooling,
  normalizeBillingDailyRows,
  quotaDisplay,
  quotaPercentText,
  quotaPercentValue,
  quotaPrimaryLabel,
  showQuotaWindows,
  validationSuccessMessage,
  weeklyLimitReached,
} from './utils/tokenDisplay'
import {
  WindowHide,
  WindowIsMaximised,
  WindowMinimise,
  WindowToggleMaximise,
} from '../wailsjs/runtime/runtime'
import {
  configureAnyRouterClaude,
  configureCodex,
  configureCodexAnyRouter,
  configureCodexNewAPI,
  configureCodexPrem,
  configureCodexSub2API,
  configureCodexZo,
  configureClaudeDesktopModels,
  configureClaudeModels,
  configureDeepSeekClaude,
  configureDeepSeekTUI,
  configureGemini,
  configureKimiClaude,
  configureMimoClaude,
  configureOpenCode,
  configurePi,
  configurePremClaude,
  configureZhipuClaude,
  configureZoClaude,
  createToken,
  chooseDataDirectory as chooseDataDirectoryWithDialog,
  clearBillingUsage,
  clearRequestHistory,
  deleteToken,
  exportCodexAuthFiles,
  exportHistory,
  exportTokens,
  getAutoStartStatus,
  getAppInfo,
  getActiveRequests,
  getBillingDates,
  getBillingSummary,
  getBillingUsage,
  getConfig,
  getDataDirectory,
  getHistory,
  getHistorySummary,
  getLogs,
  getOpenRouterModels,
  getProxyStatus,
  getTaskAutomationBrowserProfiles,
  getTokens,
  importAPIKeys,
  openExternalURL,
  refreshTokenAuth,
  saveConfig,
  setAutoStart,
  setTokenDisabled,
  setTokenSelected,
  startProxy,
  stopProxy,
  updateToken,
  validateToken,
  restoreCodex,
  restoreClaudeDesktop,
  restoreDeepSeekTUI,
  restoreGemini,
  restoreMimoClaude,
  restoreOpenCode,
  restorePi,
  restoreZhipuClaude,
} from './services/api'
import {
  Coin,
  Clock,
  DataBoard,
  HelpFilled,
  InfoFilled,
  Key,
  MagicStick,
  Memo,
  Monitor,
  Money,
  Moon,
  Setting,
  Sunny,
  SwitchButton,
  TrendCharts,
} from '@element-plus/icons-vue'

const DashboardView = defineAsyncComponent(() => import('./components/DashboardView.vue'))
const AboutView = defineAsyncComponent(() => import('./components/AboutView.vue'))
const BillingView = defineAsyncComponent(() => import('./components/BillingView.vue'))
const FirstUseGuideModal = defineAsyncComponent(() => import('./components/FirstUseGuideModal.vue'))
const HistoryView = defineAsyncComponent(() => import('./components/HistoryView.vue'))
const HelpView = defineAsyncComponent(() => import('./components/HelpView.vue'))
const LogsView = defineAsyncComponent(() => import('./components/LogsView.vue'))
const OpenRouterChatView = defineAsyncComponent(() => import('./components/OpenRouterChatView.vue'))
const QuickstartView = defineAsyncComponent(() => import('./components/QuickstartView.vue'))
const QuotasView = defineAsyncComponent(() => import('./components/QuotasView.vue'))
const SettingsView = defineAsyncComponent(() => import('./components/SettingsView.vue'))
const TokenBatchImportModal = defineAsyncComponent(() => import('./components/TokenBatchImportModal.vue'))
const TokenEditorModal = defineAsyncComponent(() => import('./components/TokenEditorModal.vue'))
const TokenTrendView = defineAsyncComponent(() => import('./components/TokenTrendView.vue'))
const TokensView = defineAsyncComponent(() => import('./components/TokensView.vue'))

const activeTab = ref('dashboard')
const activeProvider = ref('openai')
const mobileSidebarOpen = ref(false)
const tabIcons = {
  dashboard: DataBoard,
  'usage-trends': TrendCharts,
  billing: Money,
  quotas: Coin,
  tokens: Key,
  'openrouter-chat': Monitor,
  history: Clock,
  logs: Memo,
  quickstart: MagicStick,
  settings: Setting,
  about: InfoFilled,
  help: HelpFilled,
}
const navSections = [
  { label: '总览', items: tabs.filter((tab) => ['dashboard', 'usage-trends', 'billing', 'quotas'].includes(tab.key)) },
  { label: '运行', items: tabs.filter((tab) => ['tokens', 'history', 'logs', 'quickstart'].includes(tab.key)) },
  { label: '体验', items: tabs.filter((tab) => ['openrouter-chat'].includes(tab.key)) },
  { label: '系统', items: tabs.filter((tab) => ['settings', 'about', 'help'].includes(tab.key)) },
]
const tabKeys = new Set(tabs.map((tab) => tab.key))
const isDark = ref(false)
const windowMaximised = ref(false)
const loading = ref(false)
const codexConfiguring = ref(false)
const codexSub2APIConfiguring = ref(false)
const codexNewAPIConfiguring = ref(false)
const codexAnyRouterConfiguring = ref(false)
const codexZoConfiguring = ref(false)
const codexPremConfiguring = ref(false)
const codexRestoring = ref(false)
const mimoClaudeConfiguring = ref(false)
const deepSeekClaudeConfiguring = ref(false)
const kimiClaudeConfiguring = ref(false)
const zhipuClaudeConfiguring = ref(false)
const anyRouterClaudeConfiguring = ref(false)
const zoClaudeConfiguring = ref(false)
const premClaudeConfiguring = ref(false)
const claudeModelsConfiguring = ref(false)
const claudeDesktopConfiguring = ref(false)
const claudeDesktopRestoring = ref(false)
const deepSeekTUIConfiguring = ref(false)
const geminiConfiguring = ref(false)
const opencodeConfiguring = ref(false)
const piConfiguring = ref(false)
const mimoClaudeRestoring = ref(false)
const deepSeekTUIRestoring = ref(false)
const geminiRestoring = ref(false)
const opencodeRestoring = ref(false)
const piRestoring = ref(false)
const refreshingProvider = ref(false)
const dataDirChanging = ref(false)
const autoStartChanging = ref(false)
const autoStartEnabled = ref(false)
const taskAutomationBrowserProfiles = ref([])
const taskAutomationBrowserProfilesLoading = ref(false)
const taskAutomationBrowserProfilesError = ref('')
const exportingHistory = ref('')
const exportingTokens = ref(false)
const exportingCodexAuth = ref(false)
const codexAuthImporting = ref(false)
const batchImporting = ref(false)
const clearingBillingUsage = ref(false)
const clearingRequestHistory = ref(false)
const errorMessage = ref('')
const successMessage = ref('')
const quotaRefreshProgress = reactive({
  visible: false,
  percent: 0,
  total: 0,
  completed: 0,
  failed: 0,
  providerLabel: '',
  currentName: '',
})
const deleteCandidate = ref(null)
const deleteBusy = ref(false)
const toastAutoCloseMs = 4000
const appThemeStorageKey = 'omniproxy.appTheme'
const firstUseGuideStorageKey = 'omniproxy.firstRunGuideModalDismissed'
let toastTimer = null
let realtimeTimer = null
let historyRefreshSeq = 0
let taskAutomationBrowserProfileSeq = 0
const validatingIds = reactive({})
const refreshingTokenIds = reactive({})
const togglingTokenIds = reactive({})
const switchingOnlyTokenIds = reactive({})
const tokens = ref([])
const logs = ref([])
const requestHistory = ref([])
const requestHistorySummary = ref(null)
const emptyBillingSummary = () => ({
  requestCount: 0,
  inputTokens: 0,
  outputTokens: 0,
  totalTokens: 0,
  dailyRows: [],
})
const billingSummary = ref(emptyBillingSummary())
const requestHistoryFilters = ref({})
const billingUsage = ref([])
const billingDates = ref([])
const selectedBillingDate = ref(localDateKey())
const activeRequests = ref([])
const openRouterModels = ref([])
const openRouterModelsFetchedAt = ref('')
const openRouterModelsCached = ref(false)
const openRouterModelsLoading = ref(false)
const openRouterModelsError = ref('')
const selectedOpenRouterChatModel = ref('')
const selectedHistoryEntry = ref(null)
const firstUseGuideVisible = ref(window.localStorage?.getItem(firstUseGuideStorageKey) !== '1')
const firstUseGuideStepIndex = ref(0)
const subscriptionQuotaPage = ref(0)
const apiQuotaPage = ref(0)
const proxyStatus = reactive({ running: false, port: 3000 })
const config = reactive({
  proxyPort: 3000,
  controlPort: 3890,
  schedulingMode: 'queue',
  websocketMode: 'enabled',
  checkBetaUpdates: false,
  taskAutomationEnabled: false,
  taskAutomationClients: ['codex', 'claude', 'claude-desktop'],
  taskAutomationLaunchMode: 'media',
  taskAutomationLaunchTarget: '',
  taskAutomationFallbackUrl: 'https://www.douyin.com',
  taskAutomationBrowser: 'default',
  taskAutomationBrowserUserDataDir: '',
  taskAutomationBrowserProfile: '',
  taskAutomationReturnToClient: true,
  taskAutomationIdleSeconds: 5,
  taskAutomationReturnDelaySeconds: 3,
  outboundProxyEnabled: false,
  outboundProxyUrl: 'http://127.0.0.1:10808',
  outboundProxyProviders: ['openai', 'anthropic', 'gemini', 'openrouter', 'zo', 'prem'],
  outboundProxyModels: ['gpt-*', 'claude-*', 'gemini-*', '*/*'],
  upstreamBaseUrl: 'https://api.openai.com',
  openaiBaseUrl: 'https://api.openai.com',
  anthropicBaseUrl: 'https://api.anthropic.com',
  deepseekBaseUrl: 'https://api.deepseek.com',
  deepseekAnthropicBaseUrl: 'https://api.deepseek.com/anthropic',
  kimiBaseUrl: 'https://api.kimi.com/coding',
  zhipuBaseUrl: 'https://open.bigmodel.cn/api/paas/v4',
  zhipuAnthropicBaseUrl: 'https://open.bigmodel.cn/api/anthropic',
  minimaxBaseUrl: 'https://api.minimaxi.com/v1',
  minimaxAnthropicBaseUrl: 'https://api.minimaxi.com/anthropic',
  geminiBaseUrl: 'https://generativelanguage.googleapis.com',
  openrouterBaseUrl: 'https://openrouter.ai/api/v1',
  tokenrouterBaseUrl: 'https://api.tokenrouter.io',
  sub2apiBaseUrl: 'https://aiapi.aicnio.com',
  newapiBaseUrl: 'http://127.0.0.1:3000',
  anyrouterBaseUrl: 'https://anyrouter.top',
  zoBaseUrl: 'https://api.zo.computer',
  premBaseUrl: 'http://127.0.0.1:3100',
  premAutoStartPcciProxy: true,
  customGatewayBaseUrl: '',
  customGatewayAnthropicBaseUrl: '',
  xiaomiBaseUrl: '',
  xiaomiApiBaseUrl: 'https://api.xiaomimimo.com/v1',
  xiaomiApiAnthropicBaseUrl: 'https://api.xiaomimimo.com/anthropic',
  xiaomiTokenPlanBaseUrl: 'https://token-plan-cn.xiaomimimo.com/v1',
  xiaomiTokenPlanAnthropicBaseUrl: 'https://token-plan-cn.xiaomimimo.com/anthropic',
  xiaomiTokenPlanSgpBaseUrl: 'https://token-plan-sgp.xiaomimimo.com/v1',
  xiaomiTokenPlanSgpAnthropicBaseUrl: 'https://token-plan-sgp.xiaomimimo.com/anthropic',
  xiaomiTokenPlanAmsBaseUrl: 'https://token-plan-ams.xiaomimimo.com/v1',
  xiaomiTokenPlanAmsAnthropicBaseUrl: 'https://token-plan-ams.xiaomimimo.com/anthropic',
  xiaomiCredentialPriority: 'mimo_token_plan',
  codexBaseUrl: 'https://chatgpt.com/backend-api/codex',
  codexUsageEndpoint: 'https://chatgpt.com/backend-api/wham/usage',
  switchThreshold: 15,
  maxRetries: 2,
  historyRetentionDays: 14,
})
const dataDirectory = reactive({
  dataDir: '',
  bootstrapPath: '',
  envOverride: false,
  source: '',
  pendingDataDir: '',
  restartRequired: false,
})
const selectedClaudeModels = ref([])
const appInfo = reactive({
  name: 'OmniProxy',
  version: 'dev',
  isDevelopment: true,
  updateEndpoint: '',
  platform: '',
  goVersion: '',
  executablePath: '',
  startedAt: '',
})
const isMacOSPlatform = computed(() => String(appInfo.platform || '').toLowerCase().startsWith('darwin/'))
const {
  updateChecking,
  lastUpdateInfo,
  lastUpdateCheckedAt,
  titlebarUpdatePopoverOpen,
  updateDownloadStatus,
  titlebarUpdateVisible,
  titlebarUpdatePrompt,
  closeTitlebarUpdatePopover,
  toggleTitlebarUpdatePopover,
  confirmTitlebarUpdatePopover,
  handleTitlebarUpdateOutsidePointer,
  handleTitlebarUpdateKeydown,
  notifyCompletedUpdateIfNeeded,
  manualCheckForUpdates,
  scheduleUpdateChecks,
  startUpdateDownload,
  refreshUpdateDownloadStatus,
  installReadyUpdateFromUpdateSurface,
  stopAppUpdateTimers,
} = useAppUpdate({
  appInfo,
  isMacOSPlatform,
  errorMessage,
  successMessage,
  showUpdateDetails: () => selectTab('about'),
})
const {
  workspaceRef,
  workspaceScrollbarVisible,
  saveWorkspaceScrollPosition,
  restoreActiveWorkspaceScroll,
  handleWorkspaceScroll,
  pauseWorkspaceScrollSaving,
  afterPageEnter,
  hideWorkspaceScrollbar,
  handleWorkspacePointerMove,
  disposeWorkspaceScroll,
} = useWorkspaceScroll(activeTab)
const form = reactive({
  visible: false,
  editingId: '',
  name: '',
  provider: 'openai',
  originalProvider: 'openai',
  credentialType: 'api_key',
  originalCredentialType: 'api_key',
  region: 'cn',
  baseUrl: '',
  tokenValue: '',
})
const batchImportForm = reactive({
  visible: false,
  provider: 'openai',
  baseUrl: '',
  tokenText: '',
})
const enabledTokens = computed(() => tokens.value.filter((item) => !item.disabled))
const disabledTokens = computed(() => tokens.value.filter((item) => item.disabled))
const activeTokens = computed(() => enabledTokens.value.filter((item) => item.status === 'active'))
const lowTokens = computed(() => enabledTokens.value.filter((item) => item.status === 'low'))
const exhaustedTokens = computed(() =>
  enabledTokens.value.filter((item) => item.status === 'exhausted' && !isCooling(item)),
)
const invalidTokens = computed(() => enabledTokens.value.filter((item) => item.status === 'invalid'))
const coolingTokens = computed(() => enabledTokens.value.filter((item) => isCooling(item)))
const activeTokenIds = computed(() => new Set(activeRequests.value.map((item) => item.tokenId).filter(Boolean)))
const activeProviderInfo = computed(
  () => providers.find((item) => item.key === activeProvider.value) || providers[0],
)
const activeProviderTokens = computed(() => providerTokens(activeProvider.value))
const activeProviderEnabledCount = computed(
  () => activeProviderTokens.value.filter((item) => !item.disabled).length,
)
const activeProviderAPIBalanceSummaries = computed(() =>
  aggregateAPIBalanceSummaries(activeProviderTokens.value),
)
const openRouterTokens = computed(() => providerTokens('openrouter'))
const currentTabLabel = computed(() => tabs.find((tab) => tab.key === activeTab.value)?.label || '控制台')
const proxyEndpoint = computed(() => `127.0.0.1:${proxyStatus.port || config.proxyPort}`)
const appThemeLabel = computed(() => (isDark.value ? '浅色模式' : '深色模式'))
const selectedClaudeModelLabels = computed(() =>
  selectedClaudeModels.value.map((model) => claudeModelLabel(model)).filter(Boolean),
)
const canConfigureClaudeModels = computed(
  () => selectedClaudeModels.value.length > 0 && selectedClaudeModels.value.length <= claudeModelSelectionLimit,
)
const firstUseGuideSteps = [
  {
    step: '01',
    title: '添加上游账号',
    description: '先在账号管理里添加至少一个可用凭据。Codex auth.json、Claude OAuth JSON、API Key、Zo Access Token 都从这里进入。',
    actionLabel: '打开账号管理',
    actionKey: 'tokens',
    icon: Key,
  },
  {
    step: '02',
    title: '启动本地代理',
    description: '代理启动后，客户端只需要连接本机 loopback 地址；真实凭据由 OmniProxy 在请求转发时注入。',
    actionLabel: '启动代理',
    actionKey: 'proxy',
    icon: SwitchButton,
  },
  {
    step: '03',
    title: '写入客户端配置',
    description: '用一键配置把 Codex、Claude Code、Claude Desktop、OpenCode、Pi、Gemini 等客户端指向 OmniProxy。',
    actionLabel: '打开一键配置',
    actionKey: 'quickstart',
    icon: MagicStick,
  },
]
const currentFirstUseGuideStep = computed(() => firstUseGuideSteps[firstUseGuideStepIndex.value] || firstUseGuideSteps[0])
const dashboardSignals = computed(() => [
  {
    label: '代理端口',
    value: proxyStatus.port || config.proxyPort,
    meta: proxyStatus.running ? '在线' : '待启动',
  },
  {
    label: '账号池',
    value: tokens.value.length,
    meta: `${activeTokens.value.length} 可用`,
  },
  {
    label: '实时连接',
    value: activeRequests.value.length,
    meta: `${activeTokenIds.value.size} 个账号占用`,
  },
  {
    label: '今日请求',
    value: formatNumber(todayProxyRequests.value),
    meta: `${formatNumber(todayProxyTokens.value)} Token`,
  },
])
const subscriptionOverviewTokens = computed(() => tokens.value.filter((item) => showQuotaWindows(item)))
const apiOverviewTokens = computed(() => tokens.value.filter((item) => !showQuotaWindows(item)))
const quotaOverviewPageSize = 4
const subscriptionOverviewPageCount = computed(() =>
  quotaOverviewPageCount(subscriptionOverviewTokens.value.length),
)
const apiOverviewPageCount = computed(() => quotaOverviewPageCount(apiOverviewTokens.value.length))
const pagedSubscriptionOverviewTokens = computed(() =>
  quotaOverviewPageItems(subscriptionOverviewTokens.value, subscriptionQuotaPage.value),
)
const pagedApiOverviewTokens = computed(() =>
  quotaOverviewPageItems(apiOverviewTokens.value, apiQuotaPage.value),
)
const subscriptionQuotaPageText = computed(() =>
  quotaOverviewPageText(subscriptionQuotaPage.value, subscriptionOverviewPageCount.value, subscriptionOverviewTokens.value.length),
)
const apiQuotaPageText = computed(() =>
  quotaOverviewPageText(apiQuotaPage.value, apiOverviewPageCount.value, apiOverviewTokens.value.length),
)
const totalProxyRequests = computed(() => Number(billingSummary.value?.requestCount || 0))
const totalProxyTokens = computed(() => Number(billingSummary.value?.totalTokens || 0))
const totalProxyInputTokens = computed(() => Number(billingSummary.value?.inputTokens || 0))
const totalProxyOutputTokens = computed(() => Number(billingSummary.value?.outputTokens || 0))
const dailyUsageRows = computed(() => normalizeBillingDailyRows(billingSummary.value?.dailyRows || []))
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
const toolUsageRows = computed(() => buildToolUsageRows(activeRequests.value, requestHistory.value))
const isCodexForm = computed(() => form.provider === 'openai' && form.credentialType === 'codex_auth_json')
const isAutoNameForm = computed(
  () =>
    isCodexForm.value ||
    (form.provider === 'anthropic' && form.credentialType === 'claude_oauth_json'),
)

function hasWailsRuntime() {
  return typeof window !== 'undefined' && Boolean(window.runtime)
}

async function refreshWindowState() {
  if (!hasWailsRuntime()) return
  try {
    windowMaximised.value = await WindowIsMaximised()
  } catch {
    windowMaximised.value = false
  }
}

function minimiseWindow() {
  if (hasWailsRuntime()) {
    WindowMinimise()
  }
}

function toggleWindowMaximise() {
  if (!hasWailsRuntime()) return
  WindowToggleMaximise()
  window.setTimeout(refreshWindowState, 120)
}

function startWindowResize(edge) {
  if (windowMaximised.value || typeof window === 'undefined' || !window.WailsInvoke) {
    return
  }
  window.WailsInvoke(`resize:${edge}`)
}

function closeWindow() {
  if (hasWailsRuntime()) {
    WindowHide()
  }
}

function toggleAppTheme() {
  isDark.value = !isDark.value
}

function selectTab(tabKey) {
  if (!tabKeys.has(tabKey)) return
  titlebarUpdatePopoverOpen.value = false
  if (activeTab.value !== tabKey) {
    saveWorkspaceScrollPosition(activeTab.value)
    activeTab.value = tabKey
  }
  mobileSidebarOpen.value = false
}

function syncDocumentTheme(value) {
  if (typeof document === 'undefined') {
    return
  }
  document.documentElement.classList.toggle('dark', value)
  document.body.classList.toggle('dark', value)
}

onMounted(async () => {
  const savedAppTheme = window.localStorage?.getItem(appThemeStorageKey)
  if (savedAppTheme === 'dark' || savedAppTheme === 'light') {
    isDark.value = savedAppTheme === 'dark'
  } else if (window.matchMedia?.('(prefers-color-scheme: dark)').matches) {
    isDark.value = true
  }
  syncDocumentTheme(isDark.value)
  await refreshWindowState()
  window.addEventListener('resize', refreshWindowState)
  window.addEventListener('keydown', handleTitlebarUpdateKeydown)
  document.addEventListener('pointerdown', handleTitlebarUpdateOutsidePointer)
  await refreshAll()
  notifyCompletedUpdateIfNeeded()
  await refreshUpdateDownloadStatus()
  scheduleUpdateChecks()
  realtimeTimer = window.setInterval(refreshRealtime, 3000)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', refreshWindowState)
  window.removeEventListener('keydown', handleTitlebarUpdateKeydown)
  document.removeEventListener('pointerdown', handleTitlebarUpdateOutsidePointer)
  if (realtimeTimer) {
    window.clearInterval(realtimeTimer)
    realtimeTimer = null
  }
  stopAppUpdateTimers()
  if (toastTimer) {
    window.clearTimeout(toastTimer)
    toastTimer = null
  }
  disposeWorkspaceScroll()
})

watch([errorMessage, successMessage], ([error, success]) => {
  if (toastTimer) {
    window.clearTimeout(toastTimer)
    toastTimer = null
  }
  if (!error && !success) {
    return
  }
  toastTimer = window.setTimeout(() => {
    errorMessage.value = ''
    successMessage.value = ''
    toastTimer = null
  }, toastAutoCloseMs)
})

watch(isDark, (value) => {
  window.localStorage?.setItem(appThemeStorageKey, value ? 'dark' : 'light')
  syncDocumentTheme(value)
})

watch(activeTab, (tab, previousTab) => {
  saveWorkspaceScrollPosition(previousTab)
  pauseWorkspaceScrollSaving()
  hideWorkspaceScrollbar()
  if (tab === 'history') {
    refreshHistory()
  } else if (tab === 'billing') {
    refreshBilling()
  } else if (tab === 'tokens' && activeProvider.value === 'openrouter') {
    refreshOpenRouterModels()
  } else if (tab === 'openrouter-chat') {
    refreshOpenRouterModels()
  } else if (tab === 'settings') {
    refreshTaskAutomationBrowserProfiles()
  }
})

watch(
  () => [config.taskAutomationLaunchMode, config.taskAutomationBrowser],
  () => {
    if (activeTab.value === 'settings') {
      refreshTaskAutomationBrowserProfiles()
    }
  },
)

watch(activeProvider, (provider) => {
  if (activeTab.value === 'tokens' && provider === 'openrouter') {
    refreshOpenRouterModels()
  }
})

watch(openRouterModels, (models) => {
  if (!selectedOpenRouterChatModel.value && models.length) {
    selectedOpenRouterChatModel.value = models[0].id
  }
})

watch(selectedClaudeModels, (models) => {
  const normalized = []
  for (const model of models) {
    if (!model || normalized.includes(model)) continue
    normalized.push(model)
    if (normalized.length >= claudeModelSelectionLimit) break
  }
  if (normalized.length !== models.length || normalized.some((model, index) => model !== models[index])) {
    selectedClaudeModels.value = normalized
  }
})

watch(subscriptionOverviewPageCount, (count) => {
  subscriptionQuotaPage.value = clampQuotaOverviewPage(subscriptionQuotaPage.value, count)
})

watch(apiOverviewPageCount, (count) => {
  apiQuotaPage.value = clampQuotaOverviewPage(apiQuotaPage.value, count)
})

async function refreshAll() {
  loading.value = true
  errorMessage.value = ''
  try {
    const [
      loadedTokens,
      loadedConfig,
      loadedLogs,
      loadedStatus,
      loadedActiveRequests,
      loadedHistory,
      loadedBillingSummary,
      loadedDataDirectory,
      loadedAutoStart,
      loadedAppInfo,
    ] = await Promise.all([
      getTokens(),
      getConfig(),
      getLogs(),
      getProxyStatus(),
      getActiveRequests(),
      getHistory({ limit: 200 }),
      getBillingSummary(30),
      getDataDirectory(),
      getAutoStartStatus(),
      getAppInfo(),
    ])
    tokens.value = loadedTokens
    logs.value = loadedLogs
    activeRequests.value = loadedActiveRequests
    requestHistory.value = loadedHistory
    billingSummary.value = loadedBillingSummary || emptyBillingSummary()
    Object.assign(config, loadedConfig)
    Object.assign(proxyStatus, loadedStatus)
    Object.assign(dataDirectory, loadedDataDirectory, {
      pendingDataDir: '',
      restartRequired: false,
    })
    autoStartEnabled.value = Boolean(loadedAutoStart?.enabled)
    Object.assign(appInfo, loadedAppInfo)
    if (activeTab.value === 'history') {
      await refreshHistory()
    } else if (activeTab.value === 'billing') {
      await refreshBilling()
    }
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    loading.value = false
  }
}

async function refreshRealtime() {
  try {
    const requests = [
      getLogs(),
      getProxyStatus(),
      getTokens(),
      getActiveRequests(),
      getBillingSummary(30),
    ]
    if (activeTab.value !== 'history') {
      requests.push(getHistory({ limit: 200 }))
    }
    const [loadedLogs, loadedStatus, loadedTokens, loadedActiveRequests, loadedBillingSummary, loadedHistory] = await Promise.all(requests)
    logs.value = loadedLogs
    tokens.value = loadedTokens
    activeRequests.value = loadedActiveRequests
    billingSummary.value = loadedBillingSummary || emptyBillingSummary()
    if (loadedHistory) {
      requestHistory.value = loadedHistory
    }
    Object.assign(proxyStatus, loadedStatus)
    if (activeTab.value === 'history') {
      await refreshHistory()
    } else if (activeTab.value === 'billing') {
      await refreshBilling()
    }
  } catch (error) {
    errorMessage.value = error.message
  }
}

async function refreshHistory(filters = requestHistoryFilters.value) {
  try {
    const seq = ++historyRefreshSeq
    const normalizedFilters = { ...(filters || {}) }
    requestHistoryFilters.value = normalizedFilters
    const [entries, summary] = await Promise.all([
      getHistory(normalizedFilters),
      getHistorySummary(normalizedFilters, 14),
    ])
    if (seq !== historyRefreshSeq) return
    requestHistory.value = entries
    requestHistorySummary.value = summary
  } catch (error) {
    errorMessage.value = error.message
  }
}

async function refreshBilling(date = selectedBillingDate.value) {
  try {
    const normalizedDate = String(date || localDateKey()).trim() || localDateKey()
    selectedBillingDate.value = normalizedDate
    const [usage, dates, summary] = await Promise.all([
      getBillingUsage(normalizedDate),
      getBillingDates(30),
      getBillingSummary(30),
    ])
    billingUsage.value = usage || []
    billingDates.value = dates || []
    billingSummary.value = summary || emptyBillingSummary()
  } catch (error) {
    errorMessage.value = error.message
  }
}

async function refreshOpenRouterModels({ force = false } = {}) {
  if (openRouterModelsLoading.value) {
    return
  }
  openRouterModelsLoading.value = true
  openRouterModelsError.value = ''
  try {
    const result = await getOpenRouterModels(force)
    openRouterModels.value = result?.models || []
    openRouterModelsFetchedAt.value = result?.fetchedAt || ''
    openRouterModelsCached.value = Boolean(result?.cached)
  } catch (error) {
    openRouterModelsError.value = error.message
  } finally {
    openRouterModelsLoading.value = false
  }
}

function openOpenRouterChat(model) {
  const modelId = typeof model === 'string' ? model : model?.id
  if (modelId) {
    selectedOpenRouterChatModel.value = modelId
  }
  selectTab('openrouter-chat')
  if (!openRouterModels.value.length && !openRouterModelsLoading.value) {
    refreshOpenRouterModels()
  }
}

function selectOpenRouterChatModel(modelId) {
  selectedOpenRouterChatModel.value = String(modelId || '').trim()
}

async function changeBillingDate(date) {
  await refreshBilling(date)
}

function isTaskAutomationLinuxDOMode() {
  const mode = String(config.taskAutomationLaunchMode || '').trim().toLowerCase()
  return mode === 'linuxdo' || mode === 'linux.do' || mode === 'linux-do' || mode === 'browser'
}

async function refreshTaskAutomationBrowserProfiles(browser = config.taskAutomationBrowser) {
  if (!isTaskAutomationLinuxDOMode()) {
    taskAutomationBrowserProfileSeq += 1
    taskAutomationBrowserProfiles.value = []
    taskAutomationBrowserProfilesError.value = ''
    taskAutomationBrowserProfilesLoading.value = false
    return
  }
  const seq = ++taskAutomationBrowserProfileSeq
  taskAutomationBrowserProfilesLoading.value = true
  taskAutomationBrowserProfilesError.value = ''
  try {
    const profiles = await getTaskAutomationBrowserProfiles(browser || 'default')
    if (seq !== taskAutomationBrowserProfileSeq) return
    taskAutomationBrowserProfiles.value = Array.isArray(profiles) ? profiles : []
  } catch (error) {
    if (seq !== taskAutomationBrowserProfileSeq) return
    taskAutomationBrowserProfiles.value = []
    taskAutomationBrowserProfilesError.value = error.message
  } finally {
    if (seq === taskAutomationBrowserProfileSeq) {
      taskAutomationBrowserProfilesLoading.value = false
    }
  }
}

function openBillingView() {
  if (activeTab.value === 'billing') {
    refreshBilling()
    return
  }
  selectTab('billing')
}

function openFirstTokenForm() {
  selectTab('tokens')
  openCreateForm('openai')
}

function closeFirstUseGuide() {
  firstUseGuideVisible.value = false
  window.localStorage?.setItem(firstUseGuideStorageKey, '1')
}

function previousFirstUseGuideStep() {
  firstUseGuideStepIndex.value = Math.max(0, firstUseGuideStepIndex.value - 1)
}

function nextFirstUseGuideStep() {
  if (firstUseGuideStepIndex.value >= firstUseGuideSteps.length - 1) {
    closeFirstUseGuide()
    return
  }
  firstUseGuideStepIndex.value += 1
}

function runFirstUseGuideAction() {
  const step = currentFirstUseGuideStep.value
  closeFirstUseGuide()
  if (step.actionKey === 'tokens') {
    openFirstTokenForm()
  } else if (step.actionKey === 'proxy') {
    if (!proxyStatus.running) {
      toggleProxy()
    }
  } else if (step.actionKey === 'quickstart') {
    selectTab('quickstart')
  }
}

async function copyEndpointValue(value, label) {
  const text = String(value || '').trim()
  if (!text) return
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text)
    } else {
      const textarea = document.createElement('textarea')
      textarea.value = text
      textarea.setAttribute('readonly', '')
      textarea.style.position = 'fixed'
      textarea.style.opacity = '0'
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand('copy')
      document.body.removeChild(textarea)
    }
    successMessage.value = `${label || '内容'}已复制`
  } catch (error) {
    errorMessage.value = `复制失败：${error.message}`
  }
}

const providerBaseUrlKeys = new Set(['sub2api', 'newapi', 'anyrouter'])

function providerRequiresBaseUrl(provider) {
  return providerBaseUrlKeys.has(String(provider || '').trim())
}

function providerDefaultBaseUrl(provider) {
  if (provider === 'sub2api') return config.sub2apiBaseUrl
  if (provider === 'newapi') return config.newapiBaseUrl
  if (provider === 'anyrouter') return config.anyrouterBaseUrl
  return ''
}

function validateProviderBaseUrl(provider, baseUrl) {
  if (!providerRequiresBaseUrl(provider)) return true
  const label = providerLabel(provider)
  if (!baseUrl) {
    errorMessage.value = `${label} Base URL 不能为空`
    return false
  }
  try {
    const parsed = new URL(baseUrl)
    if (!['http:', 'https:'].includes(parsed.protocol) || !parsed.host) {
      errorMessage.value = `${label} Base URL 格式不正确`
      return false
    }
  } catch {
    errorMessage.value = `${label} Base URL 格式不正确`
    return false
  }
  return true
}

function openCreateForm(provider = 'openai') {
  Object.assign(form, {
    visible: true,
    editingId: '',
    name: '',
    provider,
    originalProvider: provider,
    credentialType: 'api_key',
    originalCredentialType: 'api_key',
    region: 'cn',
    baseUrl: providerDefaultBaseUrl(provider),
    tokenValue: '',
  })
}

function openEditForm(token) {
  Object.assign(form, {
    visible: true,
    editingId: token.id,
    name: token.name,
    provider: token.provider,
    originalProvider: token.provider,
    credentialType: token.credentialType || 'api_key',
    originalCredentialType: token.credentialType || 'api_key',
    region: token.region || 'cn',
    baseUrl: token.baseUrl || '',
    tokenValue: '',
  })
}

function closeForm() {
  form.visible = false
}

async function submitForm() {
  errorMessage.value = ''
  successMessage.value = ''
  const name = isAutoNameForm.value ? '' : form.name.trim()
  const tokenValue = form.tokenValue.trim()
  const provider = form.provider.trim() || 'openai'
  const credentialType = normalizedCredentialType(provider, form.credentialType)
  const region = provider === 'xiaomi' && credentialType === 'mimo_token_plan' ? form.region || 'cn' : ''
  const baseUrl = providerRequiresBaseUrl(provider) ? form.baseUrl.trim() : ''
  const isEditing = Boolean(form.editingId)
  const replacingCredential = tokenValue !== ''

  if (!isAutoNameForm.value && !name) {
    errorMessage.value = '账号名称不能为空'
    return
  }
  const duplicate = tokens.value.some(
    (item) =>
      !isAutoNameForm.value &&
      item.id !== form.editingId &&
      item.provider === provider &&
      item.name.toLowerCase() === name.toLowerCase(),
  )
  if (duplicate) {
    errorMessage.value = '同一厂商下账号名称不可重复'
    return
  }
  if (
    isEditing &&
    !replacingCredential &&
    (provider !== form.originalProvider || credentialType !== form.originalCredentialType)
  ) {
    errorMessage.value = '更改厂商或凭据类型时需要重新填写凭据'
    return
  }
  if (!validateProviderBaseUrl(provider, baseUrl)) {
    return
  }
  if ((credentialType === 'codex_auth_json' || credentialType === 'claude_oauth_json') && (!isEditing || replacingCredential)) {
    try {
      const parsed = JSON.parse(tokenValue)
      if (
        credentialType === 'claude_oauth_json' &&
        !parsed.access_token &&
        !parsed.accessToken &&
        !parsed.refresh_token &&
        !parsed.refreshToken &&
        !parsed.claudeAiOauth
      ) {
        errorMessage.value = 'Claude OAuth JSON 需要包含 access_token 或 refresh_token'
        return
      }
    } catch {
      errorMessage.value = credentialType === 'claude_oauth_json' ? 'Claude OAuth JSON 不是有效 JSON' : 'Codex auth.json 内容不是有效 JSON'
      return
    }
  } else if (replacingCredential && provider === 'xiaomi' && credentialType === 'mimo_token_plan' && !tokenValue.startsWith('tp-')) {
    errorMessage.value = 'MiMo Token Plan Key 必须以 tp- 开头'
    return
  } else if (replacingCredential && provider === 'xiaomi' && credentialType === 'api_key' && !tokenValue.startsWith('sk-')) {
    errorMessage.value = 'MiMo 按量 API Key 必须以 sk- 开头'
    return
  } else if (replacingCredential && provider === 'tokenrouter' && !tokenValue.startsWith('tr_')) {
    errorMessage.value = 'TokenRouter API Key 必须以 tr_ 开头'
    return
  } else if ((!isEditing || replacingCredential) && tokenValue.length < 12) {
    errorMessage.value = 'Token 长度过短'
    return
  }

  const payload = {
    name,
    provider,
    credentialType,
    region,
    baseUrl,
    tokenValue,
  }

  try {
    if (form.editingId) {
      await updateToken(form.editingId, payload)
    } else {
      await createToken(payload)
    }
    closeForm()
    await refreshAll()
    if (provider === 'openrouter') {
      await refreshOpenRouterModels({ force: true })
    }
    successMessage.value = form.editingId ? '账号已更新' : '账号已添加'
  } catch (error) {
    errorMessage.value = error.message
  }
}

function openBatchImport(provider = 'openai') {
  Object.assign(batchImportForm, {
    visible: true,
    provider,
    baseUrl: providerDefaultBaseUrl(provider),
    tokenText: '',
  })
}

function closeBatchImport() {
  if (batchImporting.value) return
  batchImportForm.visible = false
}

function onBatchImportProviderChange() {
  if (providerRequiresBaseUrl(batchImportForm.provider) && !batchImportForm.baseUrl) {
    batchImportForm.baseUrl = providerDefaultBaseUrl(batchImportForm.provider)
  } else if (!providerRequiresBaseUrl(batchImportForm.provider)) {
    batchImportForm.baseUrl = ''
  }
}

function batchImportPlaceholder() {
  if (batchImportForm.provider === 'zo') {
    return [
      'zo_sk_xxxxxxxxxxxxxxxxxxxxxxxx',
      'zo_sk_yyyyyyyyyyyyyyyyyyyyyyyy',
    ].join('\n')
  }
  if (batchImportForm.provider === 'prem') {
    return [
      'prem-key-xxxxxxxxxxxxxxxxxxxxxxxx',
      'prem-key-yyyyyyyyyyyyyyyyyyyyyyyy',
    ].join('\n')
  }
  return [
    'sk-aa0aeaf480484648a8a93d672d76334d  # balance: 10.14 CNY',
    'sk-460d28e38c7e4b05a13fa2bebd27159c  # balance: 0.24 USD',
    'sk-3d7acb8511ad4da18e8b0c89733f472b  # balance: 7.18 USD',
  ].join('\n')
}

async function submitBatchImport() {
  errorMessage.value = ''
  successMessage.value = ''
  const provider = batchImportForm.provider.trim() || 'openai'
  const baseUrl = providerRequiresBaseUrl(provider) ? batchImportForm.baseUrl.trim() : ''
  const tokenText = batchImportForm.tokenText.trim()

  if (!tokenText) {
    errorMessage.value = '请先粘贴要导入的 API Key'
    return
  }
  if (!validateProviderBaseUrl(provider, baseUrl)) {
    return
  }

  batchImporting.value = true
  try {
    const result = await importAPIKeys({
      provider,
      credentialType: 'api_key',
      region: '',
      baseUrl,
      tokenText,
    })
    batchImportForm.visible = false
    activeProvider.value = provider
    await refreshAll()
    if (provider === 'openrouter' && result.createdCount) {
      await refreshOpenRouterModels({ force: true })
    }

    const created = result.createdCount || 0
    const skipped = result.skipped?.length || 0
    if (created > 0) {
      successMessage.value = `已导入 ${created} 个 API Key${skipped ? `，跳过 ${skipped} 行` : ''}`
    } else {
      errorMessage.value = skipped ? `没有导入新的 API Key，已跳过 ${skipped} 行` : '没有导入新的 API Key'
    }
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    batchImporting.value = false
  }
}

async function removeToken(token) {
  deleteCandidate.value = token
}

function closeDeleteConfirm() {
  if (deleteBusy.value) return
  deleteCandidate.value = null
}

async function confirmRemoveToken() {
  if (!deleteCandidate.value?.id) return
  const target = deleteCandidate.value
  deleteBusy.value = true
  try {
    await deleteToken(target.id)
    await refreshAll()
    successMessage.value = `账号已删除：${target.name}`
    deleteCandidate.value = null
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    deleteBusy.value = false
  }
}

function replaceToken(updated) {
  if (!updated?.id) return
  tokens.value = tokens.value.map((item) => (item.id === updated.id ? updated : item))
}

async function toggleTokenEnabled(token, enabled = Boolean(token.disabled)) {
  errorMessage.value = ''
  successMessage.value = ''
  togglingTokenIds[token.id] = true
  try {
    const nextEnabled = Boolean(enabled)
    const updated = await setTokenDisabled(token.id, !nextEnabled)
    replaceToken(updated)
    successMessage.value = nextEnabled ? `已启用账号：${updated.name}` : `已停用账号：${updated.name}`
  } catch (error) {
    errorMessage.value = error.message
    await refreshRealtime()
  } finally {
    togglingTokenIds[token.id] = false
  }
}

function providerSelectedTokens(provider) {
  return providerTokens(provider).filter((item) => item.selected)
}

async function toggleTokenSelected(token) {
  if (!token?.id) return
  if (token.disabled) {
    errorMessage.value = '已停用账号需要先在账号管理中启用'
    return
  }
  errorMessage.value = ''
  successMessage.value = ''
  switchingOnlyTokenIds[token.id] = true
  try {
    const nextSelected = !token.selected
    const updatedTokens = await setTokenSelected(token.id, nextSelected)
    if (Array.isArray(updatedTokens)) {
      tokens.value = updatedTokens
    } else {
      await refreshRealtime()
    }
    const selectedCount = Array.isArray(updatedTokens)
      ? updatedTokens.filter((item) => item.provider === token.provider && item.selected).length
      : providerSelectedTokens(token.provider).length
    if (nextSelected) {
      successMessage.value = `已选择 ${providerLabel(token.provider)} 账号：${token.name}`
    } else if (selectedCount > 0) {
      successMessage.value = `已取消选择 ${token.name}，${providerLabel(token.provider)} 仍仅使用已选账号`
    } else {
      successMessage.value = `已恢复 ${providerLabel(token.provider)} 默认轮换`
    }
  } catch (error) {
    errorMessage.value = error.message
    await refreshRealtime()
  } finally {
    switchingOnlyTokenIds[token.id] = false
  }
}

async function verifyToken(token) {
  errorMessage.value = ''
  successMessage.value = ''
  validatingIds[token.id] = true
  try {
    const result = await validateToken(token.id)
    await refreshRealtime()
    if (result.ok) {
      successMessage.value = validationSuccessMessage(token, result)
    } else {
      errorMessage.value = `验证未通过：${result.status || '-'} ${result.message || ''}`
    }
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    validatingIds[token.id] = false
  }
}

async function refreshAuthToken(token) {
  if (!isCodexToken(token)) {
    errorMessage.value = '当前账号不支持刷新令牌'
    return
  }
  errorMessage.value = ''
  successMessage.value = ''
  refreshingTokenIds[token.id] = true
  try {
    const updated = await refreshTokenAuth(token.id)
    replaceToken(updated)
    await refreshRealtime()
    successMessage.value = `令牌已刷新：${updated.name}`
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    refreshingTokenIds[token.id] = false
  }
}

function openCodexAuthFilePicker() {
  errorMessage.value = ''
  successMessage.value = ''
}

async function importCodexAuthFiles(event) {
  const fileInput = event.target
  const files = Array.from(fileInput.files || [])
  fileInput.value = ''
  if (!files.length) {
    return
  }

  activeProvider.value = 'openai'
  errorMessage.value = ''
  successMessage.value = ''
  codexAuthImporting.value = true

  const knownCodexTokens = new Map()
  const knownOpenAITokens = new Map()
  tokens.value.forEach((item) => {
    if (item.provider !== 'openai') return
    knownOpenAITokens.set(item.name.toLowerCase(), item)
    if (isCodexToken(item)) {
      knownCodexTokens.set(item.name.toLowerCase(), item)
    }
  })

  const summary = {
    created: 0,
    updated: 0,
    failed: [],
  }

  try {
    for (const file of files) {
      try {
        const tokenValue = (await file.text()).trim()
        const email = codexEmailFromAuthJSON(tokenValue)
        const key = email.toLowerCase()
        const sameNameToken = knownOpenAITokens.get(key)
        if (sameNameToken && !isCodexToken(sameNameToken)) {
          throw new Error(`同名 OpenAI 账号已存在，且不是 Codex auth.json`)
        }

        const payload = {
          name: '',
          provider: 'openai',
          credentialType: 'codex_auth_json',
          tokenValue,
        }
        const existing = knownCodexTokens.get(key)
        if (existing) {
          const updated = await updateToken(existing.id, payload)
          knownCodexTokens.set(key, updated)
          knownOpenAITokens.set(key, updated)
          summary.updated += 1
        } else {
          const created = await createToken(payload)
          knownCodexTokens.set(key, created)
          knownOpenAITokens.set(key, created)
          summary.created += 1
        }
      } catch (error) {
        summary.failed.push(`${file.name}: ${error.message}`)
      }
    }

    await refreshAll()
    const importedCount = summary.created + summary.updated
    if (importedCount) {
      const parts = []
      if (summary.created) parts.push(`新增 ${summary.created} 个`)
      if (summary.updated) parts.push(`更新 ${summary.updated} 个`)
      successMessage.value = `Codex auth 文件导入完成：${parts.join('，')}`
    }
    if (summary.failed.length) {
      errorMessage.value = `导入失败 ${summary.failed.length} 个：${summary.failed.slice(0, 3).join('；')}`
    }
    if (!importedCount && !summary.failed.length) {
      successMessage.value = '没有可导入的 auth 文件'
    }
  } finally {
    codexAuthImporting.value = false
  }
}

function codexEmailFromAuthJSON(text) {
  let data
  try {
    data = JSON.parse(text)
  } catch {
    throw new Error('不是有效 JSON')
  }

  const type = codexStringField(data?.type)
  if (type && type.toLowerCase() !== 'codex') {
    throw new Error('不是 Codex auth JSON')
  }
  if (!codexAuthSecretFromData(data)) {
    throw new Error('缺少可用的 access_token 或 id_token')
  }

  const directEmail = codexStringField(data?.email)
  if (directEmail) {
    return directEmail
  }

  const idToken = codexIDTokenFromData(data)
  if (typeof idToken !== 'string' || !idToken.trim()) {
    throw new Error('缺少 email，且无法从 id_token 解析邮箱')
  }
  const parts = idToken.split('.')
  if (parts.length !== 3) {
    throw new Error('id_token 格式不正确')
  }

  let payload
  try {
    payload = JSON.parse(decodeBase64URL(parts[1]))
  } catch {
    throw new Error('无法解析 id_token')
  }

  const email = payload?.['https://api.openai.com/profile']?.email || payload?.email
  if (typeof email !== 'string' || !email.trim()) {
    throw new Error('id_token 中没有邮箱')
  }
  return email.trim()
}

function codexIDTokenFromData(data) {
  return codexStringField(data?.tokens?.id_token) || codexStringField(data?.id_token)
}

function codexAuthSecretFromData(data) {
  return (
    codexStringField(data?.tokens?.access_token) ||
    codexStringField(data?.access_token) ||
    codexStringField(data?.OPENAI_API_KEY) ||
    codexIDTokenFromData(data)
  )
}

function codexStringField(value) {
  return typeof value === 'string' ? value.trim() : ''
}

function decodeBase64URL(value) {
  const normalized = value.replace(/-/g, '+').replace(/_/g, '/')
  const padded = normalized.padEnd(normalized.length + ((4 - (normalized.length % 4)) % 4), '=')
  const binary = window.atob(padded)
  const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0))
  return new TextDecoder().decode(bytes)
}

async function persistConfig() {
  try {
    const saved = await saveConfig({
      proxyPort: Number(config.proxyPort),
      controlPort: Number(config.controlPort),
      schedulingMode: config.schedulingMode,
      websocketMode: config.websocketMode,
      checkBetaUpdates: Boolean(config.checkBetaUpdates),
      taskAutomationEnabled: Boolean(config.taskAutomationEnabled),
      taskAutomationClients: Array.isArray(config.taskAutomationClients) ? config.taskAutomationClients : [],
      taskAutomationLaunchMode: String(config.taskAutomationLaunchMode || '').trim(),
      taskAutomationLaunchTarget: config.taskAutomationLaunchTarget.trim(),
      taskAutomationFallbackUrl: config.taskAutomationFallbackUrl.trim(),
      taskAutomationBrowser: String(config.taskAutomationBrowser || '').trim(),
      taskAutomationBrowserUserDataDir: String(config.taskAutomationBrowserUserDataDir || '').trim(),
      taskAutomationBrowserProfile: String(config.taskAutomationBrowserProfile || '').trim(),
      taskAutomationReturnToClient: Boolean(config.taskAutomationReturnToClient),
      taskAutomationIdleSeconds: Number(config.taskAutomationIdleSeconds),
      taskAutomationReturnDelaySeconds: Number(config.taskAutomationReturnDelaySeconds),
      outboundProxyEnabled: Boolean(config.outboundProxyEnabled),
      outboundProxyUrl: config.outboundProxyUrl.trim(),
      outboundProxyProviders: Array.isArray(config.outboundProxyProviders) ? config.outboundProxyProviders : [],
      outboundProxyModels: Array.isArray(config.outboundProxyModels) ? config.outboundProxyModels : [],
      upstreamBaseUrl: config.upstreamBaseUrl.trim(),
      openaiBaseUrl: config.openaiBaseUrl.trim(),
      anthropicBaseUrl: config.anthropicBaseUrl.trim(),
      deepseekBaseUrl: config.deepseekBaseUrl.trim(),
      deepseekAnthropicBaseUrl: config.deepseekAnthropicBaseUrl.trim(),
      kimiBaseUrl: config.kimiBaseUrl.trim(),
      zhipuBaseUrl: config.zhipuBaseUrl.trim(),
      zhipuAnthropicBaseUrl: config.zhipuAnthropicBaseUrl.trim(),
      minimaxBaseUrl: config.minimaxBaseUrl.trim(),
      minimaxAnthropicBaseUrl: config.minimaxAnthropicBaseUrl.trim(),
      geminiBaseUrl: config.geminiBaseUrl.trim(),
      openrouterBaseUrl: config.openrouterBaseUrl.trim(),
      tokenrouterBaseUrl: config.tokenrouterBaseUrl.trim(),
      sub2apiBaseUrl: config.sub2apiBaseUrl.trim(),
      newapiBaseUrl: config.newapiBaseUrl.trim(),
      zoBaseUrl: config.zoBaseUrl.trim(),
      customGatewayBaseUrl: config.customGatewayBaseUrl.trim(),
      customGatewayAnthropicBaseUrl: config.customGatewayAnthropicBaseUrl.trim(),
      xiaomiBaseUrl: config.xiaomiBaseUrl.trim(),
      xiaomiApiBaseUrl: config.xiaomiApiBaseUrl.trim(),
      xiaomiApiAnthropicBaseUrl: config.xiaomiApiAnthropicBaseUrl.trim(),
      xiaomiTokenPlanBaseUrl: config.xiaomiTokenPlanBaseUrl.trim(),
      xiaomiTokenPlanAnthropicBaseUrl: config.xiaomiTokenPlanAnthropicBaseUrl.trim(),
      xiaomiTokenPlanSgpBaseUrl: config.xiaomiTokenPlanSgpBaseUrl.trim(),
      xiaomiTokenPlanSgpAnthropicBaseUrl: config.xiaomiTokenPlanSgpAnthropicBaseUrl.trim(),
      xiaomiTokenPlanAmsBaseUrl: config.xiaomiTokenPlanAmsBaseUrl.trim(),
      xiaomiTokenPlanAmsAnthropicBaseUrl: config.xiaomiTokenPlanAmsAnthropicBaseUrl.trim(),
      xiaomiCredentialPriority: config.xiaomiCredentialPriority,
      codexBaseUrl: config.codexBaseUrl.trim(),
      codexUsageEndpoint: config.codexUsageEndpoint.trim(),
      switchThreshold: Number(config.switchThreshold),
      maxRetries: Number(config.maxRetries),
      historyRetentionDays: Number(config.historyRetentionDays),
    })
    Object.assign(config, saved)
    await refreshRealtime()
    successMessage.value = '设置已保存'
  } catch (error) {
    errorMessage.value = error.message
  }
}

async function clearBillingUsageData() {
  try {
    await ElMessageBox.confirm(
      '将删除本地账单汇总和累计代理 Token 统计，详细请求历史不会删除。此操作无法撤销。',
      '清空账单汇总',
      {
        confirmButtonText: '清空汇总',
        cancelButtonText: '取消',
        type: 'warning',
      },
    )
    clearingBillingUsage.value = true
    errorMessage.value = ''
    await clearBillingUsage()
    billingUsage.value = []
    billingDates.value = []
    billingSummary.value = emptyBillingSummary()
    successMessage.value = '账单汇总已清空'
  } catch (action) {
    if (action instanceof Error) {
      errorMessage.value = action.message
    }
  } finally {
    clearingBillingUsage.value = false
  }
}

async function clearRequestHistoryData() {
  try {
    await ElMessageBox.confirm(
      '将删除本地请求历史明细，已保存的每日汇总会保留。此操作无法撤销。',
      '清空请求历史',
      {
        confirmButtonText: '清空历史',
        cancelButtonText: '取消',
        type: 'warning',
      },
    )
    clearingRequestHistory.value = true
    errorMessage.value = ''
    await clearRequestHistory()
    await Promise.all([refreshHistory(), refreshBilling()])
    successMessage.value = '请求历史已清空'
  } catch (action) {
    if (action instanceof Error) {
      errorMessage.value = action.message
    }
  } finally {
    clearingRequestHistory.value = false
  }
}

async function chooseDataDirectory() {
  dataDirChanging.value = true
  errorMessage.value = ''
  try {
    const result = await chooseDataDirectoryWithDialog(true)
    if (result.cancelled) {
      return
    }
    Object.assign(dataDirectory, {
      bootstrapPath: result.bootstrapPath,
      envOverride: result.envOverride,
      pendingDataDir: result.dataDir,
      restartRequired: result.restartRequired,
    })
    if (!result.restartRequired) {
      dataDirectory.dataDir = result.dataDir
      successMessage.value = '数据目录已保存'
      return
    }
    const copied = result.migratedFiles?.length ? `，已迁移 ${result.migratedFiles.join('、')}` : ''
    const skipped = result.skippedFiles?.length ? `，目标目录已有 ${result.skippedFiles.join('、')} 未覆盖` : ''
    successMessage.value = `数据目录已保存，重启 OmniProxy 后生效${copied}${skipped}`
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    dataDirChanging.value = false
  }
}

async function configureLocalCodex() {
  errorMessage.value = ''
  successMessage.value = ''
  codexConfiguring.value = true
  try {
    const result = await configureCodex()
    await refreshAll()
    successMessage.value = result.message || 'Codex 已配置为使用 OmniProxy'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    codexConfiguring.value = false
  }
}

async function configureLocalCodexSub2API() {
  errorMessage.value = ''
  successMessage.value = ''
  codexSub2APIConfiguring.value = true
  try {
    const result = await configureCodexSub2API()
    await refreshAll()
    successMessage.value = result.message || 'Codex 已配置为使用 OmniProxy sub2api'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    codexSub2APIConfiguring.value = false
  }
}

async function configureLocalCodexNewAPI() {
  errorMessage.value = ''
  successMessage.value = ''
  codexNewAPIConfiguring.value = true
  try {
    const result = await configureCodexNewAPI()
    await refreshAll()
    successMessage.value = result.message || 'Codex 已配置为使用 OmniProxy new-api'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    codexNewAPIConfiguring.value = false
  }
}

async function configureLocalCodexAnyRouter() {
  errorMessage.value = ''
  successMessage.value = ''
  codexAnyRouterConfiguring.value = true
  try {
    const result = await configureCodexAnyRouter()
    await refreshAll()
    successMessage.value = result.message || 'Codex 已配置为使用 OmniProxy AnyRouter'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    codexAnyRouterConfiguring.value = false
  }
}

async function configureLocalCodexZo() {
  errorMessage.value = ''
  successMessage.value = ''
  codexZoConfiguring.value = true
  try {
    const result = await configureCodexZo()
    await refreshAll()
    successMessage.value = result.message || 'Codex 已配置为使用 OmniProxy Zo Computer'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    codexZoConfiguring.value = false
  }
}

async function configureLocalCodexPrem() {
  errorMessage.value = ''
  successMessage.value = ''
  codexPremConfiguring.value = true
  try {
    const result = await configureCodexPrem()
    await refreshAll()
    successMessage.value = result.message || 'Codex 已配置为使用 OmniProxy Prem'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    codexPremConfiguring.value = false
  }
}

async function restoreLocalCodex() {
  errorMessage.value = ''
  successMessage.value = ''
  codexRestoring.value = true
  try {
    const result = await restoreCodex()
    successMessage.value = result.message || 'Codex 配置已恢复'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    codexRestoring.value = false
  }
}

async function configureLocalMimoClaude() {
  errorMessage.value = ''
  successMessage.value = ''
  mimoClaudeConfiguring.value = true
  try {
    const result = await configureMimoClaude()
    successMessage.value = result.message || 'Claude Code 已配置为使用 Xiaomi MiMo'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    mimoClaudeConfiguring.value = false
  }
}

async function configureLocalDeepSeekClaude() {
  errorMessage.value = ''
  successMessage.value = ''
  deepSeekClaudeConfiguring.value = true
  try {
    const result = await configureDeepSeekClaude()
    successMessage.value = result.message || 'Claude Code 已配置为使用 DeepSeek'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    deepSeekClaudeConfiguring.value = false
  }
}

async function configureLocalKimiClaude() {
  errorMessage.value = ''
  successMessage.value = ''
  kimiClaudeConfiguring.value = true
  try {
    const result = await configureKimiClaude()
    successMessage.value = result.message || 'Claude Code 已配置为使用 Kimi'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    kimiClaudeConfiguring.value = false
  }
}

async function configureLocalZhipuClaude() {
  errorMessage.value = ''
  successMessage.value = ''
  zhipuClaudeConfiguring.value = true
  try {
    const result = await configureZhipuClaude()
    successMessage.value = result.message || 'Claude Code 已配置为使用 Zhipu GLM'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    zhipuClaudeConfiguring.value = false
  }
}

async function configureLocalAnyRouterClaude() {
  errorMessage.value = ''
  successMessage.value = ''
  anyRouterClaudeConfiguring.value = true
  try {
    const result = await configureAnyRouterClaude()
    successMessage.value = result.message || 'Claude Code 已配置为使用 AnyRouter'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    anyRouterClaudeConfiguring.value = false
  }
}

async function configureLocalZoClaude() {
  errorMessage.value = ''
  successMessage.value = ''
  zoClaudeConfiguring.value = true
  try {
    const result = await configureZoClaude()
    successMessage.value = result.message || 'Claude Code 已配置为使用 Zo Computer'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    zoClaudeConfiguring.value = false
  }
}

async function configureLocalPremClaude() {
  errorMessage.value = ''
  successMessage.value = ''
  premClaudeConfiguring.value = true
  try {
    const result = await configurePremClaude()
    successMessage.value = result.message || 'Claude Code 已配置为使用 Prem'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    premClaudeConfiguring.value = false
  }
}

function claudeModelLabel(modelId) {
  return claudeModelOptions.find((option) => option.id === modelId)?.label || modelId
}

function isClaudeModelOptionDisabled(modelId) {
  return selectedClaudeModels.value.length >= claudeModelSelectionLimit && !selectedClaudeModels.value.includes(modelId)
}

function selectedClaudeModelIds() {
  return selectedClaudeModels.value.map((model) => String(model || '').trim()).filter(Boolean)
}

function validateSelectedClaudeModels() {
  const models = selectedClaudeModelIds()
  if (models.length === 0) {
    errorMessage.value = '至少选择一个 Claude Code 模型'
    return null
  }
  if (models.length > claudeModelSelectionLimit) {
    errorMessage.value = `Claude Code 最多选择 ${claudeModelSelectionLimit} 个模型`
    return null
  }
  return models
}

async function configureLocalClaudeModels() {
  errorMessage.value = ''
  successMessage.value = ''
  const models = validateSelectedClaudeModels()
  if (!models) return
  claudeModelsConfiguring.value = true
  try {
    const result = await configureClaudeModels(models)
    successMessage.value = result.message || 'Claude Code 已按选择模型完成配置'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    claudeModelsConfiguring.value = false
  }
}

async function configureLocalClaudeDesktopModels() {
  errorMessage.value = ''
  successMessage.value = ''
  const models = validateSelectedClaudeModels()
  if (!models) return
  claudeDesktopConfiguring.value = true
  try {
    const result = await configureClaudeDesktopModels(models)
    successMessage.value = result.message || 'Claude Code Desktop 已按选择模型完成配置，请重启 Claude Desktop'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    claudeDesktopConfiguring.value = false
  }
}

async function restoreLocalClaudeDesktop() {
  errorMessage.value = ''
  successMessage.value = ''
  claudeDesktopRestoring.value = true
  try {
    const result = await restoreClaudeDesktop()
    successMessage.value = result.message || 'Claude Desktop 配置已恢复'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    claudeDesktopRestoring.value = false
  }
}

async function configureLocalGemini() {
  errorMessage.value = ''
  successMessage.value = ''
  geminiConfiguring.value = true
  try {
    const result = await configureGemini()
    successMessage.value = result.message || 'Gemini CLI 已配置为使用 OmniProxy'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    geminiConfiguring.value = false
  }
}

async function configureLocalDeepSeekTUI() {
  errorMessage.value = ''
  successMessage.value = ''
  deepSeekTUIConfiguring.value = true
  try {
    const result = await configureDeepSeekTUI()
    successMessage.value = result.message || 'DeepSeek-TUI 已配置为使用 OmniProxy'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    deepSeekTUIConfiguring.value = false
  }
}

async function restoreLocalDeepSeekTUI() {
  errorMessage.value = ''
  successMessage.value = ''
  deepSeekTUIRestoring.value = true
  try {
    const result = await restoreDeepSeekTUI()
    successMessage.value = result.message || 'DeepSeek-TUI 配置已恢复'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    deepSeekTUIRestoring.value = false
  }
}

async function restoreLocalGemini() {
  errorMessage.value = ''
  successMessage.value = ''
  geminiRestoring.value = true
  try {
    const result = await restoreGemini()
    successMessage.value = result.message || 'Gemini CLI 配置已恢复'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    geminiRestoring.value = false
  }
}

async function configureLocalOpenCode() {
  errorMessage.value = ''
  successMessage.value = ''
  opencodeConfiguring.value = true
  try {
    const result = await configureOpenCode()
    successMessage.value = result.message || 'OpenCode 已配置为使用 OmniProxy'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    opencodeConfiguring.value = false
  }
}

async function restoreLocalOpenCode() {
  errorMessage.value = ''
  successMessage.value = ''
  opencodeRestoring.value = true
  try {
    const result = await restoreOpenCode()
    successMessage.value = result.message || 'OpenCode 配置已恢复'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    opencodeRestoring.value = false
  }
}

async function configureLocalPi() {
  errorMessage.value = ''
  successMessage.value = ''
  piConfiguring.value = true
  try {
    const result = await configurePi()
    successMessage.value = result.message || 'Pi Coding Agent 已配置为使用 OmniProxy'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    piConfiguring.value = false
  }
}

async function restoreLocalPi() {
  errorMessage.value = ''
  successMessage.value = ''
  piRestoring.value = true
  try {
    const result = await restorePi()
    successMessage.value = result.message || 'Pi Coding Agent 配置已恢复'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    piRestoring.value = false
  }
}

async function restoreLocalMimoClaude() {
  errorMessage.value = ''
  successMessage.value = ''
  mimoClaudeRestoring.value = true
  try {
    const result = await restoreMimoClaude()
    successMessage.value = result.message || 'Claude Code 配置已恢复'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    mimoClaudeRestoring.value = false
  }
}

async function restoreLocalZhipuClaude() {
  errorMessage.value = ''
  successMessage.value = ''
  mimoClaudeRestoring.value = true
  try {
    const result = await restoreZhipuClaude()
    successMessage.value = result.message || 'Claude Code 配置已恢复'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    mimoClaudeRestoring.value = false
  }
}

async function toggleProxy() {
  try {
    if (proxyStatus.running) {
      await stopProxy()
    } else {
      await startProxy()
    }
    await refreshRealtime()
    successMessage.value = proxyStatus.running ? '代理已启动' : '代理已停止'
  } catch (error) {
    errorMessage.value = error.message
  }
}

async function toggleAutoStart() {
  autoStartChanging.value = true
  errorMessage.value = ''
  successMessage.value = ''
  try {
    const next = !autoStartEnabled.value
    const result = await setAutoStart(next)
    autoStartEnabled.value = Boolean(result?.enabled)
    successMessage.value = autoStartEnabled.value ? '已启用开机自启' : '已关闭开机自启'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    autoStartChanging.value = false
  }
}

async function exportRequestHistory(payload) {
  const format = payload?.format
  const filters = payload?.filters || {}
  const entries = payload?.entries || []
  if (!entries.length) {
    errorMessage.value = '当前筛选条件下没有可导出的请求历史'
    return
  }
  exportingHistory.value = format
  errorMessage.value = ''
  successMessage.value = ''
  try {
    const path = await exportHistory(format, filters, entries)
    if (path) {
      successMessage.value = `请求历史已导出为 ${format.toUpperCase()}`
    }
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    exportingHistory.value = ''
  }
}

async function exportTokenBackup() {
  exportingTokens.value = true
  errorMessage.value = ''
  successMessage.value = ''
  try {
    const result = await exportTokens()
    if (result?.path) {
      successMessage.value = result.message || `账号池已导出：${result.count || 0} 个账号`
    }
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    exportingTokens.value = false
  }
}

async function exportCodexAuthBackups() {
  exportingCodexAuth.value = true
  errorMessage.value = ''
  successMessage.value = ''
  try {
    const result = await exportCodexAuthFiles()
    if (result?.directory) {
      successMessage.value = result.message || `Codex auth.json 已导出：${result.count || 0} 个文件`
    }
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    exportingCodexAuth.value = false
  }
}

function credentialDisplay(item) {
  if (item.maskedTokenValue) return item.maskedTokenValue
  if (item.credentialType === 'codex_auth_json') return 'auth.json'
  if (item.credentialType === 'claude_oauth_json') return 'OAuth JSON'
  return item.hasTokenValue ? '已保存' : '-'
}

function credentialPlaceholder() {
  if (form.editingId) {
    return '留空表示保留当前凭据'
  }
  if (form.credentialType === 'codex_auth_json') {
    return '粘贴 ~/.codex/auth.json 或 CLIProxyAPI Codex JSON 的完整内容'
  }
  if (form.credentialType === 'claude_oauth_json') {
    return '粘贴 Claude OAuth JSON，需包含 access_token / refresh_token / expired'
  }
  if (form.credentialType === 'mimo_token_plan') {
    return '粘贴 tp- 开头的 MiMo Token Plan Key'
  }
  if (form.credentialType === 'coding_plan') {
    return '粘贴 GLM Coding Plan Key'
  }
  if (form.provider === 'xiaomi') {
    return '粘贴 sk- 开头的 MiMo 按量 API Key'
  }
  if (form.provider === 'kimi') {
    return '粘贴 Kimi Code API Key'
  }
  if (form.provider === 'zhipu') {
    return '粘贴 Zhipu GLM API Key，格式通常为 id.secret'
  }
  if (form.provider === 'minimax') {
    return '粘贴 MiniMax API Key'
  }
  if (form.provider === 'gemini') {
    return '粘贴 Google Gemini API Key'
  }
  if (form.provider === 'openrouter') {
    return '粘贴 OpenRouter API Key'
  }
  if (form.provider === 'tokenrouter') {
    return '粘贴 tr_ 开头的 TokenRouter API Key'
  }
  if (form.provider === 'sub2api') {
    return '粘贴 sub2api API Key'
  }
  if (form.provider === 'newapi') {
    return '粘贴 new-api API Key'
  }
  if (form.provider === 'anyrouter') {
    return '粘贴 sk- 开头的 AnyRouter API Key'
  }
  if (form.provider === 'zo') {
    return '粘贴 zo_sk_ 开头的 Zo Access Token'
  }
  if (form.provider === 'prem') {
    return '粘贴 Prem API Key'
  }
  if (form.provider === 'custom') {
    return '粘贴自定义网关 API Key'
  }
  return '粘贴 API Key'
}

function providerTokens(provider) {
  return tokens.value.filter((item) => item.provider === provider)
}

function selectProvider(provider) {
  activeProvider.value = provider
}

function credentialLabel(item) {
  const label = credentialTypes[item.credentialType || 'api_key'] || item.credentialType || 'API Key'
  if (item.provider === 'xiaomi' && item.credentialType === 'mimo_token_plan') {
    const regionLabels = {
      cn: '中国区',
      sgp: '新加坡 SGP',
      ams: '欧洲 AMS',
    }
    return `${label} · ${regionLabels[item.region] || '中国区'}`
  }
  return label
}

function normalizedCredentialType(provider, credentialType) {
  if (provider === 'openai') {
    return credentialType === 'codex_auth_json' ? 'codex_auth_json' : 'api_key'
  }
  if (provider === 'anthropic') {
    return credentialType === 'claude_oauth_json' ? 'claude_oauth_json' : 'api_key'
  }
  if (provider === 'xiaomi') {
    return credentialType === 'mimo_token_plan' ? 'mimo_token_plan' : 'api_key'
  }
  if (provider === 'zhipu') {
    return credentialType === 'coding_plan' ? 'coding_plan' : 'api_key'
  }
  return 'api_key'
}

function onProviderChange() {
  form.credentialType = normalizedCredentialType(form.provider, form.credentialType)
  if (providerRequiresBaseUrl(form.provider) && !form.baseUrl) {
    form.baseUrl = providerDefaultBaseUrl(form.provider)
  } else if (!providerRequiresBaseUrl(form.provider)) {
    form.baseUrl = ''
  }
  if (form.editingId && form.provider !== form.originalProvider) {
    form.tokenValue = ''
  }
}

function providerLabel(providerKey) {
  return providers.find((item) => item.key === providerKey)?.label || providerKey
}

function clientToolLabel(key, fallback = '') {
  return knownClientTools.find((item) => item.key === key)?.label || fallback || key || '未知工具'
}

function buildToolUsageRows(activeItems, historyItems) {
  const byClient = new Map()
  for (const item of activeItems || []) {
    const key = item.clientKey || 'api'
    const current = byClient.get(key) || {
      clientKey: key,
      clientName: item.clientName || clientToolLabel(key),
      active: true,
      activeCount: 0,
      tokenNames: new Set(),
      providerNames: new Set(),
      models: new Set(),
      startedAt: item.startedAt,
      lastSeenAt: item.startedAt,
    }
    current.active = true
    current.activeCount += 1
    if (item.tokenName) current.tokenNames.add(item.tokenName)
    if (item.provider) current.providerNames.add(providerLabel(item.provider))
    if (item.model) current.models.add(item.model)
    if (!current.startedAt || new Date(item.startedAt).getTime() < new Date(current.startedAt).getTime()) {
      current.startedAt = item.startedAt
    }
    byClient.set(key, current)
  }

  const sortedHistory = [...(historyItems || [])].sort((a, b) => {
    return new Date(b.time || 0).getTime() - new Date(a.time || 0).getTime()
  })
  for (const entry of sortedHistory) {
    if (entry.method === 'CHECK') continue
    const key = entry.clientKey || ''
    if (!key) continue
    if (byClient.has(key)) continue
    byClient.set(key, {
      clientKey: key,
      clientName: entry.clientName || clientToolLabel(key),
      active: false,
      activeCount: 0,
      tokenNames: new Set(entry.tokenName ? [entry.tokenName] : []),
      providerNames: new Set(entry.provider ? [providerLabel(entry.provider)] : []),
      models: new Set(entry.model ? [entry.model] : []),
      startedAt: '',
      lastSeenAt: entry.time,
      status: entry.status,
    })
  }

  const order = new Map(knownClientTools.map((item, index) => [item.key, index]))
  return [...byClient.values()]
    .map((item) => ({
      ...item,
      tokenText: [...item.tokenNames].join('、') || '-',
      providerText: [...item.providerNames].join('、') || '-',
      modelText: [...item.models].join('、') || '-',
    }))
    .sort((a, b) => {
      if (a.active !== b.active) return a.active ? -1 : 1
      const rankA = order.has(a.clientKey) ? order.get(a.clientKey) : 999
      const rankB = order.has(b.clientKey) ? order.get(b.clientKey) : 999
      if (rankA !== rankB) return rankA - rankB
      return new Date(b.lastSeenAt || 0).getTime() - new Date(a.lastSeenAt || 0).getTime()
    })
    .slice(0, 8)
}

function toolUsageMeta(row) {
  const parts = []
  if (row.providerText && row.providerText !== '-') parts.push(row.providerText)
  if (row.modelText && row.modelText !== '-') parts.push(row.modelText)
  if (!row.active && row.lastSeenAt) parts.push(`最近 ${formatTime(row.lastSeenAt)}`)
  return parts.join(' · ') || '-'
}

function toolUsageDuration(row) {
  if (!row.active || !row.startedAt) return ''
  return `已运行 ${formatDuration(Math.max(0, Date.now() - new Date(row.startedAt).getTime()))}`
}

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

function changeQuotaOverviewPage(type, direction) {
  const target = type === 'subscription' ? subscriptionQuotaPage : apiQuotaPage
  const count = type === 'subscription' ? subscriptionOverviewPageCount.value : apiOverviewPageCount.value
  target.value = clampQuotaOverviewPage(target.value + direction, count)
}

function closeHistoryDiagnosis() {
  selectedHistoryEntry.value = null
}

function resetQuotaRefreshProgress() {
  Object.assign(quotaRefreshProgress, {
    visible: false,
    percent: 0,
    total: 0,
    completed: 0,
    failed: 0,
    providerLabel: '',
    currentName: '',
  })
}

function wait(ms) {
  return new Promise((resolve) => {
    window.setTimeout(resolve, ms)
  })
}

function startQuotaRefreshProgress(items) {
  Object.assign(quotaRefreshProgress, {
    visible: true,
    percent: 10,
    total: items.length,
    completed: 0,
    failed: 0,
    providerLabel: activeProviderInfo.value.label,
    currentName: items[0]?.name || '',
  })
}

function updateQuotaRefreshProgress({ completed, failed, currentName, done = false }) {
  quotaRefreshProgress.completed = completed
  quotaRefreshProgress.failed = failed
  quotaRefreshProgress.currentName = currentName || quotaRefreshProgress.currentName
  quotaRefreshProgress.percent = done
    ? 100
    : Math.min(96, Math.max(10, Math.round(10 + (completed / Math.max(quotaRefreshProgress.total, 1)) * 84)))
}

async function refreshProviderQuotas() {
  if (refreshingProvider.value) return

  const items = activeProviderTokens.value.filter((item) => !item.disabled)
  if (!items.length) {
    successMessage.value = `暂无启用的 ${activeProviderInfo.value.label} 账号可刷新`
    return
  }

  errorMessage.value = ''
  successMessage.value = ''
  refreshingProvider.value = true
  let failed = 0
  let completed = 0
  let finalErrorMessage = ''
  let finalSuccessMessage = ''
  startQuotaRefreshProgress(items)

  try {
    for (const item of items) {
      quotaRefreshProgress.currentName = item.name
      validatingIds[item.id] = true
      try {
        const result = await validateToken(item.id)
        if (!result?.ok) {
          failed += 1
        }
      } catch {
        failed += 1
      } finally {
        validatingIds[item.id] = false
        completed += 1
        updateQuotaRefreshProgress({ completed, failed, currentName: item.name })
      }
    }
    updateQuotaRefreshProgress({ completed, failed, currentName: '同步最新额度状态' })
    await refreshRealtime()
    updateQuotaRefreshProgress({
      completed,
      failed,
      currentName: failed ? '部分账号刷新失败' : '刷新完成',
      done: true,
    })
    await wait(260)
    if (failed) {
      finalErrorMessage = `已刷新 ${items.length - failed} 个账号，${failed} 个失败`
    } else {
      finalSuccessMessage = `已刷新 ${items.length} 个 ${activeProviderInfo.value.label} 账号`
    }
  } catch (error) {
    finalErrorMessage = error.message
  } finally {
    resetQuotaRefreshProgress()
    refreshingProvider.value = false
    if (finalErrorMessage) {
      errorMessage.value = finalErrorMessage
    } else if (finalSuccessMessage) {
      successMessage.value = finalSuccessMessage
    }
  }
}

async function refreshQuota(item) {
  await verifyToken(item)
}
</script>

<template>
  <div class="shell" :class="{ dark: isDark }">
    <header
      class="window-titlebar"
      :class="{ maximised: windowMaximised }"
      aria-label="窗口控制栏"
      @dblclick="toggleWindowMaximise"
    >
      <div class="window-titlebar-drag">
        <img :src="appIconUrl" alt="" />
        <div>
          <strong>OmniProxy</strong>
          <span>{{ appInfo.isDevelopment ? 'Dev' : appInfo.version }} · {{ proxyStatus.running ? '代理运行中' : '代理未启动' }}</span>
        </div>
      </div>
      <div
        v-if="titlebarUpdateVisible"
        class="window-titlebar-actions"
        aria-label="应用状态"
        @pointerdown.stop
      >
        <button
          type="button"
          class="titlebar-update-button"
          :title="titlebarUpdatePrompt.tooltip"
          aria-haspopup="dialog"
          :aria-expanded="titlebarUpdatePopoverOpen"
          @click.stop="toggleTitlebarUpdatePopover"
          @dblclick.stop
        >
          <span class="titlebar-update-mark" aria-hidden="true"></span>
          <span>新版本</span>
        </button>
        <div
          v-if="titlebarUpdatePopoverOpen"
          class="titlebar-update-popover"
          role="dialog"
          aria-label="新版本提示"
          @click.stop
          @dblclick.stop
        >
          <div class="titlebar-update-popover-head">
            <span class="titlebar-update-popover-icon" aria-hidden="true"></span>
            <div>
              <span class="titlebar-update-popover-kicker">{{ titlebarUpdatePrompt.badge }}</span>
              <strong>{{ titlebarUpdatePrompt.title }}</strong>
            </div>
            <button
              type="button"
              class="titlebar-update-popover-close"
              aria-label="关闭更新提示"
              @click="closeTitlebarUpdatePopover"
            >
              <span aria-hidden="true"></span>
            </button>
          </div>
          <p>{{ titlebarUpdatePrompt.description }}</p>
          <div class="titlebar-update-popover-meta">
            <div>
              <span>当前版本</span>
              <strong>{{ titlebarUpdatePrompt.currentVersion }}</strong>
            </div>
            <div>
              <span>最新版本</span>
              <strong>{{ titlebarUpdatePrompt.latestVersion }}</strong>
            </div>
          </div>
          <div class="titlebar-update-popover-actions">
            <button type="button" class="ghost-button compact-button" @click="closeTitlebarUpdatePopover">稍后</button>
            <button type="button" class="primary-button compact-button" @click="confirmTitlebarUpdatePopover">
              {{ titlebarUpdatePrompt.primaryText }}
            </button>
          </div>
        </div>
      </div>
      <div class="window-controls" aria-label="窗口操作">
        <button type="button" class="window-control minimise" aria-label="最小化" @click.stop="minimiseWindow">
          <span class="control-mark" aria-hidden="true"></span>
        </button>
        <button
          type="button"
          :class="['window-control', windowMaximised ? 'restore' : 'maximise']"
          :aria-label="windowMaximised ? '还原窗口' : '最大化'"
          @click.stop="toggleWindowMaximise"
        >
          <span class="control-mark" aria-hidden="true"></span>
        </button>
        <button type="button" class="window-control close" aria-label="关闭窗口" @click.stop="closeWindow">
          <span class="control-mark" aria-hidden="true"></span>
        </button>
      </div>
    </header>
    <div
      v-if="hasWailsRuntime()"
      class="window-resize-edge window-resize-edge-right"
      aria-hidden="true"
      @mousedown.prevent.stop="startWindowResize('e-resize')"
    ></div>

    <div
      v-if="mobileSidebarOpen"
      class="mobile-sidebar-backdrop"
      aria-hidden="true"
      @click="mobileSidebarOpen = false"
    ></div>

    <aside class="sidebar" :class="{ open: mobileSidebarOpen }">
      <div class="brand">
        <div class="brand-mark">
          <img :src="appIconUrl" alt="" />
        </div>
        <div>
          <strong>OmniProxy</strong>
          <span>本地 API 网关</span>
        </div>
      </div>

      <div class="sidebar-status">
        <div class="sidebar-status-main">
          <div :class="['status-light', { online: proxyStatus.running }]"></div>
          <div>
            <strong>{{ proxyStatus.running ? '代理运行中' : '代理未启动' }}</strong>
            <span>{{ proxyEndpoint }} · {{ tokens.length }} 个账号</span>
          </div>
        </div>
        <div class="sidebar-status-meta">
          <div>
            <span>端口</span>
            <strong>{{ proxyStatus.port || config.proxyPort }}</strong>
          </div>
          <div>
            <span>可用账号</span>
            <strong>{{ activeTokens.length }}</strong>
          </div>
          <div>
            <span>状态</span>
            <strong>{{ proxyStatus.running ? '运行中' : '已停止' }}</strong>
          </div>
        </div>
        <button type="button" class="sidebar-proxy-button" @click="toggleProxy">
          <component :is="SwitchButton" class="button-icon" aria-hidden="true" />
          <span>{{ proxyStatus.running ? '停止代理' : '启动代理' }}</span>
        </button>
      </div>

      <nav class="nav-list">
        <section v-for="section in navSections" :key="section.label" class="nav-section">
          <span class="nav-section-label">{{ section.label }}</span>
          <button
            v-for="tab in section.items"
            :key="tab.key"
            type="button"
            :class="{ active: activeTab === tab.key }"
            @click="selectTab(tab.key)"
          >
            <component :is="tabIcons[tab.key]" class="nav-icon" aria-hidden="true" />
            <span>{{ tab.label }}</span>
          </button>
        </section>
      </nav>

      <div class="sidebar-tools">
        <button type="button" class="ghost-button" @click="toggleAppTheme">
          <component :is="isDark ? Sunny : Moon" class="button-icon" aria-hidden="true" />
          <span>{{ appThemeLabel }}</span>
        </button>
      </div>
    </aside>

    <main
      ref="workspaceRef"
      :class="[
        'workspace',
        {
          'openrouter-workspace': activeTab === 'openrouter-chat',
          'logs-workspace': activeTab === 'logs',
          'workspace-scrollbar-visible': workspaceScrollbarVisible,
        },
      ]"
      @pointermove="handleWorkspacePointerMove"
      @pointerleave="hideWorkspaceScrollbar"
      @scroll="handleWorkspaceScroll"
    >
      <header class="topbar">
        <button
          type="button"
          class="mobile-menu-button"
          :aria-expanded="mobileSidebarOpen"
          aria-label="打开导航"
          @click="mobileSidebarOpen = true"
        >
          <span></span>
          <span></span>
          <span></span>
        </button>
        <div class="topbar-title">
          <p class="eyebrow">本地控制台</p>
          <h1>{{ currentTabLabel }}</h1>
          <p class="topbar-subtitle">OmniProxy 桌面网关</p>
        </div>
      </header>

      <TransitionGroup name="quota-refresh" tag="div" class="toast-stack quota-refresh-stack" aria-live="polite">
        <div v-if="quotaRefreshProgress.visible" key="quota-refresh" class="notice quota-refresh-toast" role="status">
          <div class="quota-refresh-orb" aria-hidden="true">
            <span></span>
          </div>
          <div class="quota-refresh-body">
            <div class="quota-refresh-title-row">
              <strong>刷新中{{ quotaRefreshProgress.percent }}%</strong>
              <span>{{ quotaRefreshProgress.completed }} / {{ quotaRefreshProgress.total }}</span>
            </div>
            <div
              class="quota-refresh-track"
              role="progressbar"
              aria-label="额度刷新进度"
              aria-valuemin="0"
              aria-valuemax="100"
              :aria-valuenow="quotaRefreshProgress.percent"
            >
              <span :style="{ width: `${quotaRefreshProgress.percent}%` }"></span>
            </div>
            <small>{{ quotaRefreshProgress.currentName || `${quotaRefreshProgress.providerLabel} 额度刷新中` }}</small>
          </div>
        </div>
      </TransitionGroup>

      <TransitionGroup name="snackbar" tag="div" class="toast-stack" aria-live="polite">
        <div v-if="errorMessage" key="error" class="alert" role="alert">
          <span class="toast-message">{{ errorMessage }}</span>
          <button type="button" aria-label="关闭错误提示" @click="errorMessage = ''">×</button>
        </div>
        <div v-if="successMessage" key="success" class="notice" role="status">
          <span class="toast-message">{{ successMessage }}</span>
          <button type="button" aria-label="关闭成功提示" @click="successMessage = ''">×</button>
        </div>
      </TransitionGroup>

      <Transition
        name="page-switch"
        mode="out-in"
        appear
        @before-enter="restoreActiveWorkspaceScroll"
        @after-enter="afterPageEnter"
      >
      <DashboardView
        v-if="activeTab === 'dashboard'"
        key="dashboard"
        :proxy-status="proxyStatus"
        :proxy-endpoint="proxyEndpoint"
        :dashboard-signals="dashboardSignals"
        :active-tokens="activeTokens"
        :invalid-tokens="invalidTokens"
        :low-tokens="lowTokens"
        :cooling-tokens="coolingTokens"
        :exhausted-tokens="exhaustedTokens"
        :disabled-tokens="disabledTokens"
        :total-proxy-tokens="totalProxyTokens"
        :total-proxy-input-tokens="totalProxyInputTokens"
        :total-proxy-output-tokens="totalProxyOutputTokens"
        :today-proxy-tokens="todayProxyTokens"
        :today-proxy-requests="todayProxyRequests"
        :total-proxy-requests="totalProxyRequests"
        :active-requests="activeRequests"
        :active-token-ids="activeTokenIds"
        :tool-usage-rows="toolUsageRows"
        :subscription-overview-tokens="subscriptionOverviewTokens"
        :subscription-quota-page="subscriptionQuotaPage"
        :subscription-overview-page-count="subscriptionOverviewPageCount"
        :subscription-quota-page-text="subscriptionQuotaPageText"
        :paged-subscription-overview-tokens="pagedSubscriptionOverviewTokens"
        :api-overview-tokens="apiOverviewTokens"
        :api-quota-page="apiQuotaPage"
        :api-overview-page-count="apiOverviewPageCount"
        :api-quota-page-text="apiQuotaPageText"
        :paged-api-overview-tokens="pagedApiOverviewTokens"
        :logs="logs"
        :daily-usage-rows="dailyUsageRows"
        :dashboard-trend-rows="dashboardTrendRows"
        :dashboard-daily-usage-rows="dashboardDailyUsageRows"
        :format-number="formatNumber"
        :format-time="formatTime"
        :client-tool-label="clientToolLabel"
        :tool-usage-meta="toolUsageMeta"
        :tool-usage-duration="toolUsageDuration"
        :quota-overview-range-text="quotaOverviewRangeText"
        :is-token-active-now="isTokenActiveNow"
        :weekly-limit-reached="weeklyLimitReached"
        :display-status-class="displayStatusClass"
        :display-status-label="displayStatusLabel"
        :provider-label="providerLabel"
        :quota-primary-label="quotaPrimaryLabel"
        :quota-percent-value="quotaPercentValue"
        :quota-percent-text="quotaPercentText"
        :credential-label="credentialLabel"
        :api-quota-display="apiQuotaDisplay"
        :api-quota-meta="apiQuotaMeta"
        :trend-width="trendWidth"
        :request-trend-width="requestTrendWidth"
        @toggle-proxy="toggleProxy"
        @refresh="refreshAll"
        @open-settings="selectTab('settings')"
        @open-billing="openBillingView"
        @open-trends="selectTab('usage-trends')"
        @change-quota-page="changeQuotaOverviewPage"
      />
      <TokenTrendView
        v-else-if="activeTab === 'usage-trends'"
        key="usage-trends"
        :daily-usage-rows="dailyUsageRows"
        :format-number="formatNumber"
        @refresh="refreshAll"
      />
      <BillingView
        v-else-if="activeTab === 'billing'"
        key="billing"
        :entries="requestHistory"
        :daily-usage="billingUsage"
        :available-dates="billingDates"
        :selected-date="selectedBillingDate"
        :format-number="formatNumber"
        @date-change="changeBillingDate"
        @refresh="refreshBilling"
      />

      <QuotasView
        v-else-if="activeTab === 'quotas'"
        key="quotas"
        :providers="providers"
        :active-provider="activeProvider"
        :active-provider-info="activeProviderInfo"
        :active-provider-tokens="activeProviderTokens"
        :active-provider-enabled-count="activeProviderEnabledCount"
        :api-balance-summaries="activeProviderAPIBalanceSummaries"
        :refreshing-provider="refreshingProvider"
        :switching-only-token-ids="switchingOnlyTokenIds"
        :validating-ids="validatingIds"
        :provider-tokens="providerTokens"
        :credential-label="credentialLabel"
        :provider-label="providerLabel"
        @refresh="refreshAll"
        @refresh-provider-quotas="refreshProviderQuotas"
        @select-provider="selectProvider"
        @toggle-token-selected="toggleTokenSelected"
        @refresh-quota="refreshQuota"
      />

      <TokensView
        v-else-if="activeTab === 'tokens'"
        key="tokens"
        :providers="providers"
        :active-provider="activeProvider"
        :active-provider-info="activeProviderInfo"
        :active-provider-tokens="activeProviderTokens"
        :api-balance-summaries="activeProviderAPIBalanceSummaries"
        :exporting-tokens="exportingTokens"
        :exporting-codex-auth="exportingCodexAuth"
        :codex-auth-importing="codexAuthImporting"
        :batch-importing="batchImporting"
        :open-router-models="openRouterModels"
        :open-router-models-loading="openRouterModelsLoading"
        :open-router-models-error="openRouterModelsError"
        :open-router-models-fetched-at="openRouterModelsFetchedAt"
        :open-router-models-cached="openRouterModelsCached"
        :validating-ids="validatingIds"
        :refreshing-token-ids="refreshingTokenIds"
        :toggling-token-ids="togglingTokenIds"
        :provider-tokens="providerTokens"
        :credential-label="credentialLabel"
        :credential-display="credentialDisplay"
        :display-status-type="displayStatusType"
        :display-status-label="displayStatusLabel"
        :health-summary="healthSummary"
        :format-time="formatTime"
        :format-number="formatNumber"
        :format-balance="formatBalance"
        :quota-display="quotaDisplay"
        @select-provider="selectProvider"
        @export-token-backup="exportTokenBackup"
        @open-codex-auth-file-picker="openCodexAuthFilePicker"
        @import-codex-auth-files="importCodexAuthFiles"
        @export-codex-auth-backups="exportCodexAuthBackups"
        @refresh-open-router-models="refreshOpenRouterModels({ force: true })"
        @open-router-model-chat="openOpenRouterChat"
        @open-create-form="openCreateForm"
        @open-batch-import="openBatchImport"
        @verify-token="verifyToken"
        @refresh-token-auth="refreshAuthToken"
        @toggle-token-enabled="toggleTokenEnabled"
        @open-edit-form="openEditForm"
        @remove-token="removeToken"
      />

      <OpenRouterChatView
        v-else-if="activeTab === 'openrouter-chat'"
        key="openrouter-chat"
        :models="openRouterModels"
        :selected-model="selectedOpenRouterChatModel"
        :models-loading="openRouterModelsLoading"
        :models-error="openRouterModelsError"
        :open-router-tokens="openRouterTokens"
        :validating-ids="validatingIds"
        :format-time="formatTime"
        :format-number="formatNumber"
        @update:selected-model="selectOpenRouterChatModel"
        @refresh-models="refreshOpenRouterModels({ force: true })"
        @refresh-token="verifyToken"
        @open-create-key="openCreateForm('openrouter')"
      />

      <HistoryView
        v-else-if="activeTab === 'history'"
        key="history"
        :entries="requestHistory"
        :summary="requestHistorySummary"
        :providers="providers"
        :exporting="exportingHistory"
        :format-time="formatTime"
        :format-duration="formatDuration"
        :format-number="formatNumber"
        :provider-label="providerLabel"
        @refresh="refreshHistory"
        @export="exportRequestHistory"
        @diagnose="selectedHistoryEntry = $event"
      />

      <LogsView
        v-else-if="activeTab === 'logs'"
        key="logs"
        :logs="logs"
        :format-time="formatTime"
        :format-duration="formatDuration"
        @refresh="refreshRealtime"
      />

      <SettingsView
        v-else-if="activeTab === 'settings'"
        key="settings"
        :config="config"
        :data-directory="dataDirectory"
        :data-dir-changing="dataDirChanging"
        :auto-start-changing="autoStartChanging"
        :auto-start-enabled="autoStartEnabled"
        :task-automation-browser-profiles="taskAutomationBrowserProfiles"
        :task-automation-browser-profiles-loading="taskAutomationBrowserProfilesLoading"
        :task-automation-browser-profiles-error="taskAutomationBrowserProfilesError"
        :clearing-billing-usage="clearingBillingUsage"
        :clearing-request-history="clearingRequestHistory"
        @persist-config="persistConfig"
        @choose-data-directory="chooseDataDirectory"
        @toggle-auto-start="toggleAutoStart"
        @refresh-task-automation-browser-profiles="refreshTaskAutomationBrowserProfiles"
        @clear-billing-usage="clearBillingUsageData"
        @clear-request-history="clearRequestHistoryData"
      />

      <AboutView
        v-else-if="activeTab === 'about'"
        key="about"
        :app-info="appInfo"
        :config="config"
        :data-directory="dataDirectory"
        :proxy-status="proxyStatus"
        :auto-start-enabled="autoStartEnabled"
        :update-checking="updateChecking"
        :update-info="lastUpdateInfo"
        :update-download-status="updateDownloadStatus"
        :update-checked-at="lastUpdateCheckedAt"
        :format-time="formatTime"
        @manual-check-for-updates="manualCheckForUpdates"
        @download-update="startUpdateDownload"
        @install-update="installReadyUpdateFromUpdateSurface"
        @open-url="openExternalURL"
      />

      <QuickstartView
        v-else-if="activeTab === 'quickstart'"
        key="quickstart"
        v-model:selected-claude-models="selectedClaudeModels"
        :config="config"
        :claude-model-options="claudeModelOptions"
        :claude-model-selection-limit="claudeModelSelectionLimit"
        :selected-claude-model-labels="selectedClaudeModelLabels"
        :can-configure-claude-models="canConfigureClaudeModels"
        :is-claude-model-option-disabled="isClaudeModelOptionDisabled"
        :codex-configuring="codexConfiguring"
        :codex-sub2api-configuring="codexSub2APIConfiguring"
        :codex-newapi-configuring="codexNewAPIConfiguring"
        :codex-anyrouter-configuring="codexAnyRouterConfiguring"
        :codex-zo-configuring="codexZoConfiguring"
        :codex-prem-configuring="codexPremConfiguring"
        :codex-restoring="codexRestoring"
        :claude-models-configuring="claudeModelsConfiguring"
        :claude-desktop-configuring="claudeDesktopConfiguring"
        :claude-desktop-restoring="claudeDesktopRestoring"
        :deep-seek-claude-configuring="deepSeekClaudeConfiguring"
        :mimo-claude-configuring="mimoClaudeConfiguring"
        :kimi-claude-configuring="kimiClaudeConfiguring"
        :zhipu-claude-configuring="zhipuClaudeConfiguring"
        :any-router-claude-configuring="anyRouterClaudeConfiguring"
        :zo-claude-configuring="zoClaudeConfiguring"
        :prem-claude-configuring="premClaudeConfiguring"
        :mimo-claude-restoring="mimoClaudeRestoring"
        :gemini-configuring="geminiConfiguring"
        :gemini-restoring="geminiRestoring"
        :opencode-configuring="opencodeConfiguring"
        :opencode-restoring="opencodeRestoring"
        :pi-configuring="piConfiguring"
        :pi-restoring="piRestoring"
        :deep-seek-tui-configuring="deepSeekTUIConfiguring"
        :deep-seek-tui-restoring="deepSeekTUIRestoring"
        @configure-codex="configureLocalCodex"
        @configure-codex-sub2api="configureLocalCodexSub2API"
        @configure-codex-newapi="configureLocalCodexNewAPI"
        @configure-codex-anyrouter="configureLocalCodexAnyRouter"
        @configure-codex-zo="configureLocalCodexZo"
        @configure-codex-prem="configureLocalCodexPrem"
        @restore-codex="restoreLocalCodex"
        @configure-claude-models="configureLocalClaudeModels"
        @configure-claude-desktop-models="configureLocalClaudeDesktopModels"
        @restore-claude-desktop="restoreLocalClaudeDesktop"
        @configure-deepseek-claude="configureLocalDeepSeekClaude"
        @configure-mimo-claude="configureLocalMimoClaude"
        @configure-kimi-claude="configureLocalKimiClaude"
        @configure-zhipu-claude="configureLocalZhipuClaude"
        @configure-anyrouter-claude="configureLocalAnyRouterClaude"
        @configure-zo-claude="configureLocalZoClaude"
        @configure-prem-claude="configureLocalPremClaude"
        @restore-mimo-claude="restoreLocalMimoClaude"
        @configure-gemini="configureLocalGemini"
        @restore-gemini="restoreLocalGemini"
        @configure-opencode="configureLocalOpenCode"
        @restore-opencode="restoreLocalOpenCode"
        @configure-pi="configureLocalPi"
        @restore-pi="restoreLocalPi"
        @configure-deepseek-tui="configureLocalDeepSeekTUI"
        @restore-deepseek-tui="restoreLocalDeepSeekTUI"
      />

      <HelpView
        v-else-if="activeTab === 'help'"
        key="help"
        :proxy-status="proxyStatus"
        :config="config"
        :active-tokens-count="activeTokens.length"
        :token-count="tokens.length"
        :low-tokens-count="lowTokens.length"
        :invalid-tokens-count="invalidTokens.length"
        :active-requests-count="activeRequests.length"
        :today-proxy-requests="todayProxyRequests"
        :format-number="formatNumber"
        @select-tab="selectTab"
        @copy-endpoint="copyEndpointValue"
      />
      </Transition>

      <DiagnosticDrawer
        :entry="selectedHistoryEntry"
        :format-time="formatTime"
        :format-duration="formatDuration"
        :provider-label="providerLabel"
        @close="closeHistoryDiagnosis"
      />

      <Transition name="modal-pop" appear>
        <FirstUseGuideModal
          v-if="firstUseGuideVisible"
          v-model:step-index="firstUseGuideStepIndex"
          :steps="firstUseGuideSteps"
          :current-step="currentFirstUseGuideStep"
          :proxy-endpoint="proxyEndpoint"
          @close="closeFirstUseGuide"
          @previous="previousFirstUseGuideStep"
          @run-action="runFirstUseGuideAction"
          @next="nextFirstUseGuideStep"
        />
      </Transition>

      <Transition name="modal-pop" appear>
        <TokenEditorModal
          v-if="form.visible"
          :form="form"
          :providers="providers"
          :is-codex-form="isAutoNameForm"
          :placeholder="credentialPlaceholder()"
          @close="closeForm"
          @submit="submitForm"
          @provider-change="onProviderChange"
        />
      </Transition>

      <Transition name="modal-pop" appear>
        <TokenBatchImportModal
          v-if="batchImportForm.visible"
          :form="batchImportForm"
          :providers="providers"
          :placeholder="batchImportPlaceholder()"
          :importing="batchImporting"
          @close="closeBatchImport"
          @submit="submitBatchImport"
          @provider-change="onBatchImportProviderChange"
        />
      </Transition>

      <Transition name="modal-pop" appear>
        <div v-if="deleteCandidate" class="danger-confirm-backdrop" @click.self="closeDeleteConfirm">
          <section class="danger-confirm" role="dialog" aria-modal="true" aria-labelledby="delete-token-title">
            <div class="danger-confirm-mark" aria-hidden="true">
              <span></span>
            </div>
            <div class="danger-confirm-body">
              <p class="danger-confirm-kicker">危险操作</p>
              <h2 id="delete-token-title">删除这个账号？</h2>
              <p>
                删除后该账号会立即从调度池移除，已保存的凭据也会从本机账号池删除。历史请求记录不会被清空。
              </p>
              <div class="danger-confirm-card">
                <span>账号</span>
                <strong>{{ deleteCandidate.name }}</strong>
                <small>{{ providerLabel(deleteCandidate.provider) }} · {{ credentialLabel(deleteCandidate) }}</small>
              </div>
            </div>
            <div class="danger-confirm-actions">
              <button type="button" class="ghost-button" :disabled="deleteBusy" @click="closeDeleteConfirm">
                取消
              </button>
              <button type="button" class="danger-button solid" :disabled="deleteBusy" @click="confirmRemoveToken">
                {{ deleteBusy ? '删除中' : '删除账号' }}
              </button>
            </div>
          </section>
        </div>
      </Transition>

      <div v-if="loading" class="loading">加载中...</div>
    </main>
  </div>
</template>
