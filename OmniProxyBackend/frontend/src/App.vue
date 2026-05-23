<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { ElMessageBox } from 'element-plus'
import AboutView from './components/AboutView.vue'
import BillingView from './components/BillingView.vue'
import DashboardView from './components/DashboardView.vue'
import DiagnosticDrawer from './components/DiagnosticDrawer.vue'
import HistoryView from './components/HistoryView.vue'
import LogsView from './components/LogsView.vue'
import OpenRouterChatView from './components/OpenRouterChatView.vue'
import SettingsView from './components/SettingsView.vue'
import TokenBatchImportModal from './components/TokenBatchImportModal.vue'
import TokenEditorModal from './components/TokenEditorModal.vue'
import TokenTrendView from './components/TokenTrendView.vue'
import TokensView from './components/TokensView.vue'
import appIconUrl from './assets/appicon.png'
import { credentialTypes, providers, statusMeta, tabs } from './constants/app'
import { formatDuration, formatNumber, formatResetTime, formatTime, localDateKey } from './utils/format'
import { aggregateAPIBalanceSummaries } from './utils/quota'
import {
  WindowHide,
  WindowIsMaximised,
  WindowMinimise,
  WindowToggleMaximise,
} from '../wailsjs/runtime/runtime'
import {
  configureCodex,
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
  configureZhipuClaude,
  configureZoClaude,
  createToken,
  chooseDataDirectory as chooseDataDirectoryWithDialog,
  checkForUpdates,
  clearBillingUsage,
  clearRequestHistory,
  deleteToken,
  downloadUpdate,
  exportCodexAuthFiles,
  exportHistory,
  exportTokens,
  getAutoStartStatus,
  getAppInfo,
  getActiveRequests,
  getBillingDates,
  getBillingUsage,
  getConfig,
  getDataDirectory,
  getHistory,
  getHistorySummary,
  getLogs,
  getOpenRouterModels,
  getProxyStatus,
  getTokens,
  getUpdateDownloadStatus,
  installDownloadedUpdate,
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
  ArrowLeft,
  ArrowRight,
  Coin,
  CircleCheckFilled,
  Clock,
  DataBoard,
  HelpFilled,
  InfoFilled,
  Key,
  Lightning,
  MagicStick,
  Memo,
  Monitor,
  Money,
  Moon,
  Plus,
  Refresh,
  RefreshRight,
  Setting,
  Sunny,
  SwitchButton,
  TrendCharts,
} from '@element-plus/icons-vue'

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
const workspaceRef = ref(null)
const workspaceScrollbarVisible = ref(false)
const codexConfiguring = ref(false)
const codexSub2APIConfiguring = ref(false)
const codexZoConfiguring = ref(false)
const codexRestoring = ref(false)
const mimoClaudeConfiguring = ref(false)
const deepSeekClaudeConfiguring = ref(false)
const kimiClaudeConfiguring = ref(false)
const zhipuClaudeConfiguring = ref(false)
const zoClaudeConfiguring = ref(false)
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
const updateChecking = ref(false)
const lastUpdateInfo = ref(null)
const lastUpdateCheckedAt = ref('')
const titlebarUpdatePopoverOpen = ref(false)
const updateDownloadStatus = ref({ state: 'idle', percent: 0, bytesReceived: 0 })
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
const skippedUpdateVersionKey = 'omniproxy.skippedUpdateVersion'
const appThemeStorageKey = 'omniproxy.appTheme'
const firstUseGuideStorageKey = 'omniproxy.firstRunGuideModalDismissed'
let toastTimer = null
let workspaceScrollbarTimer = null
let workspaceScrollSavePaused = false
let realtimeTimer = null
let updateCheckTimer = null
let updateDownloadTimer = null
let historyRefreshSeq = 0
const validatingIds = reactive({})
const refreshingTokenIds = reactive({})
const togglingTokenIds = reactive({})
const switchingOnlyTokenIds = reactive({})
const workspaceScrollPositions = reactive({})
const tokens = ref([])
const logs = ref([])
const requestHistory = ref([])
const requestHistorySummary = ref(null)
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
  outboundProxyProviders: ['openai', 'anthropic', 'gemini', 'openrouter', 'zo'],
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
  zoBaseUrl: 'https://api.zo.computer',
  customGatewayBaseUrl: '',
  customGatewayAnthropicBaseUrl: '',
  xiaomiBaseUrl: '',
  xiaomiApiBaseUrl: 'https://api.xiaomimimo.com/v1',
  xiaomiApiAnthropicBaseUrl: 'https://api.xiaomimimo.com/anthropic',
  xiaomiTokenPlanBaseUrl: 'https://token-plan-cn.xiaomimimo.com/v1',
  xiaomiTokenPlanAnthropicBaseUrl: 'https://token-plan-cn.xiaomimimo.com/anthropic',
  xiaomiTokenPlanSgpBaseUrl: 'https://token-plan-sgp.xiaomimimo.com/v1',
  xiaomiTokenPlanSgpAnthropicBaseUrl: 'https://token-plan-sgp.xiaomimimo.com/anthropic',
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
const claudeModelSelectionLimit = 4
const claudeModelOptions = [
  {
    id: 'deepseek-v4-pro[1m]',
    label: 'DeepSeek V4 Pro',
    description: 'deepseek-v4-pro[1m]',
  },
  {
    id: 'deepseek-v4-flash',
    label: 'DeepSeek V4 Flash',
    description: 'deepseek-v4-flash',
  },
  {
    id: 'mimo-v2.5-pro[1m]',
    label: 'MiMo V2.5 Pro 1M',
    description: 'mimo-v2.5-pro[1m]',
  },
  {
    id: 'mimo-v2.5-pro',
    label: 'MiMo V2.5 Pro',
    description: 'mimo-v2.5-pro',
  },
  {
    id: 'mimo-v2.5',
    label: 'MiMo V2.5',
    description: 'mimo-v2.5',
  },
  {
    id: 'kimi-for-coding',
    label: 'Kimi for Coding',
    description: 'kimi-for-coding',
  },
  {
    id: 'glm-5.1',
    label: 'GLM-5.1',
    description: 'glm-5.1',
  },
  {
    id: 'claude-opus-4-7',
    label: 'Zo Claude Opus 4.7',
    description: 'claude-opus-4-7',
  },
  {
    id: 'claude-sonnet-4-6',
    label: 'Zo Claude Sonnet 4.6',
    description: 'claude-sonnet-4-6',
  },
]
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
const titlebarUpdateVisible = computed(() => Boolean(lastUpdateInfo.value?.updateAvailable && !appInfo.isDevelopment))
const titlebarUpdatePrompt = computed(() => {
  const update = lastUpdateInfo.value || {}
  const updateAvailable = Boolean(update?.updateAvailable)
  const currentVersion = update?.currentVersion || appInfo.version || '当前版本'
  const latestVersion = update?.latestVersion || '新版本'
  const canDownload = Boolean(updateAvailable && update?.downloadUrl && update?.checksumUrl)
  return {
    update: updateAvailable ? update : null,
    canDownload,
    currentVersion,
    latestVersion,
    badge: '更新可用',
    title: `发现新版本 ${latestVersion}`,
    description: canDownload
      ? '可在应用内下载并校验更新安装包。'
      : '暂未获取到可用安装包，可以打开关于应用查看发布页。',
    primaryText: canDownload ? '下载更新' : '查看详情',
    tooltip: `发现新版本 ${latestVersion}`,
  }
})
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
const helpReadinessCards = computed(() => [
  {
    label: '本地代理',
    value: proxyStatus.running ? '运行中' : '未启动',
    detail: `入口 ${proxyEndpoint.value}`,
    state: proxyStatus.running ? 'ok' : 'warning',
    icon: SwitchButton,
  },
  {
    label: '可用账号',
    value: `${activeTokens.value.length} / ${tokens.value.length}`,
    detail: tokens.value.length ? `${lowTokens.value.length} 个低额度，${invalidTokens.value.length} 个无效` : '先添加至少一个上游账号',
    state: activeTokens.value.length ? 'ok' : 'warning',
    icon: Key,
  },
  {
    label: '调度策略',
    value: schedulingModeText(config.schedulingMode),
    detail: `低于 ${config.switchThreshold}% 跳过，最多重试 ${config.maxRetries} 次`,
    state: 'muted',
    icon: TrendCharts,
  },
  {
    label: '请求追踪',
    value: `${formatNumber(todayProxyRequests.value)} 次`,
    detail: `保留 ${config.historyRetentionDays} 天历史，当前 ${activeRequests.value.length} 个实时请求`,
    state: activeRequests.value.length ? 'ok' : 'muted',
    icon: Memo,
  },
])
const thirdPartyEndpointGroups = computed(() => {
  const base = `http://127.0.0.1:${proxyStatus.port || config.proxyPort}`
  return [
    {
      title: 'OpenAI 兼容客户端',
      note: 'Cherry Studio、Open WebUI、Continue、Chatbox 等支持 OpenAI-compatible 的客户端优先看这里。',
      endpoints: [
        {
          name: 'OmniProxy OpenAI',
          protocol: 'OpenAI Chat / Responses',
          baseUrl: `${base}/v1`,
          apiKey: 'omniproxy-local',
          models: '使用 OpenAI 账号池里的模型名，例如 gpt-5.4',
          use: '通用 OpenAI 兼容入口，适合自定义客户端直接接入。',
        },
        {
          name: 'Codex Chat',
          protocol: 'OpenAI Chat',
          baseUrl: `${base}/codex/v1`,
          apiKey: 'omniproxy-local',
          models: 'gpt-5.5 / gpt-5.4',
          use: '用 Codex auth.json 账号承接 Chat Completions 请求。',
        },
        {
          name: 'Zo Computer',
          protocol: 'OpenAI Chat / Responses',
          baseUrl: `${base}/zo/v1`,
          apiKey: 'omniproxy-local',
          models: 'gpt-5.5 / gpt-5.4 / claude-opus-4-7 / gemini-3.1-pro / glm-5',
          use: '适合把 Zo Token 暴露给 Cherry Studio 这类 OpenAI-compatible 客户端。',
        },
        {
          name: 'OpenRouter',
          protocol: 'OpenAI Chat',
          baseUrl: `${base}/openrouter/v1`,
          apiKey: 'omniproxy-local',
          models: 'openrouter/auto 或 OpenRouter 模型 ID',
          use: '使用 OpenRouter API Key 账号池。',
        },
        {
          name: 'TokenRouter',
          protocol: 'OpenAI Chat',
          baseUrl: `${base}/tokenrouter/v1`,
          apiKey: 'omniproxy-local',
          models: 'auto:balance / auto:quality / auto:speed / auto:cost',
          use: '使用 TokenRouter API Key 账号池和自动路由模型。',
        },
        {
          name: 'sub2api',
          protocol: 'OpenAI Chat / Responses',
          baseUrl: `${base}/sub2api/v1`,
          apiKey: 'omniproxy-local',
          models: '由 sub2api 上游决定',
          use: '转发到 sub2api OpenAI 兼容网关。',
        },
        {
          name: '自定义网关',
          protocol: 'OpenAI Chat',
          baseUrl: `${base}/custom/v1`,
          apiKey: 'omniproxy-local',
          models: 'custom-model 或上游支持的模型名',
          use: '转发到设置页填写的自定义 OpenAI 兼容网关。',
        },
      ],
    },
    {
      title: 'Anthropic / Claude 兼容客户端',
      note: '适合支持 Anthropic Messages API 的客户端。Claude Desktop 建议用一键配置写入 3P Gateway Profile。',
      endpoints: [
        {
          name: 'Claude Router',
          protocol: 'Anthropic Messages',
          baseUrl: `${base}/anthropic-router`,
          apiKey: 'omniproxy',
          models: 'deepseek-v4-pro[1m] / kimi-for-coding / glm-5.1 / claude-opus-4-7',
          use: '按模型分流到 DeepSeek、MiMo、Kimi、GLM、Zo 或 Anthropic。',
        },
        {
          name: 'Anthropic 官方账号池',
          protocol: 'Anthropic Messages',
          baseUrl: `${base}/anthropic/v1`,
          apiKey: 'omniproxy-local',
          models: 'Claude 模型名',
          use: '使用 Anthropic API Key 或 Claude OAuth JSON 账号池。',
        },
        {
          name: 'Zo Anthropic',
          protocol: 'Anthropic Messages',
          baseUrl: `${base}/zo/v1`,
          apiKey: 'omniproxy-local',
          models: 'claude-opus-4-7 / claude-sonnet-4-6',
          use: '用 Zo Token 适配 Anthropic Messages 请求。',
        },
        {
          name: 'sub2api Anthropic',
          protocol: 'Anthropic Messages',
          baseUrl: `${base}/sub2api/anthropic/v1`,
          apiKey: 'omniproxy-local',
          models: '由 sub2api 上游决定',
          use: '转发到 sub2api Anthropic 兼容入口。',
        },
        {
          name: '自定义 Anthropic 网关',
          protocol: 'Anthropic Messages',
          baseUrl: `${base}/custom/anthropic/v1`,
          apiKey: 'omniproxy-local',
          models: 'custom-model 或上游支持的模型名',
          use: '转发到设置页填写的自定义 Anthropic 兼容网关。',
        },
      ],
    },
    {
      title: '厂商直连与原生协议',
      note: '当客户端明确支持某个厂商协议，或你想固定走某个账号池时使用。',
      endpoints: [
        {
          name: 'DeepSeek',
          protocol: 'OpenAI Chat',
          baseUrl: `${base}/deepseek/v1`,
          apiKey: 'omniproxy-local',
          models: 'deepseek-v4-pro / deepseek-v4-flash',
          use: '固定使用 DeepSeek API Key 账号池。',
        },
        {
          name: 'Kimi',
          protocol: 'OpenAI Chat',
          baseUrl: `${base}/kimi/v1`,
          apiKey: 'omniproxy-local',
          models: 'kimi-for-coding',
          use: '固定使用 Kimi API Key 账号池。',
        },
        {
          name: 'Xiaomi MiMo',
          protocol: 'OpenAI Chat',
          baseUrl: `${base}/xiaomi/v1`,
          apiKey: 'omniproxy-local',
          models: 'mimo-v2.5-pro / mimo-v2.5',
          use: '固定使用 MiMo API Key 或 Token Plan 账号池。',
        },
        {
          name: 'Zhipu GLM',
          protocol: 'OpenAI Chat',
          baseUrl: `${base}/zhipu/v1`,
          apiKey: 'omniproxy-local',
          models: 'glm-5.1',
          use: '固定使用 Zhipu GLM 账号池。',
        },
        {
          name: 'MiniMax',
          protocol: 'OpenAI Chat',
          baseUrl: `${base}/minimax/v1`,
          apiKey: 'omniproxy-local',
          models: 'MiniMax-M2.7',
          use: '固定使用 MiniMax API Key 账号池。',
        },
        {
          name: 'Gemini Native',
          protocol: 'Gemini API',
          baseUrl: `${base}/gemini`,
          apiKey: 'omniproxy-local',
          models: 'gemini-3-pro-preview / gemini-3-flash-preview',
          use: '用于支持 Gemini 原生 API 的客户端。',
        },
        {
          name: 'sub2api Gemini',
          protocol: 'Gemini API',
          baseUrl: `${base}/sub2api/gemini`,
          apiKey: 'omniproxy-local',
          models: '由 sub2api 上游决定',
          use: '转发到 sub2api Gemini 兼容入口。',
        },
      ],
    },
    {
      title: '编程工具路由',
      note: '这些入口也可手动配置，但通常优先使用“一键配置”。',
      endpoints: [
        {
          name: 'Codex backend',
          protocol: 'Codex backend',
          baseUrl: `${base}/backend-api/codex`,
          apiKey: 'omniproxy-local',
          models: 'gpt-5.5 / gpt-5.4',
          use: 'Codex CLI backend API，使用 OpenAI Codex auth.json 账号。',
        },
        {
          name: 'OpenCode Router',
          protocol: 'OpenAI Chat',
          baseUrl: `${base}/opencode-router/v1`,
          apiKey: 'omniproxy-local',
          models: 'gpt-5.4 / deepseek-v4-pro / glm-5.1 / mimo-v2.5-pro',
          use: 'OpenCode 按模型自动分流。',
        },
        {
          name: 'Pi Router',
          protocol: 'OpenAI Chat',
          baseUrl: `${base}/pi-router/v1`,
          apiKey: 'omniproxy-local',
          models: 'gpt-5.4 / auto:balance / openrouter/auto / custom-model',
          use: 'Pi Coding Agent 按 provider 和模型自动分流。',
        },
        {
          name: 'Claude Desktop Gateway',
          protocol: 'Claude Desktop 3P Gateway',
          baseUrl: `${base}/claude-desktop`,
          apiKey: 'omniproxy-claude-desktop',
          models: '由 Claude Desktop Profile 映射决定',
          use: '仅用于 Claude Desktop 3P Gateway；建议通过一键配置写入。',
        },
      ],
    },
  ]
})
const helpCredentialGroups = [
  {
    title: '订阅与 OAuth 账号',
    summary: 'Codex auth.json、Claude OAuth JSON、MiMo Token Plan、GLM Coding Plan',
    detail: '适合需要订阅额度窗口、自动刷新额度或客户端专用鉴权的场景。',
  },
  {
    title: '按量 API Key',
    summary: 'OpenAI、Anthropic、DeepSeek、Kimi、MiMo、Gemini、OpenRouter、TokenRouter、Zo Computer',
    detail: '适合 OpenAI / Anthropic 兼容接口转发，额度页会展示余额、剩余额度或最近统计。',
  },
  {
    title: '网关类账号',
    summary: 'sub2api、自定义网关',
    detail: '适合把已有兼容网关纳入 OmniProxy 调度，并统一暴露本机 loopback 入口。',
  },
]
const helpWorkflowSteps = [
  {
    step: '01',
    title: '准备账号池',
    description: '在账号管理中按厂商添加凭据。Codex auth.json 会自动解析账号名；API Key 可以批量导入，适合密集账号池。',
    actions: [
      { label: '账号管理', tab: 'tokens' },
      { label: '一键配置', tab: 'quickstart' },
    ],
  },
  {
    step: '02',
    title: '确认路由和策略',
    description: '在全局设置确认本地端口、上游 Base URL、调度模式、低额度跳过阈值和自动重试次数。',
    actions: [{ label: '全局设置', tab: 'settings' }],
  },
  {
    step: '03',
    title: '启动本地代理',
    description: '客户端只连 127.0.0.1，真实上游 Token 留在本机。代理会根据账号状态、选择范围和并发占用自动挑选账号。',
    actions: [{ label: '回到仪表盘', tab: 'dashboard' }],
  },
  {
    step: '04',
    title: '观察额度与请求',
    description: '额度页看账号是否低额度、耗尽或无效；请求历史和实时日志用于定位失败原因、模型、Token 消耗和重试路径。',
    actions: [
      { label: '额度', tab: 'quotas' },
      { label: '请求历史', tab: 'history' },
      { label: '实时日志', tab: 'logs' },
    ],
  },
]
const helpTroubleshootingItems = [
  {
    problem: '客户端没有请求进入 OmniProxy',
    action: '先确认本地代理已启动，Base URL 使用 127.0.0.1 对应端口；如果使用一键配置，重新写入并检查客户端配置文件路径。',
  },
  {
    problem: '账号返回 401、鉴权失败或显示无效',
    action: '在账号管理中验证该账号。订阅类账号优先刷新认证，API Key 类账号检查上游 Base URL 和 Key 类型是否匹配。',
  },
  {
    problem: '频繁 429 或额度过低',
    action: '到额度页查看每个账号窗口和余额。需要临时隔离时，只选择可用账号；需要自动避让时调高低额度跳过阈值。',
  },
  {
    problem: 'Claude Code 模型不符合预期',
    action: '在一键配置中重新选择最多 4 个 Claude 模型槽位，并注意 DeepSeek、MiMo、Kimi、GLM 的模型名差异。',
  },
  {
    problem: '响应慢或并发被占用',
    action: '看仪表盘实时连接和请求历史。优先平衡使用会避开并发占用更高的账号，队列模式更适合固定优先级。',
  },
]
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
const totalProxyRequests = computed(() =>
  tokens.value.reduce((sum, item) => sum + Number(item.stats?.requestCount || 0), 0),
)
const totalProxyTokens = computed(() =>
  tokens.value.reduce((sum, item) => sum + Number(item.stats?.totalTokens || 0), 0),
)
const totalProxyInputTokens = computed(() =>
  tokens.value.reduce((sum, item) => sum + Number(item.stats?.inputTokens || 0), 0),
)
const totalProxyOutputTokens = computed(() =>
  tokens.value.reduce((sum, item) => sum + Number(item.stats?.outputTokens || 0), 0),
)
const dailyUsageRows = computed(() => aggregateDailyUsage(tokens.value))
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

