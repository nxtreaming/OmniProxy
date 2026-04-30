<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { ElMessageBox } from 'element-plus'
import AboutView from './components/AboutView.vue'
import DiagnosticDrawer from './components/DiagnosticDrawer.vue'
import HistoryView from './components/HistoryView.vue'
import LogsView from './components/LogsView.vue'
import SettingsView from './components/SettingsView.vue'
import TokenEditorModal from './components/TokenEditorModal.vue'
import TokensView from './components/TokensView.vue'
import appIconUrl from './assets/appicon.png'
import { credentialTypes, providers, statusMeta, tabs } from './constants/app'
import { formatDuration, formatNumber, formatResetTime, formatTime, localDateKey } from './utils/format'
import {
  configureCodex,
  configureDeepSeekClaude,
  configureGemini,
  configureKimiClaude,
  configureMimoClaude,
  configureOpenCode,
  createToken,
  chooseDataDirectory as chooseDataDirectoryWithDialog,
  checkForUpdates,
  deleteToken,
  exportCodexAuthFiles,
  exportHistory,
  exportTokens,
  getAutoStartStatus,
  getAppInfo,
  getActiveRequests,
  getConfig,
  getDataDirectory,
  getHistory,
  getLogs,
  getProxyStatus,
  getTokens,
  importMimoCookieFromHAR,
  openExternalURL,
  saveConfig,
  setAutoStart,
  startProxy,
  stopProxy,
  updateToken,
  validateToken,
  restoreCodex,
  restoreGemini,
  restoreMimoClaude,
  restoreOpenCode,
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
  Refresh,
  RefreshRight,
  Setting,
  SwitchButton,
} from '@element-plus/icons-vue'

const activeTab = ref('dashboard')
const activeProvider = ref('openai')
const tabIcons = {
  dashboard: DataBoard,
  quotas: Coin,
  tokens: Key,
  history: Clock,
  logs: Memo,
  quickstart: MagicStick,
  settings: Setting,
  about: InfoFilled,
  help: HelpFilled,
}
const isDark = ref(false)
const loading = ref(false)
const codexConfiguring = ref(false)
const codexRestoring = ref(false)
const mimoClaudeConfiguring = ref(false)
const deepSeekClaudeConfiguring = ref(false)
const kimiClaudeConfiguring = ref(false)
const geminiConfiguring = ref(false)
const opencodeConfiguring = ref(false)
const mimoClaudeRestoring = ref(false)
const geminiRestoring = ref(false)
const opencodeRestoring = ref(false)
const mimoCookieImporting = ref(false)
const refreshingProvider = ref(false)
const dataDirChanging = ref(false)
const autoStartChanging = ref(false)
const autoStartEnabled = ref(false)
const updateChecking = ref(false)
const lastUpdateInfo = ref(null)
const lastUpdateCheckedAt = ref('')
const exportingHistory = ref('')
const exportingTokens = ref(false)
const exportingCodexAuth = ref(false)
const codexAuthImporting = ref(false)
const errorMessage = ref('')
const successMessage = ref('')
const toastAutoCloseMs = 4000
const skippedUpdateVersionKey = 'omniproxy.skippedUpdateVersion'
let toastTimer = null
let realtimeTimer = null
let updateCheckTimer = null
const validatingIds = reactive({})
const tokens = ref([])
const logs = ref([])
const requestHistory = ref([])
const activeRequests = ref([])
const selectedHistoryEntry = ref(null)
const proxyStatus = reactive({ running: false, port: 3000 })
const config = reactive({
  proxyPort: 3000,
  controlPort: 3890,
  schedulingMode: 'queue',
  websocketMode: 'enabled',
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
  customGatewayBaseUrl: '',
  customGatewayAnthropicBaseUrl: '',
  xiaomiBaseUrl: '',
  xiaomiApiBaseUrl: 'https://api.xiaomimimo.com/v1',
  xiaomiApiAnthropicBaseUrl: 'https://api.xiaomimimo.com/anthropic',
  xiaomiTokenPlanBaseUrl: 'https://token-plan-cn.xiaomimimo.com/v1',
  xiaomiTokenPlanAnthropicBaseUrl: 'https://token-plan-cn.xiaomimimo.com/anthropic',
  xiaomiPlatformCookie: '',
  xiaomiCredentialPriority: 'mimo_token_plan',
  codexBaseUrl: 'https://chatgpt.com/backend-api/codex',
  codexUsageEndpoint: 'https://chatgpt.com/backend-api/wham/usage',
  switchThreshold: 15,
  maxRetries: 2,
})
const dataDirectory = reactive({
  dataDir: '',
  bootstrapPath: '',
  envOverride: false,
  source: '',
  pendingDataDir: '',
  restartRequired: false,
})
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
const form = reactive({
  visible: false,
  editingId: '',
  name: '',
  provider: 'openai',
  originalProvider: 'openai',
  credentialType: 'api_key',
  originalCredentialType: 'api_key',
  tokenValue: '',
})
const activeTokens = computed(() => tokens.value.filter((item) => item.status === 'active'))
const lowTokens = computed(() => tokens.value.filter((item) => item.status === 'low'))
const exhaustedTokens = computed(() =>
  tokens.value.filter((item) => item.status === 'exhausted' && !isCooling(item)),
)
const invalidTokens = computed(() => tokens.value.filter((item) => item.status === 'invalid'))
const coolingTokens = computed(() => tokens.value.filter((item) => isCooling(item)))
const activeTokenIds = computed(() => new Set(activeRequests.value.map((item) => item.tokenId).filter(Boolean)))
const activeProviderInfo = computed(
  () => providers.find((item) => item.key === activeProvider.value) || providers[0],
)
const activeProviderTokens = computed(() => providerTokens(activeProvider.value))
const subscriptionOverviewTokens = computed(() => tokens.value.filter((item) => showQuotaWindows(item)))
const apiOverviewTokens = computed(() => tokens.value.filter((item) => !showQuotaWindows(item)))
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
const recentDailyUsageRows = computed(() => dailyUsageRows.value.slice(0, 14).reverse())
const usageTrendMax = computed(() =>
  Math.max(1, ...recentDailyUsageRows.value.map((row) => Number(row.totalTokens || 0))),
)
const requestTrendMax = computed(() =>
  Math.max(1, ...recentDailyUsageRows.value.map((row) => Number(row.requestCount || 0))),
)
const trendGridColumns = computed(
  () => `repeat(${Math.max(1, recentDailyUsageRows.value.length)}, minmax(0, 1fr))`,
)
const toolUsageRows = computed(() => buildToolUsageRows(activeRequests.value, requestHistory.value))
const isCodexForm = computed(
  () => form.provider === 'openai' && form.credentialType === 'codex_auth_json',
)

