<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ChooseDataDirectory, DataDirectory } from '../wailsjs/go/main/DesktopApp'
import {
  configureCodex,
  configureDeepSeekClaude,
  configureKimiClaude,
  configureMimoClaude,
  createToken,
  deleteToken,
  getConfig,
  getLogs,
  getProxyStatus,
  getTokens,
  saveConfig,
  startProxy,
  stopProxy,
  updateToken,
  validateToken,
  restoreCodex,
  restoreMimoClaude,
} from './services/api'
import {
  Connection,
  MagicStick,
  Refresh,
  RefreshRight,
  SwitchButton,
} from '@element-plus/icons-vue'

const tabs = [
  { key: 'dashboard', label: '仪表盘' },
  { key: 'quotas', label: '额度' },
  { key: 'tokens', label: '账号管理' },
  { key: 'logs', label: '实时日志' },
  { key: 'quickstart', label: '一键配置' },
  { key: 'settings', label: '全局设置' },
  { key: 'help', label: '使用说明' },
]

const providers = [
  { key: 'openai', label: 'OpenAI', note: '支持 API Key 和 Codex auth.json' },
  { key: 'anthropic', label: 'Anthropic', note: 'API Key' },
  { key: 'deepseek', label: 'DeepSeek', note: 'API Key' },
  { key: 'kimi', label: 'Kimi Code', note: 'API Key，Claude Code 模型 kimi-for-coding' },
  { key: 'xiaomi', label: 'Xiaomi MiMo', note: '按量 API Key 或 Token Plan' },
]

const credentialTypes = {
  api_key: 'API Key',
  codex_auth_json: 'Codex auth.json',
  mimo_token_plan: 'MiMo Token Plan',
}