function closeTitlebarUpdatePopover() {
  titlebarUpdatePopoverOpen.value = false
}

function toggleTitlebarUpdatePopover() {
  titlebarUpdatePopoverOpen.value = !titlebarUpdatePopoverOpen.value
}

async function confirmTitlebarUpdatePopover() {
  const prompt = titlebarUpdatePrompt.value
  closeTitlebarUpdatePopover()
  if (prompt.canDownload && prompt.update) {
    await startUpdateDownload(prompt.update)
    return
  }
  selectTab('about')
}

function handleTitlebarUpdateOutsidePointer() {
  closeTitlebarUpdatePopover()
}

function handleTitlebarUpdateKeydown(event) {
  if (event.key === 'Escape') {
    closeTitlebarUpdatePopover()
  }
}

function saveWorkspaceScrollPosition(tabKey = activeTab.value) {
  const target = workspaceRef.value
  if (!target || !tabKey) return
  workspaceScrollPositions[tabKey] = target.scrollTop || 0
}

function restoreWorkspaceScrollPosition(tabKey = activeTab.value) {
  const target = workspaceRef.value
  if (!target || !tabKey) return
  const savedTop = Number(workspaceScrollPositions[tabKey] || 0)
  const maxTop = Math.max(0, target.scrollHeight - target.clientHeight)
  target.scrollTop = Math.min(savedTop, maxTop)
}