onMounted(async () => {
  if (window.matchMedia?.('(prefers-color-scheme: dark)').matches) {
    isDark.value = true
  }
  await refreshAll()
  updateCheckTimer = window.setTimeout(() => checkForAvailableUpdate(), 2500)
  realtimeTimer = window.setInterval(refreshRealtime, 3000)
})

onBeforeUnmount(() => {
  if (realtimeTimer) {
    window.clearInterval(realtimeTimer)
    realtimeTimer = null
  }
  if (updateCheckTimer) {
    window.clearTimeout(updateCheckTimer)
    updateCheckTimer = null
  }
  if (toastTimer) {
    window.clearTimeout(toastTimer)
    toastTimer = null
  }
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

watch(activeTab, (tab) => {
  if (tab === 'history') {
    refreshHistory()
  }
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

    const releaseURL = update.downloadUrl || update.releaseUrl
    if (!releaseURL) {
      if (manual) {
        errorMessage.value = `发现新版本 ${update.latestVersion}，但没有可用下载地址`
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
    openExternalURL(releaseURL)
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

async function refreshHistory(filters = {}) {
  try {
    requestHistory.value = await getHistory(filters)
  } catch (error) {
    errorMessage.value = error.message
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
    tokenValue: '',
  })
}

function closeForm() {
  form.visible = false
}

async function submitForm() {
  errorMessage.value = ''
  successMessage.value = ''
  const name = isCodexForm.value ? '' : form.name.trim()
  const tokenValue = form.tokenValue.trim()
  const provider = form.provider.trim() || 'openai'
  const credentialType = normalizedCredentialType(provider, form.credentialType)
  const isEditing = Boolean(form.editingId)
  const replacingCredential = tokenValue !== ''

  if (!isCodexForm.value && !name) {
    errorMessage.value = '账号名称不能为空'
    return
  }
  const duplicate = tokens.value.some(
    (item) =>
      !isCodexForm.value &&
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
  if (credentialType === 'codex_auth_json' && (!isEditing || replacingCredential)) {
    try {
      JSON.parse(tokenValue)
    } catch {
      errorMessage.value = 'Codex auth.json 内容不是有效 JSON'
      return
    }
  } else if (replacingCredential && provider === 'xiaomi' && credentialType === 'mimo_token_plan' && !tokenValue.startsWith('tp-')) {
    errorMessage.value = 'MiMo Token Plan Key 必须以 tp- 开头'
    return
  } else if (replacingCredential && provider === 'xiaomi' && credentialType === 'api_key' && !tokenValue.startsWith('sk-')) {
    errorMessage.value = 'MiMo 按量 API Key 必须以 sk- 开头'
    return
  } else if ((!isEditing || replacingCredential) && tokenValue.length < 12) {
    errorMessage.value = 'Token 长度过短'
    return
  }

  const payload = {
    name,
    provider,
    credentialType,
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
    successMessage.value = form.editingId ? '账号已更新' : '账号已添加'
  } catch (error) {
    errorMessage.value = error.message
  }
}

async function removeToken(token) {
  if (!window.confirm(`删除账号「${token.name}」？`)) {
    return
  }
  try {
    await deleteToken(token.id)
    await refreshAll()
    successMessage.value = '账号已删除'
  } catch (error) {
    errorMessage.value = error.message
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
      successMessage.value = `验证通过：${result.status}，耗时 ${result.durationMs}ms`
    } else {
      errorMessage.value = `验证未通过：${result.status || '-'} ${result.message || ''}`
    }
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    validatingIds[token.id] = false
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

  const idToken = data?.tokens?.id_token
  if (typeof idToken !== 'string' || !idToken.trim()) {
    throw new Error('缺少 tokens.id_token')
  }
  const parts = idToken.split('.')
  if (parts.length !== 3) {
    throw new Error('tokens.id_token 格式不正确')
  }

  let payload
  try {
    payload = JSON.parse(decodeBase64URL(parts[1]))
  } catch {
    throw new Error('无法解析 tokens.id_token')
  }

  const email = payload?.['https://api.openai.com/profile']?.email || payload?.email
  if (typeof email !== 'string' || !email.trim()) {
    throw new Error('tokens.id_token 中没有邮箱')
  }
  return email.trim()
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
      customGatewayBaseUrl: config.customGatewayBaseUrl.trim(),
      customGatewayAnthropicBaseUrl: config.customGatewayAnthropicBaseUrl.trim(),
      xiaomiBaseUrl: config.xiaomiBaseUrl.trim(),
      xiaomiApiBaseUrl: config.xiaomiApiBaseUrl.trim(),
      xiaomiApiAnthropicBaseUrl: config.xiaomiApiAnthropicBaseUrl.trim(),
      xiaomiTokenPlanBaseUrl: config.xiaomiTokenPlanBaseUrl.trim(),
      xiaomiTokenPlanAnthropicBaseUrl: config.xiaomiTokenPlanAnthropicBaseUrl.trim(),
      xiaomiPlatformCookie: config.xiaomiPlatformCookie.trim(),
      xiaomiCredentialPriority: config.xiaomiCredentialPriority,
      codexBaseUrl: config.codexBaseUrl.trim(),
      codexUsageEndpoint: config.codexUsageEndpoint.trim(),
      switchThreshold: Number(config.switchThreshold),
      maxRetries: Number(config.maxRetries),
    })
    Object.assign(config, saved)
    await refreshRealtime()
    successMessage.value = '设置已保存'
  } catch (error) {
    errorMessage.value = error.message
  }
}

async function importMimoCookie() {
  errorMessage.value = ''
  successMessage.value = ''
  mimoCookieImporting.value = true
  try {
    const result = await importMimoCookieFromHAR()
    const loadedConfig = await getConfig()
    Object.assign(config, loadedConfig)
    successMessage.value = `${result.message || 'MiMo Cookie 已导入'}，长度 ${result.length || 0}`
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    mimoCookieImporting.value = false
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
  return item.hasTokenValue ? '已保存' : '-'
}

function credentialPlaceholder() {
  if (form.editingId) {
    return '留空表示保留当前凭据'
  }
  if (form.credentialType === 'codex_auth_json') {
    return '粘贴 ~/.codex/auth.json 的完整 JSON 内容'
  }
  if (form.credentialType === 'mimo_token_plan') {
    return '粘贴 tp- 开头的 MiMo Token Plan Key'
  }
  if (form.provider === 'xiaomi') {
    return '粘贴 sk- 开头的 MiMo 按量 API Key'
  }
  if (form.provider === 'kimi') {
    return '粘贴 Kimi Code API Key'
  }
  if (form.provider === 'zhipu') {
    return '粘贴 Zhipu GLM API Key'
  }
  if (form.provider === 'minimax') {
    return '粘贴 MiniMax API Key'
  }
  if (form.provider === 'gemini') {
    return '粘贴 Google Gemini API Key'
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
  return credentialTypes[item.credentialType || 'api_key'] || item.credentialType || 'API Key'
}

function normalizedCredentialType(provider, credentialType) {
  if (provider === 'openai') {
    return credentialType === 'codex_auth_json' ? 'codex_auth_json' : 'api_key'
  }
  if (provider === 'xiaomi') {
    return credentialType === 'mimo_token_plan' ? 'mimo_token_plan' : 'api_key'
  }
  return 'api_key'
}

function onProviderChange() {
  form.credentialType = normalizedCredentialType(form.provider, form.credentialType)
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
  { key: 'opencode', label: 'OpenCode' },
  { key: 'gemini', label: 'Gemini CLI' },
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

function hasBalanceUsage(item) {
  return Boolean(item.usage?.balanceUnit)
}

function quotaDisplay(item) {
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

function isMimoTokenPlan(item) {
  return item?.provider === 'xiaomi' && item?.credentialType === 'mimo_token_plan'
}

function showQuotaWindows(item) {
  return isCodexToken(item) || isMimoTokenPlan(item) || Boolean(item?.usage?.subscriptionQuotaAvailable)
}

function quotaPrimaryLabel(item) {
  return isMimoTokenPlan(item) ? '本月额度' : '5h额度'
}

function quotaSecondaryLabel(item) {
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

function tokenUsageSummary(item) {
  const total = Number(item.stats?.totalTokens || 0)
  const input = Number(item.stats?.inputTokens || 0)
  const output = Number(item.stats?.outputTokens || 0)
  const requests = Number(item.stats?.requestCount || 0)
  if (total > 0) {
    return `Token ${formatNumber(total)} · 入 ${formatNumber(input)} · 出 ${formatNumber(output)}`
  }
  return requests > 0 ? 'Token 未上报' : 'Token 0'
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
  return Array.from(byDate.values()).sort((a, b) => b.date.localeCompare(a.date)).slice(0, 30)
}

function isCooling(item) {
  return item?.cooldownUntil && new Date(item.cooldownUntil).getTime() > Date.now()
}

function displayStatusLabel(item) {
  if (isCooling(item)) return '冷却中'
  return statusLabel(item.status)
}

function displayStatusClass(item) {
  if (isCooling(item)) return 'warning'
  return statusClass(item.status)
}

function displayStatusType(item) {
  if (isCooling(item)) return 'warning'
  return statusType(item.status)
}

function healthSummary(item) {
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

function closeHistoryDiagnosis() {
  selectedHistoryEntry.value = null
}

async function refreshProviderQuotas() {
  const items = activeProviderTokens.value
  if (!items.length) {
    successMessage.value = `暂无 ${activeProviderInfo.value.label} 账号可刷新`
    return
  }

  errorMessage.value = ''
  successMessage.value = ''
  refreshingProvider.value = true
  let failed = 0
  try {
    for (const item of items) {
      validatingIds[item.id] = true
      try {
        await validateToken(item.id)
      } catch {
        failed += 1
      } finally {
        validatingIds[item.id] = false
      }
    }
    await refreshRealtime()
    if (failed) {
      errorMessage.value = `已刷新 ${items.length - failed} 个账号，${failed} 个失败`
    } else {
      successMessage.value = `已刷新 ${items.length} 个 ${activeProviderInfo.value.label} 账号`
    }
  } finally {
    refreshingProvider.value = false
  }
}

async function refreshQuota(item) {
  await verifyToken(item)
}
</script>

<template>
  <div class="shell" :class="{ dark: isDark }">
    <aside class="sidebar">
      <div class="brand">
        <div class="brand-mark">
          <img :src="appIconUrl" alt="" />
        </div>
        <div>
          <strong>OmniProxy</strong>
          <span>AI 令牌调度网关</span>
        </div>
      </div>

      <nav class="nav-list">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          type="button"
          :class="{ active: activeTab === tab.key }"
          @click="activeTab = tab.key"
        >
          <component :is="tabIcons[tab.key]" class="nav-icon" aria-hidden="true" />
          <span>{{ tab.label }}</span>
        </button>
      </nav>

      <div class="sidebar-tools">
        <button type="button" class="ghost-button" @click="isDark = !isDark">
          {{ isDark ? '浅色模式' : '深色模式' }}
        </button>
      </div>
    </aside>

    <main class="workspace">
      <header class="topbar">
        <div>
          <p class="eyebrow">本地控制台</p>
          <h1>{{ tabs.find((tab) => tab.key === activeTab)?.label }}</h1>
        </div>
        <div class="topbar-actions">
          <el-tag :type="proxyStatus.running ? 'success' : 'info'" effect="light" round>
            代理 {{ proxyStatus.running ? '运行中' : '已停止' }} · :{{ proxyStatus.port }}
          </el-tag>
          <el-button type="primary" :icon="SwitchButton" @click="toggleProxy">
            {{ proxyStatus.running ? '停止代理' : '启动代理' }}
          </el-button>
        </div>
      </header>

      <div v-if="errorMessage || successMessage" class="toast-stack" aria-live="polite">
        <div v-if="errorMessage" class="alert" role="alert">
          <span class="toast-message">{{ errorMessage }}</span>
          <button type="button" aria-label="关闭错误提示" @click="errorMessage = ''">×</button>
        </div>
        <div v-if="successMessage" class="notice" role="status">
          <span class="toast-message">{{ successMessage }}</span>
          <button type="button" aria-label="关闭成功提示" @click="successMessage = ''">×</button>
        </div>
      </div>

      <Transition name="page-switch" mode="out-in" appear>
      <section v-if="activeTab === 'dashboard'" key="dashboard" class="view-grid">
        <article class="metric-card account-status-card">
          <span>账号状态</span>
          <div class="account-status-metrics">
            <div>
              <strong>{{ activeTokens.length }}</strong>
              <small>正常账号</small>
            </div>
            <div>
              <strong>{{ invalidTokens.length }}</strong>
              <small>无效账号</small>
            </div>
          </div>
          <small>低额度 {{ lowTokens.length }} · 冷却 {{ coolingTokens.length }} · 耗尽 {{ exhaustedTokens.length }}</small>
        </article>
        <article class="metric-card">
          <span>代理总 Token</span>
          <strong>{{ formatNumber(totalProxyTokens) }}</strong>
          <small>输入 {{ formatNumber(totalProxyInputTokens) }} · 输出 {{ formatNumber(totalProxyOutputTokens) }}</small>
        </article>
        <article class="metric-card">
          <span>今日 Token</span>
          <strong>{{ formatNumber(todayProxyTokens) }}</strong>
          <small>累计请求 {{ formatNumber(totalProxyRequests) }} 次</small>
        </article>

        <section class="panel full tool-usage-panel">
          <div class="section-heading">
            <div>
              <h2>编程工具账号占用</h2>
              <p>按 Codex、Claude Code、OpenCode、Gemini CLI 等工具区分正在使用的账号</p>
            </div>
          </div>
          <div v-if="toolUsageRows.length" class="tool-usage-grid">
            <div
              v-for="row in toolUsageRows"
              :key="row.clientKey"
              :class="['tool-usage-row', { active: row.active }]"
            >
              <div>
                <strong>{{ row.clientName || clientToolLabel(row.clientKey) }}</strong>
                <small>{{ toolUsageMeta(row) }}</small>
              </div>
              <div>
                <span :class="['tag', row.active ? 'success' : 'muted']">
                  {{ row.active ? `使用中 ${row.activeCount}` : '最近使用' }}
                </span>
                <small v-if="toolUsageDuration(row)">{{ toolUsageDuration(row) }}</small>
              </div>
              <div class="tool-account-cell" :title="row.tokenText">
                <span>账号</span>
                <strong>{{ row.tokenText }}</strong>
              </div>
            </div>
          </div>
          <div v-else class="empty">暂无工具使用记录</div>
        </section>

        <section class="panel wide">
          <div class="section-heading">
            <div>
              <h2>额度概览</h2>
              <p>订阅额度和 API / 余额状态分开展示</p>
            </div>
            <button type="button" class="ghost-button" @click="refreshAll">刷新</button>
          </div>
          <div class="quota-overview-grid">
            <section class="quota-overview-block">
              <div class="quota-overview-head">
                <strong>订阅额度</strong>
                <small>Codex / Token Plan</small>
              </div>
              <div class="quota-list compact-quota-list">
                <div
                  v-for="item in subscriptionOverviewTokens"
                  :key="item.id"
                  :class="['quota-row', 'subscription-quota-row', { 'current-quota-row': isTokenActiveNow(item) }]"
                  :aria-current="isTokenActiveNow(item) ? 'true' : undefined"
                >
                  <div class="quota-account">
                    <div class="quota-account-title">
                      <strong>{{ item.name }}</strong>
                      <span v-if="isTokenActiveNow(item)" class="current-usage-badge">正在使用</span>
                      <span :class="['tag', displayStatusClass(item)]">{{ displayStatusLabel(item) }}</span>
                    </div>
                    <small class="current-usage-meta">
                      {{ providerLabel(item.provider) }} · {{ quotaPrimaryLabel(item) }}
                    </small>
                  </div>
                  <div class="progress">
                    <span :style="{ width: `${quotaPercentValue(item, 'primaryRemainingPercent')}%` }"></span>
                  </div>
                  <small class="quota-percent">
                    {{ quotaPercentText(item, 'primaryRemainingPercent') }}
                  </small>
                </div>
                <div v-if="!subscriptionOverviewTokens.length" class="empty">暂无订阅额度账号</div>
              </div>
            </section>

            <section class="quota-overview-block">
              <div class="quota-overview-head">
                <strong>API / 余额状态</strong>
                <small>API Key 不按百分比展示</small>
              </div>
              <div class="quota-list compact-quota-list">
                <div
                  v-for="item in apiOverviewTokens"
                  :key="item.id"
                  :class="['quota-row', 'api-quota-row', { 'current-quota-row': isTokenActiveNow(item) }]"
                  :aria-current="isTokenActiveNow(item) ? 'true' : undefined"
                >
                  <div class="quota-account">
                    <div class="quota-account-title">
                      <strong>{{ item.name }}</strong>
                      <span v-if="isTokenActiveNow(item)" class="current-usage-badge">正在使用</span>
                      <span :class="['tag', displayStatusClass(item)]">{{ displayStatusLabel(item) }}</span>
                    </div>
                    <small class="current-usage-meta">{{ providerLabel(item.provider) }} · {{ credentialLabel(item) }}</small>
                  </div>
                  <div class="api-quota-value">
                    <strong>{{ apiQuotaDisplay(item) }}</strong>
                    <small>{{ apiQuotaMeta(item) }}</small>
                  </div>
                </div>
                <div v-if="!apiOverviewTokens.length" class="empty">暂无 API Key 账号</div>
              </div>
            </section>
          </div>
        </section>

        <section class="panel">
          <div class="section-heading">
            <div>
              <h2>最近日志</h2>
              <p>最新代理转发记录</p>
            </div>
          </div>
          <div class="log-list compact">
            <div v-for="entry in logs.slice(0, 6)" :key="entry.id" class="log-row">
              <span :class="['dot', entry.level]"></span>
              <p>
                <span v-if="entry.clientName" class="log-inline-model">{{ entry.clientName }}</span>
                <span v-if="entry.model" class="log-inline-model">{{ entry.model }}</span>
                {{ entry.message }}
              </p>
              <small>{{ formatTime(entry.time) }}</small>
            </div>
            <div v-if="!logs.length" class="empty">暂无日志</div>
          </div>
        </section>

        <section class="panel full">
          <div class="section-heading">
            <div>
              <h2>分天用量统计</h2>
              <p>Token 数来自上游 usage；请求数按成功通过代理返回的请求统计</p>
            </div>
          </div>
          <div v-if="recentDailyUsageRows.length" class="trend-panels">
            <div class="trend-panel" aria-label="最近 Token 趋势">
              <div class="trend-panel-head">
                <span>Token 趋势</span>
                <strong>{{ formatNumber(totalProxyTokens) }}</strong>
              </div>
              <div class="usage-trend" :style="{ gridTemplateColumns: trendGridColumns }">
                <div
                  v-for="row in recentDailyUsageRows"
                  :key="row.date"
                  class="trend-column"
                  :title="`${row.date} · ${formatNumber(row.totalTokens)} Token`"
                >
                  <div class="trend-bar">
                    <span :style="{ height: trendHeight(row) }"></span>
                  </div>
                  <small>{{ row.date.slice(5) }}</small>
                </div>
              </div>
            </div>
            <div class="trend-panel" aria-label="最近请求次数趋势">
              <div class="trend-panel-head">
                <span>请求次数趋势</span>
                <strong>{{ formatNumber(totalProxyRequests) }}</strong>
              </div>
              <div class="usage-trend request-trend" :style="{ gridTemplateColumns: trendGridColumns }">
                <div
                  v-for="row in recentDailyUsageRows"
                  :key="`requests-${row.date}`"
                  class="trend-column"
                  :title="`${row.date} · ${formatNumber(row.requestCount)} 次请求`"
                >
                  <div class="trend-bar">
                    <span :style="{ height: requestTrendHeight(row) }"></span>
                  </div>
                  <small>{{ row.date.slice(5) }}</small>
                </div>
              </div>
            </div>
          </div>
          <div class="usage-table">
            <div class="usage-row header">
              <span>日期</span>
              <span>总 Token</span>
              <span>输入</span>
              <span>输出</span>
              <span>请求</span>
            </div>
            <div v-for="row in dailyUsageRows" :key="row.date" class="usage-row">
              <span>{{ row.date }}</span>
              <strong>{{ formatNumber(row.totalTokens) }}</strong>
              <span>{{ formatNumber(row.inputTokens) }}</span>
              <span>{{ formatNumber(row.outputTokens) }}</span>
              <span>{{ formatNumber(row.requestCount) }}</span>
            </div>
            <div v-if="!dailyUsageRows.length" class="empty">暂无代理 Token 用量</div>
          </div>
        </section>
      </section>

      <section v-else-if="activeTab === 'quotas'" key="quotas" class="panel">
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
            <p>{{ activeProviderTokens.length }} 个账号 · {{ activeProviderInfo.note }}</p>
          </div>
        </div>

        <div class="quota-card-grid">
          <el-card
            v-for="item in activeProviderTokens"
            :key="item.id"
            class="quota-card"
            shadow="hover"
            :body-style="{ padding: '0' }"
          >
            <div class="quota-card-inner">
              <div class="quota-card-head">
                <div>
                  <strong class="account-name">{{ item.name }}</strong>
                  <span>{{ isCodexToken(item) ? 'Codex auth.json' : credentialLabel(item) }} · {{ providerLabel(item.provider) }}</span>
                  <small class="health-line">{{ healthSummary(item) }}</small>
                </div>
                <div class="quota-head-actions">
                  <el-tag v-if="item.usage?.subscriptionQuotaAvailable && item.usage?.planType" type="primary" effect="plain">
                    {{ planLabel(item.usage?.planType) }}
                  </el-tag>
                  <el-tag :type="displayStatusType(item)" effect="light" class="status-tag">{{ displayStatusLabel(item) }}</el-tag>
                  <el-tooltip content="刷新额度" placement="top">
                    <el-button
                      circle
                      :icon="Refresh"
                      :loading="validatingIds[item.id]"
                      @click="refreshQuota(item)"
                    />
                  </el-tooltip>
                </div>
              </div>

              <div :class="['quota-layout', { 'codex-layout': isCodexToken(item) }]">
                <div v-if="showQuotaWindows(item)" class="quota-limit">
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

                <div v-if="showQuotaWindows(item)" class="quota-limit">
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

                <div v-if="!isCodexToken(item)" class="quota-stat">
                  <span>{{ quotaStatLabel(item) }}</span>
                  <strong>{{ hasBalanceUsage(item) ? quotaDisplay(item) : `${item.usage?.apiRemaining || item.remaining}%` }}</strong>
                  <small>{{ quotaStatMeta(item) }}</small>
                </div>

                <div class="quota-stat">
                  <span>代理请求</span>
                  <strong>{{ formatNumber(item.stats?.requestCount) }} 次</strong>
                  <small class="quota-detail token-usage-detail">{{ tokenUsageSummary(item) }}</small>
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
        :exporting-tokens="exportingTokens"
        :exporting-codex-auth="exportingCodexAuth"
        :codex-auth-importing="codexAuthImporting"
        :validating-ids="validatingIds"
        :provider-tokens="providerTokens"
        :credential-label="credentialLabel"
        :credential-display="credentialDisplay"
        :display-status-type="displayStatusType"
        :display-status-label="displayStatusLabel"
        :health-summary="healthSummary"
        :format-time="formatTime"
        :format-number="formatNumber"
        :quota-display="quotaDisplay"
        @select-provider="selectProvider"
        @export-token-backup="exportTokenBackup"
        @open-codex-auth-file-picker="openCodexAuthFilePicker"
        @import-codex-auth-files="importCodexAuthFiles"
        @export-codex-auth-backups="exportCodexAuthBackups"
        @open-create-form="openCreateForm"
        @verify-token="verifyToken"
        @open-edit-form="openEditForm"
        @remove-token="removeToken"
      />

      <HistoryView
        v-else-if="activeTab === 'history'"
        key="history"
        :entries="requestHistory"
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
        :mimo-cookie-importing="mimoCookieImporting"
        @persist-config="persistConfig"
        @choose-data-directory="chooseDataDirectory"
        @toggle-auto-start="toggleAutoStart"
        @import-mimo-cookie="importMimoCookie"
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
        :update-checked-at="lastUpdateCheckedAt"
        :format-time="formatTime"
        @manual-check-for-updates="manualCheckForUpdates"
        @open-url="openExternalURL"
      />

      <section v-else-if="activeTab === 'quickstart'" key="quickstart" class="panel help-panel">
        <div class="section-heading">
          <div>
            <h2>一键配置</h2>
            <p>把本机 Codex、Claude Code、Gemini CLI 或 OpenCode 指向 OmniProxy，本页只负责写入本地客户端配置</p>
          </div>
        </div>

        <div class="help-grid">
          <article class="wide-help">
            <strong>Codex</strong>
            <p>本地 Codex 会写入 <code>%USERPROFILE%\.codex\config.toml</code>，使用账号池里的 OpenAI Codex auth.json。</p>
            <pre class="help-code"><code>OpenAI Codex Base URL: http://127.0.0.1:{{ config.proxyPort }}/backend-api/codex</code></pre>
            <div class="help-actions">
              <el-button type="primary" :icon="MagicStick" :loading="codexConfiguring" @click="configureLocalCodex">
                {{ codexConfiguring ? '配置中' : '配置 Codex OpenAI' }}
              </el-button>
              <el-button :icon="RefreshRight" :loading="codexRestoring" @click="restoreLocalCodex">
                {{ codexRestoring ? '恢复中' : '恢复 Codex 配置' }}
              </el-button>
            </div>
          </article>

          <article class="wide-help">
            <strong>Claude Code</strong>
            <p>每次只接入一个 Claude Code 上游。DeepSeek 和 MiMo 会分别写入两个模型槽位，并清理 OmniProxy 旧的多模型列表配置。</p>
            <pre class="help-code"><code>Claude Router URL: http://127.0.0.1:{{ config.proxyPort }}/anthropic-router
DeepSeek: deepseek-v4-pro[1m] / deepseek-v4-flash
MiMo: MiMo-V2.5-Pro / MiMo-V2.5
Kimi model: kimi-for-coding</code></pre>
            <div class="help-actions">
              <el-button type="primary" :icon="MagicStick" :loading="deepSeekClaudeConfiguring" @click="configureLocalDeepSeekClaude">
                {{ deepSeekClaudeConfiguring ? '配置中' : '接入 Claude DeepSeek' }}
              </el-button>
              <el-button type="primary" plain :icon="MagicStick" :loading="mimoClaudeConfiguring" @click="configureLocalMimoClaude">
                {{ mimoClaudeConfiguring ? '配置中' : '接入 Claude MiMo' }}
              </el-button>
              <el-button type="primary" plain :icon="MagicStick" :loading="kimiClaudeConfiguring" @click="configureLocalKimiClaude">
                {{ kimiClaudeConfiguring ? '配置中' : '接入 Claude Kimi' }}
              </el-button>
              <el-button :icon="RefreshRight" :loading="mimoClaudeRestoring" @click="restoreLocalMimoClaude">
                {{ mimoClaudeRestoring ? '恢复中' : '恢复 Claude 配置' }}
              </el-button>
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
            <p>写入 <code>%USERPROFILE%\.config\opencode\opencode.json</code>，添加 OmniProxy、Gemini 和自定义网关三个 provider。</p>
            <pre class="help-code"><code>OpenAI-compatible Router: http://127.0.0.1:{{ config.proxyPort }}/opencode-router/v1
Gemini Native: http://127.0.0.1:{{ config.proxyPort }}/gemini
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
        </div>
      </section>

      <section v-else-if="activeTab === 'help'" key="help" class="panel help-panel">
        <div class="section-heading">
          <div>
            <h2>使用说明</h2>
            <p>按厂商维护账号，启动本地代理后在客户端里使用代理地址</p>
          </div>
        </div>

        <div class="help-grid">
          <article>
            <strong>1. 添加账号</strong>
            <p>进入账号管理，先在顶部选择厂商，再添加对应账号。OpenAI 支持 API Key 和 Codex auth.json，Codex 会自动从 id_token 解析邮箱作为账号名。</p>
          </article>
          <article>
            <strong>2. 查看额度</strong>
            <p>进入额度页面，选择厂商后查看每个账号的状态。普通 API Key 显示余额或 API 剩余额度；Codex 和 Token Plan 显示对应订阅额度窗口。</p>
          </article>
          <article>
            <strong>3. 启动代理</strong>
            <p>确认代理设置里的端口和各厂商 Base URL 后，点击右上角启动代理。客户端请求走本地代理端口，由程序按账号状态自动调度。</p>
          </article>
          <article>
            <strong>4. 账号调度模式</strong>
            <p>队列模式按账号列表顺序优先使用前面的可用账号；优先平衡使用会优先选择并发更少、剩余额度更高、最近更少使用的账号。</p>
          </article>
          <article>
            <strong>5. 排查问题</strong>
            <p>请求失败时先看实时日志，再在账号管理里验证对应账号。额度过低或账号无效时，程序会按阈值跳过不可用账号。</p>
          </article>
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

      <TokenEditorModal
        v-if="form.visible"
        :form="form"
        :providers="providers"
        :is-codex-form="isCodexForm"
        :placeholder="credentialPlaceholder()"
        @close="closeForm"
        @submit="submitForm"
        @provider-change="onProviderChange"
      />

      <div v-if="loading" class="loading">加载中...</div>
    </main>
  </div>
</template>