const activeTab = ref('dashboard')
const activeProvider = ref('openai')
const isDark = ref(false)
const loading = ref(false)
const codexConfiguring = ref(false)
const codexRestoring = ref(false)
const mimoClaudeConfiguring = ref(false)
const deepSeekClaudeConfiguring = ref(false)
const kimiClaudeConfiguring = ref(false)
const mimoClaudeRestoring = ref(false)
const refreshingProvider = ref(false)
const dataDirChanging = ref(false)
const errorMessage = ref('')
const successMessage = ref('')
const toastAutoCloseMs = 4000
let toastTimer = null
const validatingIds = reactive({})
const tokens = ref([])
const logs = ref([])
const proxyStatus = reactive({ running: false, port: 3000 })
const config = reactive({
  proxyPort: 3000,
  controlPort: 3890,
  upstreamBaseUrl: 'https://api.openai.com',
  openaiBaseUrl: 'https://api.openai.com',
  anthropicBaseUrl: 'https://api.anthropic.com',
  deepseekBaseUrl: 'https://api.deepseek.com',
  deepseekAnthropicBaseUrl: 'https://api.deepseek.com/anthropic',
  kimiBaseUrl: 'https://api.kimi.com/coding',
  xiaomiBaseUrl: '',
  xiaomiApiBaseUrl: 'https://api.xiaomimimo.com/v1',
  xiaomiApiAnthropicBaseUrl: 'https://api.xiaomimimo.com/anthropic',
  xiaomiTokenPlanBaseUrl: 'https://token-plan-cn.xiaomimimo.com/v1',
  xiaomiTokenPlanAnthropicBaseUrl: 'https://token-plan-cn.xiaomimimo.com/anthropic',
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
const form = reactive({
  visible: false,
  editingId: '',
  name: '',
  provider: 'openai',
  credentialType: 'api_key',
  tokenValue: '',
})

const activeTokens = computed(() => tokens.value.filter((item) => item.status === 'active'))
const lowTokens = computed(() => tokens.value.filter((item) => item.status === 'low'))
const exhaustedTokens = computed(() => tokens.value.filter((item) => item.status === 'exhausted'))
const invalidTokens = computed(() => tokens.value.filter((item) => item.status === 'invalid'))
const currentToken = computed(() => {
  const usable = [...activeTokens.value, ...lowTokens.value]
  const lastUsed = usable
    .filter((item) => item.lastUsedAt)
    .sort((a, b) => new Date(b.lastUsedAt).getTime() - new Date(a.lastUsedAt).getTime())[0]
  return lastUsed || usable[0] || null
})
const activeProviderInfo = computed(
  () => providers.find((item) => item.key === activeProvider.value) || providers[0],
)
const activeProviderTokens = computed(() => providerTokens(activeProvider.value))
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
const isCodexForm = computed(
  () => form.provider === 'openai' && form.credentialType === 'codex_auth_json',
)

const statusMeta = {
  active: { label: '正常', className: 'success' },
  low: { label: '低额度', className: 'warning' },
  exhausted: { label: '耗尽', className: 'muted' },
  invalid: { label: '无效', className: 'danger' },
}

onMounted(async () => {
  if (window.matchMedia?.('(prefers-color-scheme: dark)').matches) {
    isDark.value = true
  }
  await refreshAll()
  window.setInterval(refreshRealtime, 3000)
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

async function refreshAll() {
  loading.value = true
  errorMessage.value = ''
  try {
    const [loadedTokens, loadedConfig, loadedLogs, loadedStatus, loadedDataDirectory] = await Promise.all([
      getTokens(),
      getConfig(),
      getLogs(),
      getProxyStatus(),
      DataDirectory(),
    ])
    tokens.value = loadedTokens
    logs.value = loadedLogs
    Object.assign(config, loadedConfig)
    Object.assign(proxyStatus, loadedStatus)
    Object.assign(dataDirectory, loadedDataDirectory, {
      pendingDataDir: '',
      restartRequired: false,
    })
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    loading.value = false
  }
}

async function refreshRealtime() {
  try {
    const [loadedLogs, loadedStatus, loadedTokens] = await Promise.all([
      getLogs(),
      getProxyStatus(),
      getTokens(),
    ])
    logs.value = loadedLogs
    tokens.value = loadedTokens
    Object.assign(proxyStatus, loadedStatus)
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
    credentialType: 'api_key',
    tokenValue: '',
  })
}

function openEditForm(token) {
  Object.assign(form, {
    visible: true,
    editingId: token.id,
    name: token.name,
    provider: token.provider,
    credentialType: token.credentialType || 'api_key',
    tokenValue: token.tokenValue,
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
  if (credentialType === 'codex_auth_json') {
    try {
      JSON.parse(tokenValue)
    } catch {
      errorMessage.value = 'Codex auth.json 内容不是有效 JSON'
      return
    }
  } else if (provider === 'xiaomi' && credentialType === 'mimo_token_plan' && !tokenValue.startsWith('tp-')) {
    errorMessage.value = 'MiMo Token Plan Key 必须以 tp- 开头'
    return
  } else if (provider === 'xiaomi' && credentialType === 'api_key' && !tokenValue.startsWith('sk-')) {
    errorMessage.value = 'MiMo 按量 API Key 必须以 sk- 开头'
    return
  } else if (tokenValue.length < 12) {
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

async function persistConfig() {
  try {
    const saved = await saveConfig({
      proxyPort: Number(config.proxyPort),
      controlPort: Number(config.controlPort),
      upstreamBaseUrl: config.upstreamBaseUrl.trim(),
      openaiBaseUrl: config.openaiBaseUrl.trim(),
      anthropicBaseUrl: config.anthropicBaseUrl.trim(),
      deepseekBaseUrl: config.deepseekBaseUrl.trim(),
      deepseekAnthropicBaseUrl: config.deepseekAnthropicBaseUrl.trim(),
      kimiBaseUrl: config.kimiBaseUrl.trim(),
      xiaomiBaseUrl: config.xiaomiBaseUrl.trim(),
      xiaomiApiBaseUrl: config.xiaomiApiBaseUrl.trim(),
      xiaomiApiAnthropicBaseUrl: config.xiaomiApiAnthropicBaseUrl.trim(),
      xiaomiTokenPlanBaseUrl: config.xiaomiTokenPlanBaseUrl.trim(),
      xiaomiTokenPlanAnthropicBaseUrl: config.xiaomiTokenPlanAnthropicBaseUrl.trim(),
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

async function chooseDataDirectory() {
  dataDirChanging.value = true
  errorMessage.value = ''
  try {
    const result = await ChooseDataDirectory(true)
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

function maskToken(value) {
  if (!value) return ''
  if (value.length <= 12) return `${value.slice(0, 3)}...`
  return `${value.slice(0, 7)}...${value.slice(-4)}`
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
}

function formatTime(value) {
  if (!value) return '-'
  return new Intl.DateTimeFormat('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  }).format(new Date(value))
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

function formatResetTime(value) {
  if (!value) return '-'
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value * 1000))
}

function usageUpdatedAt(item) {
  return item.usage?.updatedAt ? formatTime(item.usage.updatedAt) : '-'
}

function isCodexToken(item) {
  return item?.provider === 'openai' && item?.credentialType === 'codex_auth_json'
}

function formatNumber(value) {
  return new Intl.NumberFormat('zh-CN').format(Number(value || 0))
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

function localDateKey(date = new Date()) {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
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
        <div class="brand-mark">OP</div>
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
          {{ tab.label }}
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
          <el-tag v-if="currentToken" type="primary" effect="plain" round class="current-account-tag">
            使用账号：{{ currentToken.name }}
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

      <section v-if="activeTab === 'dashboard'" class="view-grid">
        <article class="metric-card">
          <span>正在使用账号</span>
          <strong class="metric-token-name">{{ currentToken?.name || '暂无可用账号' }}</strong>
          <small>{{ currentToken ? `${providerLabel(currentToken.provider)} · ${currentToken.remaining}% · ${formatTime(currentToken.lastUsedAt)}` : '请先添加账号' }}</small>
        </article>
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
          <small>低额度 {{ lowTokens.length }} · 耗尽 {{ exhaustedTokens.length }}</small>
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

        <section class="panel wide">
          <div class="section-heading">
            <div>
              <h2>额度概览</h2>
              <p>根据上游响应头实时更新</p>
            </div>
            <button type="button" class="ghost-button" @click="refreshAll">刷新</button>
          </div>
          <div class="quota-list">
            <div v-for="item in tokens" :key="item.id" class="quota-row">
              <div>
                <strong>{{ item.name }}</strong>
                <span :class="['tag', statusClass(item.status)]">{{ statusLabel(item.status) }}</span>
              </div>
              <div class="progress">
                <span :style="{ width: `${Math.max(0, Math.min(100, item.remaining))}%` }"></span>
              </div>
              <small>{{ item.remaining }}%</small>
            </div>
            <div v-if="!tokens.length" class="empty">暂无账号</div>
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
              <p>{{ entry.message }}</p>
              <small>{{ formatTime(entry.time) }}</small>
            </div>
            <div v-if="!logs.length" class="empty">暂无日志</div>
          </div>
        </section>

        <section class="panel full">
          <div class="section-heading">
            <div>
              <h2>分天 Token 统计</h2>
              <p>Token 数来自上游 usage；请求数统计成功通过代理返回的请求</p>
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

      <section v-if="activeTab === 'quotas'" class="panel">
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
                </div>
                <div class="quota-head-actions">
                  <el-tag v-if="isCodexToken(item) && item.usage?.subscriptionQuotaAvailable" type="primary" effect="plain">
                    {{ planLabel(item.usage?.planType) }}
                  </el-tag>
                  <el-tag :type="statusType(item.status)" effect="light" class="status-tag">{{ statusLabel(item.status) }}</el-tag>
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
                <div class="quota-limit">
                  <div class="quota-limit-title">
                    <span>5h额度</span>
                    <strong v-if="item.usage?.subscriptionQuotaAvailable">{{ item.usage.primaryRemainingPercent }}%</strong>
                    <strong v-else>-</strong>
                  </div>
                  <el-progress
                    :percentage="item.usage?.subscriptionQuotaAvailable ? item.usage.primaryRemainingPercent : 0"
                    :show-text="false"
                    :stroke-width="8"
                  />
                  <small v-if="item.usage?.subscriptionQuotaAvailable">
                    已用 {{ item.usage.primaryUsedPercent }}% · 重置 {{ formatResetTime(item.usage.primaryResetAt) }}
                  </small>
                  <small v-else>{{ isCodexToken(item) ? '点击刷新额度获取' : 'API Key 暂无订阅额度' }}</small>
                </div>

                <div class="quota-limit">
                  <div class="quota-limit-title">
                    <span>1 周额度</span>
                    <strong v-if="item.usage?.subscriptionQuotaAvailable">{{ item.usage.secondaryRemainingPercent }}%</strong>
                    <strong v-else>-</strong>
                  </div>
                  <el-progress
                    :percentage="item.usage?.subscriptionQuotaAvailable ? item.usage.secondaryRemainingPercent : 0"
                    :show-text="false"
                    :stroke-width="8"
                  />
                  <small v-if="item.usage?.subscriptionQuotaAvailable">
                    已用 {{ item.usage.secondaryUsedPercent }}% · 重置 {{ formatResetTime(item.usage.secondaryResetAt) }}
                  </small>
                  <small v-else>{{ isCodexToken(item) ? '点击刷新额度获取' : 'API Key 暂无订阅额度' }}</small>
                </div>

                <div v-if="!isCodexToken(item)" class="quota-stat">
                  <span>API 剩余额度</span>
                  <strong>{{ item.usage?.apiRemaining || item.remaining }}%</strong>
                  <small>最后更新 {{ usageUpdatedAt(item) }}</small>
                </div>

                <div class="quota-stat">
                  <span>代理请求</span>
                  <strong>{{ formatNumber(item.stats?.requestCount) }} 次</strong>
                  <small>{{ tokenUsageSummary(item) }}</small>
                </div>
              </div>
            </div>
          </el-card>
          <div v-if="!activeProviderTokens.length" class="empty">
            暂无 {{ activeProviderInfo.label }} 账号
          </div>
        </div>
      </section>

      <section v-if="activeTab === 'tokens'" class="panel">
        <div class="section-heading">
          <div>
            <h2>账号管理</h2>
            <p>按厂商独立管理账号池，新添加账号默认显示在对应分组顶部</p>
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
            <p>{{ activeProviderInfo.note }} · {{ activeProviderTokens.length }} 个账号</p>
          </div>
          <el-button type="primary" :icon="Connection" @click="openCreateForm(activeProvider)">
            添加 {{ activeProviderInfo.label }}
          </el-button>
        </div>

        <div class="table-wrap">
          <table class="account-table">
            <colgroup>
              <col class="account-col-name" />
              <col class="account-col-credential-type" />
              <col class="account-col-credential" />
              <col class="account-col-quota" />
              <col class="account-col-usage" />
              <col class="account-col-status" />
              <col class="account-col-last-used" />
              <col class="account-col-actions" />
            </colgroup>
            <thead>
              <tr>
                <th>账号名称</th>
                <th>凭据类型</th>
                <th>凭据</th>
                <th>额度</th>
                <th>代理用量</th>
                <th>状态</th>
                <th>最后使用</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in activeProviderTokens" :key="item.id">
                <td>
                  <strong>{{ item.name }}</strong>
                  <small v-if="item.lastError">{{ item.lastError }}</small>
                </td>
                <td>{{ credentialLabel(item) }}</td>
                <td class="mono">{{ item.credentialType === 'codex_auth_json' ? 'auth.json' : maskToken(item.tokenValue) }}</td>
                <td>{{ item.remaining }}%</td>
                <td>
                  {{ formatNumber(item.stats?.totalTokens) }}
                  <small>{{ formatNumber(item.stats?.requestCount) }} 次请求</small>
                </td>
                <td><el-tag :type="statusType(item.status)" effect="light" class="status-tag">{{ statusLabel(item.status) }}</el-tag></td>
                <td>{{ formatTime(item.lastUsedAt) }}</td>
                <td class="actions-cell">
                  <div class="row-actions">
                    <el-button size="small" :icon="Refresh" :loading="validatingIds[item.id]" @click="verifyToken(item)">
                      {{ validatingIds[item.id] ? '验证中' : '验证' }}
                    </el-button>
                    <el-button size="small" @click="openEditForm(item)">编辑</el-button>
                    <el-button size="small" type="danger" plain @click="removeToken(item)">删除</el-button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
          <div v-if="!activeProviderTokens.length" class="empty">
            暂无 {{ activeProviderInfo.label }} 账号
          </div>
        </div>
      </section>

      <section v-if="activeTab === 'logs'" class="panel">
        <div class="section-heading">
          <div>
            <h2>实时日志</h2>
            <p>每 3 秒自动刷新</p>
          </div>
          <button type="button" class="ghost-button" @click="refreshRealtime">刷新</button>
        </div>
        <div class="log-list">
          <div v-for="entry in logs" :key="entry.id" class="log-row">
            <span :class="['dot', entry.level]"></span>
            <div>
              <strong>{{ entry.method || 'SYSTEM' }} {{ entry.path || '' }}</strong>
              <p>{{ entry.message }}</p>
            </div>
            <small class="log-status">{{ entry.status || '-' }}</small>
            <small class="log-duration">{{ entry.durationMs || 0 }}ms</small>
            <small class="log-token" :title="entry.tokenName || '-'">{{ entry.tokenName || '-' }}</small>
            <time class="log-time">{{ formatTime(entry.time) }}</time>
          </div>
          <div v-if="!logs.length" class="empty">暂无日志</div>
        </div>
      </section>

      <section v-if="activeTab === 'settings'" class="panel settings-panel">
        <div class="section-heading">
          <div>
            <h2>代理设置</h2>
            <p>保存后新请求会使用最新配置，端口变更需要重启代理</p>
          </div>
          <button type="button" class="primary-button" @click="persistConfig">保存设置</button>
        </div>
        <div class="data-directory-row">
          <div>
            <span>数据目录</span>
            <strong>{{ dataDirectory.dataDir || '未加载' }}</strong>
            <small v-if="dataDirectory.pendingDataDir && dataDirectory.restartRequired">
              重启后使用：{{ dataDirectory.pendingDataDir }}
            </small>
            <small v-else-if="dataDirectory.envOverride">
              当前由 OMNIPROXY_DATA_DIR 环境变量控制
            </small>
            <small v-else-if="dataDirectory.bootstrapPath">
              指针文件：{{ dataDirectory.bootstrapPath }}
            </small>
          </div>
          <button
            type="button"
            class="ghost-button"
            :disabled="dataDirectory.envOverride || dataDirChanging"
            @click="chooseDataDirectory"
          >
            {{ dataDirChanging ? '选择中' : '更改目录' }}
          </button>
        </div>
        <div class="settings-grid">
          <label>
            <span>代理端口</span>
            <input v-model="config.proxyPort" type="number" min="1" max="65535" />
          </label>
          <label>
            <span>控制端口</span>
            <input v-model="config.controlPort" type="number" min="1" max="65535" />
          </label>
          <label class="wide-field">
            <span>OpenAI API Base URL</span>
            <input v-model="config.openaiBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>Anthropic API Base URL</span>
            <input v-model="config.anthropicBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>DeepSeek API Base URL</span>
            <input v-model="config.deepseekBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>DeepSeek Anthropic Base URL</span>
            <input v-model="config.deepseekAnthropicBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>Kimi Code Base URL</span>
            <input v-model="config.kimiBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>Xiaomi MiMo 按量 OpenAI Base URL</span>
            <input v-model="config.xiaomiApiBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>Xiaomi MiMo 按量 Anthropic Base URL</span>
            <input v-model="config.xiaomiApiAnthropicBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>Xiaomi MiMo Token Plan OpenAI Base URL</span>
            <input v-model="config.xiaomiTokenPlanBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>Xiaomi MiMo Token Plan Anthropic Base URL</span>
            <input v-model="config.xiaomiTokenPlanAnthropicBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>Codex ChatGPT Base URL</span>
            <input v-model="config.codexBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>兼容旧版上游 API Base URL</span>
            <input v-model="config.upstreamBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>Codex 限额查询地址</span>
            <input v-model="config.codexUsageEndpoint" type="url" />
          </label>
          <label>
            <span>额度切换阈值</span>
            <input v-model="config.switchThreshold" type="number" min="1" max="100" />
          </label>
          <label>
            <span>自动重试次数</span>
            <input v-model="config.maxRetries" type="number" min="0" max="5" />
          </label>
        </div>
      </section>

      <section v-if="activeTab === 'quickstart'" class="panel help-panel">
        <div class="section-heading">
          <div>
            <h2>一键配置</h2>
            <p>把本机 Codex 或 Claude Code 指向 OmniProxy，本页只负责写入本地客户端配置</p>
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
        </div>
      </section>

      <section v-if="activeTab === 'help'" class="panel help-panel">
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
            <p>进入额度页面，选择厂商后查看每个账号的状态。Codex 账号刷新后会显示订阅类型、5h额度和 1 周额度。</p>
          </article>
          <article>
            <strong>3. 启动代理</strong>
            <p>确认代理设置里的端口和各厂商 Base URL 后，点击右上角启动代理。客户端请求走本地代理端口，由程序按账号状态自动调度。</p>
          </article>
          <article>
            <strong>4. 排查问题</strong>
            <p>请求失败时先看实时日志，再在账号管理里验证对应账号。额度过低或账号无效时，程序会按阈值跳过不可用账号。</p>
          </article>
        </div>
      </section>

      <div v-if="form.visible" class="modal-backdrop" @click.self="closeForm">
        <form class="modal" @submit.prevent="submitForm">
          <div class="section-heading">
            <div>
              <h2>{{ form.editingId ? '编辑账号' : '添加账号' }}</h2>
              <p>{{ isCodexForm ? 'Codex 将自动使用 auth.json 中的邮箱作为账号名称' : '账号名称必填且不可重复' }}</p>
            </div>
            <button type="button" class="icon-button" @click="closeForm">×</button>
          </div>
          <label v-if="!isCodexForm">
            <span>账号名称</span>
            <input v-model="form.name" autofocus />
          </label>
          <div v-else class="form-hint">
            账号名称会从 `tokens.id_token` 自动解析邮箱，无需手动填写。
          </div>
          <label>
            <span>厂商</span>
            <select v-model="form.provider" @change="onProviderChange">
              <option v-for="provider in providers" :key="provider.key" :value="provider.key">
                {{ provider.label }}
              </option>
            </select>
          </label>
          <label>
            <span>凭据类型</span>
            <select v-model="form.credentialType" :disabled="form.provider !== 'openai' && form.provider !== 'xiaomi'">
              <option value="api_key">{{ form.provider === 'xiaomi' ? 'MiMo 按量 API Key (sk-)' : 'API Key' }}</option>
              <option v-if="form.provider === 'openai'" value="codex_auth_json">Codex auth.json</option>
              <option v-if="form.provider === 'xiaomi'" value="mimo_token_plan">MiMo Token Plan (tp-)</option>
            </select>
          </label>
          <label>
            <span>{{ form.credentialType === 'codex_auth_json' ? 'auth.json 内容' : form.credentialType === 'mimo_token_plan' ? 'Token Plan Key' : 'API Key' }}</span>
            <textarea
              v-model="form.tokenValue"
              :rows="form.credentialType === 'codex_auth_json' ? 9 : 4"
              :placeholder="form.credentialType === 'codex_auth_json' ? '粘贴 ~/.codex/auth.json 的完整 JSON 内容' : form.credentialType === 'mimo_token_plan' ? '粘贴 tp- 开头的 MiMo Token Plan Key' : form.provider === 'xiaomi' ? '粘贴 sk- 开头的 MiMo 按量 API Key' : form.provider === 'kimi' ? '粘贴 Kimi Code API Key' : '粘贴 API Key'"
            ></textarea>
          </label>
          <div class="modal-actions">
            <button type="button" class="ghost-button" @click="closeForm">取消</button>
            <button type="submit" class="primary-button">保存</button>
          </div>
        </form>
      </div>

      <div v-if="loading" class="loading">加载中...</div>
    </main>
  </div>
</template>