function restoreActiveWorkspaceScroll() {
  nextTick(() => {
    restoreWorkspaceScrollPosition(activeTab.value)
  })
}

function handleWorkspaceScroll(event) {
  if (workspaceScrollSavePaused) return
  if (event?.currentTarget !== workspaceRef.value) return
  saveWorkspaceScrollPosition(activeTab.value)
}

function pauseWorkspaceScrollSaving() {
  workspaceScrollSavePaused = true
}

function resumeWorkspaceScrollSaving() {
  workspaceScrollSavePaused = false
}

function afterPageEnter() {
  restoreActiveWorkspaceScroll()
  resumeWorkspaceScrollSaving()
}

function clearWorkspaceScrollbarTimer() {
  if (workspaceScrollbarTimer) {
    window.clearTimeout(workspaceScrollbarTimer)
    workspaceScrollbarTimer = null
  }
}

function hideWorkspaceScrollbar() {
  clearWorkspaceScrollbarTimer()
  workspaceScrollbarVisible.value = false
}

function handleWorkspacePointerMove(event) {
  const target = event.currentTarget
  if (!target || target.scrollHeight <= target.clientHeight) {
    hideWorkspaceScrollbar()
    return
  }

  const rect = target.getBoundingClientRect()
  const scrollbarHotZone = 14
  const inScrollbarArea = event.clientX >= rect.right - scrollbarHotZone && event.clientX <= rect.right

  if (!inScrollbarArea) {
    hideWorkspaceScrollbar()
    return
  }

  if (workspaceScrollbarVisible.value || workspaceScrollbarTimer) return
  workspaceScrollbarTimer = window.setTimeout(() => {
    workspaceScrollbarVisible.value = true
    workspaceScrollbarTimer = null
  }, 500)
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
  await refreshUpdateDownloadStatus()
  updateCheckTimer = window.setTimeout(() => checkForAvailableUpdate(), 2500)
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
  if (updateCheckTimer) {
    window.clearTimeout(updateCheckTimer)
    updateCheckTimer = null
  }
  stopUpdateDownloadPolling()
  if (toastTimer) {
    window.clearTimeout(toastTimer)
    toastTimer = null
  }
  saveWorkspaceScrollPosition()
  clearWorkspaceScrollbarTimer()
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
  }
})

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
      getDataDirectory(),
      getAutoStartStatus(),
      getAppInfo(),
    ])
    tokens.value = loadedTokens
    logs.value = loadedLogs
    activeRequests.value = loadedActiveRequests
    requestHistory.value = loadedHistory
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
    ]
    if (activeTab.value !== 'history') {
      requests.push(getHistory({ limit: 200 }))
    }
    const [loadedLogs, loadedStatus, loadedTokens, loadedActiveRequests, loadedHistory] = await Promise.all(requests)
    logs.value = loadedLogs
    tokens.value = loadedTokens
    activeRequests.value = loadedActiveRequests
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

async function checkForAvailableUpdate({ manual = false } = {}) {
  let promptedVersion = ''
  if (manual) {
    updateChecking.value = true
    errorMessage.value = ''
    successMessage.value = ''
  }
  try {
    const update = await checkForUpdates()
    lastUpdateInfo.value = update
    lastUpdateCheckedAt.value = new Date().toISOString()
    if (update?.currentVersion) {
      appInfo.version = update.currentVersion
      const normalizedVersion = String(update.currentVersion).trim().toLowerCase()
      appInfo.isDevelopment = normalizedVersion === 'dev' || normalizedVersion === 'development'
    }
    if (appInfo.isDevelopment) {
      if (manual) {
        successMessage.value = '开发版本不会请求远端更新接口'
      }
      return
    }
    if (!update?.updateAvailable || !update.latestVersion) {
      if (manual) {
        successMessage.value = update?.currentVersion
          ? `当前已是最新版本（${update.currentVersion}）`
          : '当前已是最新版本'
      }
      return
    }
    promptedVersion = update.latestVersion
    if (!manual && window.localStorage?.getItem(skippedUpdateVersionKey) === update.latestVersion) {
      return
    }

    if (!update.downloadUrl || !update.checksumUrl) {
      if (manual) {
        errorMessage.value = `发现新版本 ${update.latestVersion}，但没有可用的安装包或校验文件`
      }
      return
    }
    const currentVersion = update.currentVersion || '当前版本'
    await ElMessageBox.confirm(
      `当前版本：${currentVersion}\n最新版本：${update.latestVersion}\n\n是否下载更新安装包？`,
      `发现新版本 ${update.latestVersion}`,
      {
        confirmButtonText: '下载更新',
        cancelButtonText: '跳过此版本',
        distinguishCancelAndClose: true,
        type: 'info',
      },
    )
    await startUpdateDownload(update)
  } catch (action) {
    if (action instanceof Error) {
      if (manual) {
        errorMessage.value = action.message
      }
      return
    }
    if (typeof action === 'string') {
      if (action === 'cancel' && promptedVersion) {
        window.localStorage?.setItem(skippedUpdateVersionKey, promptedVersion)
      } else if (manual && action !== 'close') {
        errorMessage.value = action
      }
      return
    }
    if (manual && action) {
      errorMessage.value = String(action)
    }
  } finally {
    if (manual) {
      updateChecking.value = false
    }
  }
}

function manualCheckForUpdates() {
  return checkForAvailableUpdate({ manual: true })
}

function updateDownloadPayload(update = lastUpdateInfo.value) {
  return {
    version: update?.latestVersion || '',
    downloadUrl: update?.downloadUrl || '',
    checksumUrl: update?.checksumUrl || '',
    fileName: update?.downloadFileName || '',
    expectedSize: Number(update?.downloadSize || 0),
  }
}

async function startUpdateDownload(update = lastUpdateInfo.value) {
  const payload = updateDownloadPayload(update)
  if (!payload.downloadUrl) {
    errorMessage.value = '没有可用的安装包下载地址'
    return
  }
  if (!payload.checksumUrl) {
    errorMessage.value = '没有可用的 SHA256 校验文件，已取消应用内下载'
    return
  }
  errorMessage.value = ''
  successMessage.value = ''
  const status = await downloadUpdate(payload)
  updateDownloadStatus.value = status || { state: 'downloading' }
  startUpdateDownloadPolling()
}

function startUpdateDownloadPolling() {
  if (updateDownloadTimer) {
    return
  }
  updateDownloadTimer = window.setInterval(refreshUpdateDownloadStatus, 1000)
}

function stopUpdateDownloadPolling() {
  if (!updateDownloadTimer) {
    return
  }
  window.clearInterval(updateDownloadTimer)
  updateDownloadTimer = null
}

async function refreshUpdateDownloadStatus() {
  try {
    const status = await getUpdateDownloadStatus()
    if (status) {
      updateDownloadStatus.value = status
      if (status.state === 'downloading') {
        startUpdateDownloadPolling()
      } else if (['downloaded', 'failed', 'installing', 'idle'].includes(status.state)) {
        stopUpdateDownloadPolling()
      }
    }
  } catch {
    stopUpdateDownloadPolling()
  }
}

async function installReadyUpdate() {
  try {
    await ElMessageBox.confirm(
      '安装器将从已校验的本地文件启动。安装过程中可能需要关闭当前 OmniProxy 窗口。',
      '安装更新',
      {
        confirmButtonText: '立即安装',
        cancelButtonText: '稍后',
        type: 'info',
      },
    )
    const status = await installDownloadedUpdate()
    updateDownloadStatus.value = status || updateDownloadStatus.value
    successMessage.value = '更新安装器已启动'
  } catch (action) {
    if (action instanceof Error) {
      errorMessage.value = action.message
    }
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
    const [usage, dates] = await Promise.all([
      getBillingUsage(normalizedDate),
      getBillingDates(30),
    ])
    billingUsage.value = usage || []
    billingDates.value = dates || []
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
    baseUrl: provider === 'sub2api' ? config.sub2apiBaseUrl : '',
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
  const baseUrl = provider === 'sub2api' ? form.baseUrl.trim() : ''
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
  if (provider === 'sub2api') {
    if (!baseUrl) {
      errorMessage.value = 'sub2api Base URL 不能为空'
      return
    }
    try {
      const parsed = new URL(baseUrl)
      if (!['http:', 'https:'].includes(parsed.protocol) || !parsed.host) {
        errorMessage.value = 'sub2api Base URL 格式不正确'
        return
      }
    } catch {
      errorMessage.value = 'sub2api Base URL 格式不正确'
      return
    }
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
    baseUrl: provider === 'sub2api' ? config.sub2apiBaseUrl : '',
    tokenText: '',
  })
}

function closeBatchImport() {
  if (batchImporting.value) return
  batchImportForm.visible = false
}

function onBatchImportProviderChange() {
  if (batchImportForm.provider === 'sub2api' && !batchImportForm.baseUrl) {
    batchImportForm.baseUrl = config.sub2apiBaseUrl
  } else if (batchImportForm.provider !== 'sub2api') {
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
  const baseUrl = provider === 'sub2api' ? batchImportForm.baseUrl.trim() : ''
  const tokenText = batchImportForm.tokenText.trim()

  if (!tokenText) {
    errorMessage.value = '请先粘贴要导入的 API Key'
    return
  }
  if (provider === 'sub2api') {
    if (!baseUrl) {
      errorMessage.value = 'sub2api Base URL 不能为空'
      return
    }
    try {
      const parsed = new URL(baseUrl)
      if (!['http:', 'https:'].includes(parsed.protocol) || !parsed.host) {
        errorMessage.value = 'sub2api Base URL 格式不正确'
        return
      }
    } catch {
      errorMessage.value = 'sub2api Base URL 格式不正确'
      return
    }
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
      '将删除本地每日账单汇总，详细请求历史不会删除。此操作无法撤销。',
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
      '将删除本地请求历史明细，已保存的每日账单汇总会保留。此操作无法撤销。',
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
    requestHistory.value = []
    requestHistorySummary.value = null
    await refreshBilling()
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
  if (form.provider === 'zo') {
    return '粘贴 zo_sk_ 开头的 Zo Access Token'
  }
  if (form.provider === 'custom') {
    return '粘贴自定义网关 API Key'
  }
  return '粘贴 API Key'
}

function schedulingModeText(value) {
  if (value === 'balanced') return '优先平衡使用'
  if (value === 'queue') return '队列模式'
  return value || '-'
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
    return `${label} · ${item.region === 'sgp' ? '海外 SGP' : '中国区'}`
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
  if (form.provider === 'sub2api' && !form.baseUrl) {
    form.baseUrl = config.sub2apiBaseUrl
  } else if (form.provider !== 'sub2api') {
    form.baseUrl = ''
  }
  if (form.editingId && form.provider !== form.originalProvider) {
    form.tokenValue = ''
  }
}

function statusLabel(status) {
  return statusMeta[status]?.label || status
}

function statusClass(status) {
  return statusMeta[status]?.className || 'muted'
}

function statusType(status) {
  const types = {
    active: 'success',
    low: 'warning',
    exhausted: 'info',
    invalid: 'danger',
  }
  return types[status] || 'info'
}

function providerLabel(providerKey) {
  return providers.find((item) => item.key === providerKey)?.label || providerKey
}

const knownClientTools = [
  { key: 'codex', label: 'Codex' },
  { key: 'claude', label: 'Claude Code' },
  { key: 'claude-desktop', label: 'Claude Code Desktop' },
  { key: 'gemini', label: 'Gemini CLI' },
  { key: 'opencode', label: 'OpenCode' },
  { key: 'pi', label: 'Pi Coding Agent' },
  { key: 'deepseek-tui', label: 'DeepSeek-TUI' },
  { key: 'openrouter', label: 'OpenRouter' },
  { key: 'tokenrouter', label: 'TokenRouter' },
  { key: 'sub2api', label: 'sub2api' },
  { key: 'cursor', label: 'Cursor' },
  { key: 'vscode', label: 'VS Code' },
  { key: 'windsurf', label: 'Windsurf' },
  { key: 'aider', label: 'Aider' },
  { key: 'continue', label: 'Continue' },
  { key: 'custom', label: '自定义网关' },
  { key: 'api', label: 'API Client' },
]

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

function planLabel(plan) {
  const normalized = String(plan || '').toLowerCase()
  const labels = {
    team: 'Team',
    pro: 'Pro',
    plus: 'Plus',
    free: 'Free',
    enterprise: 'Enterprise',
  }
  return labels[normalized] || plan || '未知'
}

function usageUpdatedAt(item) {
  return item.usage?.updatedAt ? formatTime(item.usage.updatedAt) : '-'
}

function quotaPercentValue(item, field) {
  if (!item?.usage?.subscriptionQuotaAvailable) return 0
  const value = Number(item.usage?.[field])
  if (!Number.isFinite(value)) return 0
  return Math.max(0, Math.min(100, Math.round(value)))
}

function quotaPercentText(item, field) {
  return item?.usage?.subscriptionQuotaAvailable ? `${quotaPercentValue(item, field)}%` : '-'
}

function formatBalance(value) {
  const number = Number(value || 0)
  const fractionDigits = Math.abs(number) > 0 && Math.abs(number) < 1 ? 4 : 2
  return new Intl.NumberFormat('zh-CN', {
    minimumFractionDigits: fractionDigits,
    maximumFractionDigits: fractionDigits,
  }).format(number)
}

function apiBalanceSummaryMeta(summary) {
  const parts = [`${formatNumber(summary.count)} 个 API Key`]
  if (Number(summary.total || 0) > 0) {
    parts.push(`总额 ${formatBalance(summary.total)} ${summary.unit}`)
  }
  if (Number(summary.used || 0) > 0) {
    parts.push(`已用 ${formatBalance(summary.used)} ${summary.unit}`)
  }
  return parts.join(' · ')
}

function hasBalanceUsage(item) {
  return Boolean(item.usage?.balanceUnit)
}

function hasOpenRouterQuotaUsage(item) {
  if (item?.provider !== 'openrouter') return false
  return hasBalanceUsage(item) || Boolean(item.usage?.balanceUnlimited)
}

function quotaDisplay(item) {
  if (item.usage?.balanceUnlimited) {
    return '不限制'
  }
  if (hasBalanceUsage(item)) {
    return `${formatBalance(item.usage.balanceRemaining)} ${item.usage.balanceUnit}`
  }
  return `${item.remaining}%`
}

function quotaStatLabel(item) {
  return hasBalanceUsage(item) ? '账户余额' : 'API 剩余额度'
}

function quotaStatMeta(item) {
  if (hasBalanceUsage(item)) {
    const parts = []
    if (item.usage?.balanceUnlimited) {
      parts.push('未设置消费上限')
    }
    if (Number(item.usage?.balanceTotal || 0) > 0) {
      parts.push(`总额 ${formatBalance(item.usage.balanceTotal)} ${item.usage.balanceUnit}`)
    }
    if (Number(item.usage?.balanceUsed || 0) > 0) {
      parts.push(`已用 ${formatBalance(item.usage.balanceUsed)} ${item.usage.balanceUnit}`)
    }
    parts.push(`最后更新 ${usageUpdatedAt(item)}`)
    return parts.join(' · ')
  }
  return `最后更新 ${usageUpdatedAt(item)}`
}

function openRouterQuotaValue(item, field) {
  if (!hasOpenRouterQuotaUsage(item)) {
    return '-'
  }
  return `${formatBalance(item.usage?.[field])} ${item.usage.balanceUnit}`
}

function openRouterQuotaRemaining(item) {
  if (!hasOpenRouterQuotaUsage(item)) {
    return '待刷新'
  }
  if (item.usage?.balanceUnlimited) {
    return '不限制'
  }
  if (Number(item.usage?.balanceTotal || 0) <= 0 && Number(item.usage?.balanceRemaining || 0) <= 0) {
    return '未返回'
  }
  return openRouterQuotaValue(item, 'balanceRemaining')
}

function openRouterQuotaLimit(item) {
  if (!hasOpenRouterQuotaUsage(item)) {
    return '-'
  }
  if (item.usage?.balanceUnlimited) {
    return '不限制'
  }
  if (Number(item.usage?.balanceTotal || 0) <= 0) {
    return '未设置'
  }
  return openRouterQuotaValue(item, 'balanceTotal')
}

function openRouterQuotaMeta(item) {
  if (!hasOpenRouterQuotaUsage(item)) {
    return item?.disabled ? '已停用，启用后可刷新额度' : '点击刷新额度获取 OpenRouter /key 余额'
  }
  const parts = []
  if (item?.usage?.planType) {
    parts.push(item.usage.planType)
  }
  if (item?.usage?.message) {
    parts.push(item.usage.message)
  }
  parts.push(`最后更新 ${usageUpdatedAt(item)}`)
  return parts.join(' · ')
}

function validationSuccessMessage(token, result) {
  if (token?.provider === 'openrouter' && result?.usage) {
    const usage = result.usage
    if (usage.balanceUnlimited) {
      const used = usage.balanceUnit ? `${formatBalance(usage.balanceUsed)} ${usage.balanceUnit}` : '-'
      return `OpenRouter 额度已刷新：消费上限不限制，已用 ${used}`
    }
    if (usage.balanceUnit) {
      const remaining = `${formatBalance(usage.balanceRemaining)} ${usage.balanceUnit}`
      const used = `${formatBalance(usage.balanceUsed)} ${usage.balanceUnit}`
      return `OpenRouter 额度已刷新：剩余 ${remaining}，已用 ${used}`
    }
  }
  return `验证通过：${result.status}，耗时 ${result.durationMs}ms`
}

function balancePackages(item) {
  return Array.isArray(item?.usage?.balancePackages) ? item.usage.balancePackages : []
}

function balancePackageCounts(pkg) {
  const status = String(pkg?.status || '').toUpperCase()
  const type = String(pkg?.consumeType || '').toUpperCase()
  return (!status || status === 'EFFECTIVE') && (!type || type === 'TOKENS')
}

function balancePackageTypeLabel(pkg) {
  const type = String(pkg?.consumeType || '').toUpperCase()
  if (type === 'TIMES') return '次数包'
  if (type === 'TOKENS' || !type) return 'Token 包'
  return type
}

function balancePackageAmount(pkg) {
  const unit = pkg?.unit || (String(pkg?.consumeType || '').toUpperCase() === 'TIMES' ? '次' : 'Token')
  return `${formatNumber(pkg?.balanceRemaining)} ${unit}`
}

function balancePackageMeta(pkg) {
  const parts = []
  if (pkg?.balanceTotal && Number(pkg.balanceTotal) !== Number(pkg.balanceRemaining || 0)) {
    parts.push(`总量 ${formatNumber(pkg.balanceTotal)}`)
  }
  if (pkg?.status && pkg.status !== 'EFFECTIVE') {
    parts.push(pkg.status)
  }
  if (pkg?.expirationTime) {
    parts.push(`到期 ${formatPackageExpiration(pkg.expirationTime)}`)
  }
  if (pkg?.suitableModel) {
    parts.push(pkg.suitableModel)
  }
  return parts.join(' · ') || (balancePackageCounts(pkg) ? '计入 Token 余额' : '仅展示，不计入 Token 余额')
}

function formatPackageExpiration(value) {
  if (!value) return '-'
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))
}

function apiQuotaDisplay(item) {
  if (hasBalanceUsage(item)) {
    return quotaDisplay(item)
  }
  const remaining = Number(item.usage?.apiRemaining || 0)
  if (remaining > 0) {
    return `余量 ${formatNumber(remaining)}`
  }
  return displayStatusLabel(item)
}

function apiQuotaMeta(item) {
  if (hasBalanceUsage(item)) {
    return quotaStatMeta(item)
  }
  if (Number(item.usage?.apiRemaining || 0) > 0) {
    return `来自上游 rate-limit header · 最后更新 ${usageUpdatedAt(item)}`
  }
  return healthSummary(item)
}

function isCodexToken(item) {
  return item?.provider === 'openai' && item?.credentialType === 'codex_auth_json'
}

function isClaudeOAuthToken(item) {
  return item?.provider === 'anthropic' && item?.credentialType === 'claude_oauth_json'
}

function isMimoTokenPlan(item) {
  return item?.provider === 'xiaomi' && item?.credentialType === 'mimo_token_plan'
}

function isZhipuCodingPlan(item) {
  return item?.provider === 'zhipu' && item?.credentialType === 'coding_plan'
}

function showQuotaWindows(item) {
  return isCodexToken(item) || isMimoTokenPlan(item) || Boolean(item?.usage?.subscriptionQuotaAvailable)
}

function quotaWindowAvailable(item, windowName) {
  if (!item?.usage?.subscriptionQuotaAvailable) return false
  const prefix = windowName === 'secondary' ? 'secondary' : 'primary'
  return ['UsedPercent', 'RemainingPercent', 'ResetAt'].some((suffix) => {
    const value = Number(item.usage?.[`${prefix}${suffix}`])
    return Number.isFinite(value) && value > 0
  })
}

function isCodexFreePlan(item) {
  return isCodexToken(item) && String(item?.usage?.planType || '').trim().toLowerCase() === 'free'
}

function showPrimaryQuotaWindow(item) {
  if (!showQuotaWindows(item)) return false
  if (!item?.usage?.subscriptionQuotaAvailable) return true
  if (isCodexFreePlan(item) && quotaWindowAvailable(item, 'secondary')) return false
  return quotaWindowAvailable(item, 'primary')
}

function showSecondaryQuotaWindow(item) {
  if (!showQuotaWindows(item)) return false
  if (!item?.usage?.subscriptionQuotaAvailable) return true
  if (isCodexFreePlan(item) && quotaWindowAvailable(item, 'primary')) return false
  return quotaWindowAvailable(item, 'secondary')
}

function quotaWindowCount(item) {
  return Number(showPrimaryQuotaWindow(item)) + Number(showSecondaryQuotaWindow(item))
}

function quotaPrimaryLabel(item) {
  if (isZhipuCodingPlan(item)) return '窗口额度'
  if (isCodexFreePlan(item)) return '1 周额度'
  return isMimoTokenPlan(item) ? '本月额度' : '5h额度'
}

function quotaSecondaryLabel(item) {
  if (isZhipuCodingPlan(item)) return '周额度'
  return isMimoTokenPlan(item) ? '套餐额度' : '1 周额度'
}

function quotaResetLabel(item) {
  return isMimoTokenPlan(item) ? '到期' : '重置'
}

function quotaUnavailableText(item) {
  if (isCodexToken(item)) return '点击刷新额度获取'
  if (isMimoTokenPlan(item)) return 'Token Plan 暂无订阅额度'
  return '暂无订阅额度'
}

function weeklyLimitReached(item) {
  if (!item?.usage?.subscriptionQuotaAvailable) return false
  if (!isZhipuCodingPlan(item) && !isCodexToken(item) && !isMimoTokenPlan(item)) return false
  const remaining = Number(item.usage?.secondaryRemainingPercent)
  const used = Number(item.usage?.secondaryUsedPercent)
  return Number.isFinite(remaining) && remaining <= 0 && Number.isFinite(used) && used > 0
}

function tokenUsageMetrics(item) {
  const total = Number(item.stats?.totalTokens || 0)
  const input = Number(item.stats?.inputTokens || 0)
  const output = Number(item.stats?.outputTokens || 0)
  const requests = Number(item.stats?.requestCount || 0)
  if (total > 0) {
    return [
      { label: 'Token', value: formatNumber(total) },
      { label: '入', value: formatNumber(input) },
      { label: '出', value: formatNumber(output) },
    ]
  }
  return [{ label: 'Token', value: requests > 0 ? '未上报' : '0' }]
}

function aggregateDailyUsage(items) {
  const byDate = new Map()
  for (const item of items) {
    for (const daily of item.stats?.daily || []) {
      const current = byDate.get(daily.date) || {
        date: daily.date,
        requestCount: 0,
        inputTokens: 0,
        outputTokens: 0,
        totalTokens: 0,
      }
      current.requestCount += Number(daily.requestCount || 0)
      current.inputTokens += Number(daily.inputTokens || 0)
      current.outputTokens += Number(daily.outputTokens || 0)
      current.totalTokens += Number(daily.totalTokens || 0)
      byDate.set(daily.date, current)
    }
  }
  return Array.from(byDate.values()).sort((a, b) => b.date.localeCompare(a.date))
}

function isCooling(item) {
  return item?.cooldownUntil && new Date(item.cooldownUntil).getTime() > Date.now()
}

function displayStatusLabel(item) {
  if (item?.disabled) return '已停用'
  if (isCooling(item)) return '冷却中'
  return statusLabel(item.status)
}

function displayStatusClass(item) {
  if (item?.disabled) return 'muted'
  if (isCooling(item)) return 'warning'
  return statusClass(item.status)
}

function displayStatusType(item) {
  if (item?.disabled) return 'info'
  if (isCooling(item)) return 'warning'
  return statusType(item.status)
}

function healthSummary(item) {
  if (item?.disabled) {
    return '已停用，不参与调度和自动检查'
  }
  if (isCooling(item)) {
    return `冷却到 ${formatTime(item.cooldownUntil)} 后自动复检`
  }
  if (item.health?.lastCheckedAt) {
    const status = item.health.lastStatus ? ` · ${item.health.lastStatus}` : ''
    return `健康检查 ${formatTime(item.health.lastCheckedAt)}${status}`
  }
  return '等待健康检查'
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

      <section v-else-if="activeTab === 'quotas'" key="quotas" class="panel quotas-page-panel">
        <div class="section-heading">
          <div>
            <h2>账号状态</h2>
            <p>按厂商查看订阅额度、API 剩余额度和代理用量</p>
          </div>
          <div class="section-actions">
            <el-button :icon="Refresh" @click="refreshAll">刷新列表</el-button>
            <el-button type="primary" plain :icon="RefreshRight" :loading="refreshingProvider" @click="refreshProviderQuotas">
              全部刷新
            </el-button>
          </div>
        </div>

        <div class="provider-switch" aria-label="厂商选择">
          <button
            v-for="provider in providers"
            :key="provider.key"
            type="button"
            :class="{ active: activeProvider === provider.key }"
            @click="selectProvider(provider.key)"
          >
            {{ provider.label }}
            <span>{{ providerTokens(provider.key).length }}</span>
          </button>
        </div>

        <div class="provider-summary">
          <div>
            <h3>{{ activeProviderInfo.label }}</h3>
            <p>{{ activeProviderEnabledCount }} 启用 / {{ activeProviderTokens.length }} 总数 · {{ activeProviderInfo.note }}</p>
          </div>
          <div
            v-if="activeProvider === 'openrouter' && activeProviderTokens.length"
            class="provider-api-balance-summary openrouter-provider-summary"
            aria-label="OpenRouter 额度"
          >
            <article v-for="item in activeProviderTokens" :key="`openrouter-provider-summary-${item.id}`">
              <span>{{ item.name }}</span>
              <strong>{{ openRouterQuotaRemaining(item) }}</strong>
              <small>{{ openRouterQuotaMeta(item) }}</small>
              <small>已用 {{ openRouterQuotaValue(item, 'balanceUsed') }} · 上限 {{ openRouterQuotaLimit(item) }}</small>
            </article>
          </div>
          <div
            v-else-if="activeProviderAPIBalanceSummaries.length"
            class="provider-api-balance-summary"
            aria-label="API Key 总额度"
          >
            <article v-for="summary in activeProviderAPIBalanceSummaries" :key="summary.unit">
              <span>API Key 总额度 · {{ summary.unit }}</span>
              <strong>{{ formatBalance(summary.remaining) }} {{ summary.unit }}</strong>
              <small>{{ apiBalanceSummaryMeta(summary) }}</small>
            </article>
          </div>
        </div>

        <div class="quota-card-grid">
          <el-card
            v-for="item in activeProviderTokens"
            :key="item.id"
            :class="['quota-card', { 'quota-card-disabled': item.disabled }]"
            shadow="hover"
            :body-style="{ padding: '0' }"
          >
            <div class="quota-card-inner">
              <div class="quota-card-head">
                <div class="quota-card-title-row">
                  <strong class="account-name">{{ item.name }}</strong>
                  <div class="quota-status-tags">
                    <el-tag
                      v-if="item.usage?.subscriptionQuotaAvailable && item.usage?.planType"
                      type="primary"
                      effect="plain"
                      class="quota-chip"
                    >
                      {{ planLabel(item.usage?.planType) }}
                    </el-tag>
                    <el-tag v-if="weeklyLimitReached(item)" type="danger" effect="light">周限额已达</el-tag>
                    <el-tag :type="displayStatusType(item)" effect="light" class="status-tag quota-chip">{{ displayStatusLabel(item) }}</el-tag>
                  </div>
                </div>
                <div class="quota-card-meta-row">
                  <span>{{ isCodexToken(item) ? 'Codex auth.json' : credentialLabel(item) }} · {{ providerLabel(item.provider) }}</span>
                  <div class="quota-card-actions">
                    <el-button
                      size="small"
                      :class="['account-action-button', 'account-select-button', { selected: item.selected }]"
                      :type="item.selected ? 'primary' : ''"
                      :plain="!item.selected"
                      :icon="item.selected ? CircleCheckFilled : Plus"
                      :loading="switchingOnlyTokenIds[item.id]"
                      :disabled="item.disabled"
                      @click="toggleTokenSelected(item)"
                    >
                      {{ item.selected ? '已选' : '选择' }}
                    </el-button>
                    <el-tooltip content="刷新额度" placement="top">
                      <el-button
                        size="small"
                        class="account-action-button"
                        plain
                        :icon="Refresh"
                        :loading="validatingIds[item.id]"
                        @click="refreshQuota(item)"
                      >
                        刷新
                      </el-button>
                    </el-tooltip>
                  </div>
                </div>
                <small class="health-line">{{ healthSummary(item) }}</small>
              </div>

              <div
                :class="[
                  'quota-layout',
                  {
                    'codex-layout': isCodexToken(item),
                    'single-window-layout': quotaWindowCount(item) === 1,
                    'api-only-layout': !isCodexToken(item) && quotaWindowCount(item) === 0,
                  },
                ]"
              >
                <div v-if="showPrimaryQuotaWindow(item)" class="quota-limit">
                  <div class="quota-limit-title">
                    <span>{{ quotaPrimaryLabel(item) }}</span>
                    <strong v-if="item.usage?.subscriptionQuotaAvailable">{{ quotaPercentText(item, 'primaryRemainingPercent') }}</strong>
                    <strong v-else>-</strong>
                  </div>
                  <el-progress
                    :percentage="quotaPercentValue(item, 'primaryRemainingPercent')"
                    :show-text="false"
                    :stroke-width="8"
                  />
                  <small v-if="item.usage?.subscriptionQuotaAvailable" class="quota-detail quota-reset-detail">
                    <span>已用 <strong>{{ quotaPercentText(item, 'primaryUsedPercent') }}</strong></span>
                    <span>{{ quotaResetLabel(item) }} <strong>{{ formatResetTime(item.usage.primaryResetAt) }}</strong></span>
                  </small>
                  <small v-else>{{ quotaUnavailableText(item) }}</small>
                </div>

                <div v-if="showSecondaryQuotaWindow(item)" class="quota-limit">
                  <div class="quota-limit-title">
                    <span>{{ quotaSecondaryLabel(item) }}</span>
                    <strong v-if="item.usage?.subscriptionQuotaAvailable">{{ quotaPercentText(item, 'secondaryRemainingPercent') }}</strong>
                    <strong v-else>-</strong>
                  </div>
                  <el-progress
                    :percentage="quotaPercentValue(item, 'secondaryRemainingPercent')"
                    :show-text="false"
                    :stroke-width="8"
                  />
                  <small v-if="item.usage?.subscriptionQuotaAvailable" class="quota-detail quota-reset-detail">
                    <span>已用 <strong>{{ quotaPercentText(item, 'secondaryUsedPercent') }}</strong></span>
                    <span>{{ quotaResetLabel(item) }} <strong>{{ formatResetTime(item.usage.secondaryResetAt) }}</strong></span>
                  </small>
                  <small v-else>{{ quotaUnavailableText(item) }}</small>
                </div>

                <div v-if="!isCodexToken(item)" class="quota-stat quota-stat-balance">
                  <span>{{ quotaStatLabel(item) }}</span>
                  <strong>{{ hasBalanceUsage(item) ? quotaDisplay(item) : `${item.usage?.apiRemaining || item.remaining}%` }}</strong>
                  <small>{{ quotaStatMeta(item) }}</small>
                </div>

                <div class="quota-stat quota-stat-usage">
                  <span>代理请求</span>
                  <strong>{{ formatNumber(item.stats?.requestCount) }} 次</strong>
                  <small class="quota-detail token-usage-detail">
                    <span v-for="metric in tokenUsageMetrics(item)" :key="metric.label">
                      {{ metric.label }} <strong>{{ metric.value }}</strong>
                    </span>
                  </small>
                </div>
              </div>

              <div v-if="balancePackages(item).length" class="balance-package-list">
                <div class="balance-package-head">
                  <span>资源包明细</span>
                  <small>Token 包计入余额，次数包仅展示</small>
                </div>
                <div
                  v-for="(pkg, index) in balancePackages(item)"
                  :key="`${pkg.name || 'package'}-${pkg.consumeType || 'token'}-${pkg.expirationTime || ''}-${index}`"
                  :class="['balance-package-row', { muted: !balancePackageCounts(pkg) }]"
                  :title="pkg.suitableScene || pkg.suitableModel || pkg.name"
                >
                  <div>
                    <strong>{{ pkg.name || balancePackageTypeLabel(pkg) }}</strong>
                    <small>{{ balancePackageMeta(pkg) }}</small>
                  </div>
                  <div>
                    <span class="package-type">{{ balancePackageTypeLabel(pkg) }}</span>
                    <strong>{{ balancePackageAmount(pkg) }}</strong>
                  </div>
                </div>
              </div>
            </div>
          </el-card>
          <div v-if="!activeProviderTokens.length" class="empty">
            暂无 {{ activeProviderInfo.label }} 账号
          </div>
        </div>
      </section>

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
        :clearing-billing-usage="clearingBillingUsage"
        :clearing-request-history="clearingRequestHistory"
        @persist-config="persistConfig"
        @choose-data-directory="chooseDataDirectory"
        @toggle-auto-start="toggleAutoStart"
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
        @install-update="installReadyUpdate"
        @open-url="openExternalURL"
      />

      <section v-else-if="activeTab === 'quickstart'" key="quickstart" class="help-panel quickstart-panel">
        <div class="help-grid">
          <article class="wide-help">
            <strong>Codex</strong>
            <p>本地 Codex 会写入 <code>%USERPROFILE%\.codex\config.toml</code>。OpenAI Codex 使用 auth.json；sub2api 使用账号池里的 sub2api API Key。</p>
            <pre class="help-code"><code>OpenAI Codex Base URL: http://127.0.0.1:{{ config.proxyPort }}/backend-api/codex
sub2api OpenAI/Codex: http://127.0.0.1:{{ config.proxyPort }}/sub2api
sub2api Anthropic: http://127.0.0.1:{{ config.proxyPort }}/sub2api/anthropic
sub2api Gemini: http://127.0.0.1:{{ config.proxyPort }}/sub2api/gemini
Zo Computer: http://127.0.0.1:{{ config.proxyPort }}/zo</code></pre>
            <div class="help-actions">
              <el-button type="primary" :icon="MagicStick" :loading="codexConfiguring" @click="configureLocalCodex">
                {{ codexConfiguring ? '配置中' : '配置 Codex OpenAI' }}
              </el-button>
              <el-button type="primary" plain :icon="MagicStick" :loading="codexSub2APIConfiguring" @click="configureLocalCodexSub2API">
                {{ codexSub2APIConfiguring ? '配置中' : '配置 Codex sub2api' }}
              </el-button>
              <el-button type="primary" plain :icon="MagicStick" :loading="codexZoConfiguring" @click="configureLocalCodexZo">
                {{ codexZoConfiguring ? '配置中' : '配置 Codex Zo' }}
              </el-button>
              <el-button :icon="RefreshRight" :loading="codexRestoring" @click="restoreLocalCodex">
                {{ codexRestoring ? '恢复中' : '恢复 Codex 配置' }}
              </el-button>
            </div>
          </article>

          <article class="wide-help">
            <strong>Claude Code</strong>
            <p>每次只接入一个 Claude Code 上游，也可以按需选择最多 4 个模型写入模型槽位，并清理 OmniProxy 旧配置。</p>
            <pre class="help-code"><code>Claude Router URL: http://127.0.0.1:{{ config.proxyPort }}/anthropic-router
DeepSeek: deepseek-v4-pro[1m] / deepseek-v4-flash
MiMo: MiMo-V2.5-Pro / MiMo-V2.5
Kimi model: kimi-for-coding
GLM model: glm-5.1
Zo models: claude-opus-4-7 / claude-sonnet-4-6</code></pre>
            <div class="claude-model-config">
              <div class="claude-model-config-head">
                <span>可选模型</span>
                <small>{{ selectedClaudeModels.length }} / {{ claudeModelSelectionLimit }}</small>
              </div>
              <div class="claude-model-picker" role="group" aria-label="Claude Code 可选模型">
                <label
                  v-for="option in claudeModelOptions"
                  :key="option.id"
                  :class="[
                    'claude-model-choice',
                    {
                      selected: selectedClaudeModels.includes(option.id),
                      disabled: isClaudeModelOptionDisabled(option.id),
                    },
                  ]"
                >
                  <input
                    v-model="selectedClaudeModels"
                    type="checkbox"
                    :value="option.id"
                    :disabled="isClaudeModelOptionDisabled(option.id)"
                  />
                  <span>
                    <strong>{{ option.label }}</strong>
                    <small>{{ option.description }}</small>
                  </span>
                </label>
              </div>
              <small class="claude-model-selection">
                已选：{{ selectedClaudeModelLabels.length ? selectedClaudeModelLabels.join('、') : '未选择' }}
              </small>
              <small class="claude-model-selection">
                CLI 写入 <code>%USERPROFILE%\.claude\settings.json</code>；Desktop 写入 Claude 3P Gateway Profile，配置后请完全退出并重启 Claude Desktop。
              </small>
            </div>
            <div class="claude-action-panel">
              <div class="claude-action-row">
                <span>按当前选择写入</span>
                <div class="help-actions claude-actions">
                  <el-button
                    type="success"
                    :icon="MagicStick"
                    :loading="claudeModelsConfiguring"
                    :disabled="!canConfigureClaudeModels"
                    @click="configureLocalClaudeModels"
                  >
                    {{ claudeModelsConfiguring ? '配置中' : 'Claude CLI' }}
                  </el-button>
                  <el-button
                    type="success"
                    plain
                    :icon="Monitor"
                    :loading="claudeDesktopConfiguring"
                    :disabled="!canConfigureClaudeModels"
                    @click="configureLocalClaudeDesktopModels"
                  >
                    {{ claudeDesktopConfiguring ? '配置中' : 'Claude Desktop' }}
                  </el-button>
                  <el-button :icon="RefreshRight" :loading="claudeDesktopRestoring" @click="restoreLocalClaudeDesktop">
                    {{ claudeDesktopRestoring ? '恢复中' : '恢复 Desktop' }}
                  </el-button>
                </div>
              </div>
              <div class="claude-action-row">
                <span>快捷单模型</span>
                <div class="help-actions claude-actions">
                  <el-button type="primary" :icon="MagicStick" :loading="deepSeekClaudeConfiguring" @click="configureLocalDeepSeekClaude">
                    {{ deepSeekClaudeConfiguring ? '配置中' : 'DeepSeek' }}
                  </el-button>
                  <el-button type="primary" plain :icon="MagicStick" :loading="mimoClaudeConfiguring" @click="configureLocalMimoClaude">
                    {{ mimoClaudeConfiguring ? '配置中' : 'MiMo' }}
                  </el-button>
                  <el-button type="primary" plain :icon="MagicStick" :loading="kimiClaudeConfiguring" @click="configureLocalKimiClaude">
                    {{ kimiClaudeConfiguring ? '配置中' : 'Kimi' }}
                  </el-button>
                  <el-button type="primary" plain :icon="MagicStick" :loading="zhipuClaudeConfiguring" @click="configureLocalZhipuClaude">
                    {{ zhipuClaudeConfiguring ? '配置中' : 'GLM' }}
                  </el-button>
                  <el-button type="primary" plain :icon="MagicStick" :loading="zoClaudeConfiguring" @click="configureLocalZoClaude">
                    {{ zoClaudeConfiguring ? '配置中' : 'Zo' }}
                  </el-button>
                  <el-button :icon="RefreshRight" :loading="mimoClaudeRestoring" @click="restoreLocalMimoClaude">
                    {{ mimoClaudeRestoring ? '恢复中' : '恢复 CLI' }}
                  </el-button>
                </div>
              </div>
            </div>
          </article>

          <article class="wide-help">
            <strong>Gemini CLI</strong>
            <p>写入 <code>%USERPROFILE%\.gemini\.env</code> 和 <code>settings.json</code>，使用账号池里的 Gemini API Key。</p>
            <pre class="help-code"><code>GOOGLE_GEMINI_BASE_URL=http://127.0.0.1:{{ config.proxyPort }}/gemini
GEMINI_MODEL=gemini-3-pro-preview</code></pre>
            <div class="help-actions">
              <el-button type="primary" :icon="MagicStick" :loading="geminiConfiguring" @click="configureLocalGemini">
                {{ geminiConfiguring ? '配置中' : '配置 Gemini CLI' }}
              </el-button>
              <el-button :icon="RefreshRight" :loading="geminiRestoring" @click="restoreLocalGemini">
                {{ geminiRestoring ? '恢复中' : '恢复 Gemini 配置' }}
              </el-button>
            </div>
          </article>

          <article class="wide-help">
            <strong>OpenCode</strong>
            <p>写入 <code>%USERPROFILE%\.config\opencode\opencode.json</code>，添加 OmniProxy、Gemini、OpenRouter、TokenRouter、Zo Computer 和自定义网关 provider。</p>
            <pre class="help-code"><code>OpenAI-compatible Router: http://127.0.0.1:{{ config.proxyPort }}/opencode-router/v1
Gemini Native: http://127.0.0.1:{{ config.proxyPort }}/gemini
OpenRouter: http://127.0.0.1:{{ config.proxyPort }}/openrouter/v1
TokenRouter: http://127.0.0.1:{{ config.proxyPort }}/tokenrouter/v1
Zo Computer: http://127.0.0.1:{{ config.proxyPort }}/zo/v1
Custom Gateway: http://127.0.0.1:{{ config.proxyPort }}/custom/v1</code></pre>
            <div class="help-actions">
              <el-button type="primary" :icon="MagicStick" :loading="opencodeConfiguring" @click="configureLocalOpenCode">
                {{ opencodeConfiguring ? '配置中' : '配置 OpenCode' }}
              </el-button>
              <el-button :icon="RefreshRight" :loading="opencodeRestoring" @click="restoreLocalOpenCode">
                {{ opencodeRestoring ? '恢复中' : '恢复 OpenCode 配置' }}
              </el-button>
            </div>
          </article>

          <article class="wide-help">
            <strong>Pi Coding Agent</strong>
            <p>写入 <code>%USERPROFILE%\.pi\agent\models.json</code>，添加 OmniProxy 和 Zo Computer provider，可通过 <code>pi --provider omniproxy --model gpt-5.4</code> 使用。</p>
            <pre class="help-code"><code>Pi Router: http://127.0.0.1:{{ config.proxyPort }}/pi-router/v1
Anthropic Router: http://127.0.0.1:{{ config.proxyPort }}/anthropic-router
Gemini Native: http://127.0.0.1:{{ config.proxyPort }}/gemini/v1beta
OpenRouter: http://127.0.0.1:{{ config.proxyPort }}/openrouter/v1
TokenRouter auto: http://127.0.0.1:{{ config.proxyPort }}/pi-router/v1 + model auto:balance
Zo Computer: http://127.0.0.1:{{ config.proxyPort }}/zo/v1
Custom Gateway: http://127.0.0.1:{{ config.proxyPort }}/custom/v1</code></pre>
            <div class="help-actions">
              <el-button type="primary" :icon="MagicStick" :loading="piConfiguring" @click="configureLocalPi">
                {{ piConfiguring ? '配置中' : '配置 Pi Coding Agent' }}
              </el-button>
              <el-button :icon="RefreshRight" :loading="piRestoring" @click="restoreLocalPi">
                {{ piRestoring ? '恢复中' : '恢复 Pi 配置' }}
              </el-button>
            </div>
          </article>

          <article class="wide-help">
            <strong>DeepSeek-TUI</strong>
            <p>写入 <code>%USERPROFILE%\.deepseek\config.toml</code>，使用 DeepSeek-TUI 内置 DeepSeek provider 连接 OmniProxy 的 DeepSeek 账号池。</p>
            <pre class="help-code"><code>provider = "deepseek"
default_text_model = "deepseek-v4-pro"
[providers.deepseek]
base_url = "http://127.0.0.1:{{ config.proxyPort }}/deepseek/v1"
api_key = "omniproxy-local"</code></pre>
            <div class="help-actions">
              <el-button type="primary" :icon="MagicStick" :loading="deepSeekTUIConfiguring" @click="configureLocalDeepSeekTUI">
                {{ deepSeekTUIConfiguring ? '配置中' : '配置 DeepSeek-TUI' }}
              </el-button>
              <el-button :icon="RefreshRight" :loading="deepSeekTUIRestoring" @click="restoreLocalDeepSeekTUI">
                {{ deepSeekTUIRestoring ? '恢复中' : '恢复 DeepSeek-TUI 配置' }}
              </el-button>
            </div>
          </article>
        </div>
      </section>

      <section v-else-if="activeTab === 'help'" key="help" class="help-page">
        <div class="help-guide">
          <div class="help-readiness-grid" aria-label="当前接入状态">
            <article
              v-for="card in helpReadinessCards"
              :key="card.label"
              :class="['help-readiness-card', card.state]"
            >
              <component :is="card.icon" class="help-card-icon" aria-hidden="true" />
              <div>
                <span>{{ card.label }}</span>
                <strong>{{ card.value }}</strong>
                <small>{{ card.detail }}</small>
              </div>
            </article>
          </div>

          <div class="help-section-block">
            <div class="help-section-title">
              <Lightning class="help-section-icon" aria-hidden="true" />
              <div>
                <strong>推荐工作流</strong>
                <p>从账号准备到请求诊断，按顺序检查更容易定位问题。</p>
              </div>
            </div>
            <div class="help-flow">
              <article v-for="item in helpWorkflowSteps" :key="item.step" class="help-flow-step">
                <span class="help-step-index">{{ item.step }}</span>
                <div>
                  <strong>{{ item.title }}</strong>
                  <p>{{ item.description }}</p>
                  <div class="help-step-actions">
                    <el-button
                      v-for="action in item.actions"
                      :key="action.label"
                      size="small"
                      text
                      type="primary"
                      @click="selectTab(action.tab)"
                    >
                      {{ action.label }}
                    </el-button>
                  </div>
                </div>
              </article>
            </div>
          </div>

          <div class="help-section-block">
            <div class="help-section-title">
              <Key class="help-section-icon" aria-hidden="true" />
              <div>
                <strong>账号类型怎么选</strong>
                <p>不同凭据会影响可用路由、额度展示和刷新方式。</p>
              </div>
            </div>
            <div class="help-credential-grid">
              <article v-for="group in helpCredentialGroups" :key="group.title">
                <strong>{{ group.title }}</strong>
                <code>{{ group.summary }}</code>
                <p>{{ group.detail }}</p>
              </article>
            </div>
          </div>

          <div class="help-section-block">
            <div class="help-section-title">
              <Monitor class="help-section-icon" aria-hidden="true" />
              <div>
                <strong>第三方客户端接口</strong>
                <p>Cherry Studio 这类客户端一般选择 OpenAI-compatible provider，填写 Base URL、任意非空 API Key 和模型名即可。</p>
              </div>
            </div>
            <div class="endpoint-reference">
              <div class="endpoint-reference-note">
                <strong>Cherry Studio 推荐</strong>
                <p>OpenAI 兼容：Base URL 填 <code>http://127.0.0.1:{{ proxyStatus.port || config.proxyPort }}/v1</code>；Zo Computer 填 <code>http://127.0.0.1:{{ proxyStatus.port || config.proxyPort }}/zo/v1</code>；API Key 填 <code>omniproxy-local</code>。</p>
              </div>
              <section v-for="group in thirdPartyEndpointGroups" :key="group.title" class="endpoint-group">
                <div class="endpoint-group-head">
                  <div>
                    <strong>{{ group.title }}</strong>
                    <p>{{ group.note }}</p>
                  </div>
                </div>
                <div class="endpoint-table">
                  <article v-for="endpoint in group.endpoints" :key="`${group.title}-${endpoint.name}`" class="endpoint-row">
                    <div class="endpoint-main">
                      <span class="tag muted">{{ endpoint.protocol }}</span>
                      <strong>{{ endpoint.name }}</strong>
                      <p>{{ endpoint.use }}</p>
                    </div>
                    <div class="endpoint-fields">
                      <div>
                        <span>Base URL</span>
                        <code>{{ endpoint.baseUrl }}</code>
                        <button type="button" class="ghost-button compact-button" @click="copyEndpointValue(endpoint.baseUrl, 'Base URL')">
                          复制
                        </button>
                      </div>
                      <div>
                        <span>API Key</span>
                        <code>{{ endpoint.apiKey }}</code>
                        <button type="button" class="ghost-button compact-button" @click="copyEndpointValue(endpoint.apiKey, 'API Key')">
                          复制
                        </button>
                      </div>
                      <div>
                        <span>模型</span>
                        <code>{{ endpoint.models }}</code>
                        <button type="button" class="ghost-button compact-button" @click="copyEndpointValue(endpoint.models, '模型')">
                          复制
                        </button>
                      </div>
                    </div>
                  </article>
                </div>
              </section>
            </div>
          </div>

          <div class="help-section-block">
            <div class="help-section-title">
              <RefreshRight class="help-section-icon" aria-hidden="true" />
              <div>
                <strong>常见排查路径</strong>
                <p>先确认请求是否进入本机代理，再判断账号、额度、模型和上游响应。</p>
              </div>
            </div>
            <div class="help-troubleshooting-list">
              <article v-for="item in helpTroubleshootingItems" :key="item.problem">
                <CircleCheckFilled class="help-check-icon" aria-hidden="true" />
                <div>
                  <strong>{{ item.problem }}</strong>
                  <p>{{ item.action }}</p>
                </div>
              </article>
            </div>
          </div>
        </div>
      </section>
      </Transition>

      <DiagnosticDrawer
        :entry="selectedHistoryEntry"
        :format-time="formatTime"
        :format-duration="formatDuration"
        :provider-label="providerLabel"
        @close="closeHistoryDiagnosis"
      />

      <Transition name="modal-pop" appear>
        <div v-if="firstUseGuideVisible" class="first-run-backdrop" @click.self="closeFirstUseGuide">
          <section class="first-run-dialog" role="dialog" aria-modal="true" aria-labelledby="first-run-title">
            <div class="first-run-head">
              <div>
                <span class="section-kicker">首次使用</span>
                <h2 id="first-run-title">3 步完成 OmniProxy 接入</h2>
                <p>这个向导只会自动显示一次；后续可以在“使用说明”和“一键配置”里查看同样的入口。</p>
              </div>
              <button type="button" class="ghost-button" @click="closeFirstUseGuide">跳过</button>
            </div>

            <div class="first-run-progress" aria-label="引导进度">
              <button
                v-for="(step, index) in firstUseGuideSteps"
                :key="step.step"
                type="button"
                :class="{ active: index === firstUseGuideStepIndex, done: index < firstUseGuideStepIndex }"
                @click="firstUseGuideStepIndex = index"
              >
                <span>{{ step.step }}</span>
                <strong>{{ step.title }}</strong>
              </button>
            </div>

            <div class="first-run-body">
              <div class="first-run-icon">
                <component :is="currentFirstUseGuideStep.icon" aria-hidden="true" />
              </div>
              <div>
                <span class="first-run-step">{{ currentFirstUseGuideStep.step }} / 03</span>
                <h3>{{ currentFirstUseGuideStep.title }}</h3>
                <p>{{ currentFirstUseGuideStep.description }}</p>
                <div class="first-run-endpoint">
                  <span>当前本地入口</span>
                  <code>{{ proxyEndpoint }}</code>
                </div>
              </div>
            </div>

            <div class="first-run-actions">
              <button type="button" class="ghost-button" @click="closeFirstUseGuide">跳过向导</button>
              <div>
                <button
                  type="button"
                  class="ghost-button"
                  :disabled="firstUseGuideStepIndex === 0"
                  @click="previousFirstUseGuideStep"
                >
                  上一步
                </button>
                <button type="button" class="ghost-button" @click="runFirstUseGuideAction">
                  {{ currentFirstUseGuideStep.actionLabel }}
                </button>
                <button type="button" class="primary-button" @click="nextFirstUseGuideStep">
                  {{ firstUseGuideStepIndex >= firstUseGuideSteps.length - 1 ? '完成' : '下一步' }}
                </button>
              </div>
            </div>
          </section>
        </div>
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
