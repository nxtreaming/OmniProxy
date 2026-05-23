<script setup>
import { ref } from 'vue'

const coreUrlsExpanded = ref(false)
const thirdPartyUrlsExpanded = ref(false)

const props = defineProps({
  config: {
    type: Object,
    required: true,
  },
  dataDirectory: {
    type: Object,
    required: true,
  },
  dataDirChanging: {
    type: Boolean,
    required: true,
  },
  autoStartChanging: {
    type: Boolean,
    required: true,
  },
  autoStartEnabled: {
    type: Boolean,
    required: true,
  },
  clearingBillingUsage: {
    type: Boolean,
    required: true,
  },
  clearingRequestHistory: {
    type: Boolean,
    required: true,
  },
})

defineEmits([
  'persist-config',
  'choose-data-directory',
  'toggle-auto-start',
  'clear-billing-usage',
  'clear-request-history',
])

const outboundProxyPresets = [
  { label: '10808 mixed', url: 'http://127.0.0.1:10808' },
  { label: '7890 mixed', url: 'http://127.0.0.1:7890' },
  { label: 'SOCKS5 10808', url: 'socks5://127.0.0.1:10808' },
]
const taskAutomationClientOptions = [
  { key: 'codex', label: 'Codex' },
  { key: 'claude', label: 'Claude Code' },
  { key: 'claude-desktop', label: 'Claude Desktop' },
  { key: 'opencode', label: 'OpenCode' },
  { key: 'gemini', label: 'Gemini CLI' },
  { key: 'deepseek-tui', label: 'DeepSeek-TUI' },
  { key: 'pi', label: 'Pi Agent' },
]
const taskAutomationLaunchModes = [
  { key: 'media', label: '短视频 / 媒体' },
  { key: 'linuxdo', label: 'Linux.do 浏览器' },
]
const taskAutomationTargetPresets = [
  { key: 'douyin', label: '抖音', mode: 'media', target: 'preset:douyin', fallbackUrl: 'https://www.douyin.com' },
  { key: 'bilibili', label: '哔哩哔哩', mode: 'media', target: 'preset:bilibili', fallbackUrl: 'https://www.bilibili.com' },
  { key: 'linuxdo', label: 'Linux.do', mode: 'linuxdo', target: 'preset:linuxdo', fallbackUrl: 'https://linux.do/' },
]
const taskAutomationBrowserOptions = [
  { key: 'default', label: '默认浏览器' },
  { key: 'edge', label: 'Microsoft Edge' },
  { key: 'chrome', label: 'Google Chrome' },
  { key: 'firefox', label: 'Firefox' },
]
const recommendedOutboundProxyProviders = ['openai', 'anthropic', 'gemini', 'openrouter', 'zo']
const outboundProxyProviderGroups = [
  {
    title: '国内网络建议出站',
    note: '这些接入厂商通常依赖海外入口，模型列表、额度刷新和对话请求都会走出站代理。',
    items: [
      {
        key: 'openai-codex',
        label: 'OpenAI / Codex',
        providers: ['openai'],
        description: 'OpenAI API、Codex auth.json、chatgpt.com Codex 接口',
        recommended: true,
      },
      {
        key: 'anthropic-claude',
        label: 'Anthropic Claude',
        providers: ['anthropic'],
        description: 'Anthropic API、Claude OAuth 和 Claude 兼容路由',
        recommended: true,
      },
      {
        key: 'google-gemini',
        label: 'Google Gemini',
        providers: ['gemini'],
        description: 'Google Gemini 原生接口和模型列表',
        recommended: true,
      },
      {
        key: 'openrouter',
        label: 'OpenRouter',
        providers: ['openrouter'],
        description: 'OpenRouter 模型列表、测试对话、余额和代理转发',
        recommended: true,
      },
      {
        key: 'zo',
        label: 'Zo Computer',
        providers: ['zo'],
        description: 'Zo Computer 模型映射、模型列表和对话请求',
        recommended: true,
      },
    ],
  },
  {
    title: '国内通常可直连',
    note: '这些是当前内置国内厂商，默认不走出站代理。',
    items: [
      {
        key: 'deepseek',
        label: 'DeepSeek',
        providers: ['deepseek'],
        description: 'DeepSeek API 和 DeepSeek 兼容路由',
      },
      {
        key: 'kimi',
        label: 'Kimi Code',
        providers: ['kimi'],
        description: 'kimi-for-coding',
      },
      {
        key: 'zhipu',
        label: 'Zhipu GLM',
        providers: ['zhipu'],
        description: '智谱 GLM API、Coding Plan 和兼容接口',
      },
      {
        key: 'minimax',
        label: 'MiniMax',
        providers: ['minimax'],
        description: 'MiniMax API 和 Coding Plan',
      },
      {
        key: 'mimo',
        label: 'Xiaomi MiMo',
        providers: ['xiaomi'],
        description: 'Xiaomi MiMo API Key 和 Token Plan',
      },
    ],
  },
  {
    title: '取决于你的上游',
    note: '自定义网关、Sub2API、TokenRouter 是否需要出站，取决于你配置的实际服务地址。',
    items: [
      {
        key: 'tokenrouter',
        label: 'TokenRouter',
        providers: ['tokenrouter'],
        description: 'TokenRouter 账号、模型和路由规则接口',
      },
      {
        key: 'sub2api',
        label: 'Sub2API',
        providers: ['sub2api'],
        description: 'Sub2API OpenAI / Anthropic / Gemini 兼容接口',
      },
      {
        key: 'custom',
        label: '自定义网关',
        providers: ['custom'],
        description: '自定义 OpenAI / Anthropic 兼容网关',
      },
    ],
  },
]

function setOutboundProxyUrl(url) {
  props.config.outboundProxyUrl = url
  props.config.outboundProxyEnabled = true
}

function resetOutboundProxyProviders() {
  props.config.outboundProxyProviders = [...recommendedOutboundProxyProviders]
}

function resetTaskAutomationClients() {
  props.config.taskAutomationClients = ['codex', 'claude', 'claude-desktop']
}

function taskAutomationLaunchMode() {
  const mode = String(props.config.taskAutomationLaunchMode || '').trim().toLowerCase()
  return mode === 'linuxdo' || mode === 'linux.do' || mode === 'linux-do' || mode === 'browser' ? 'linuxdo' : 'media'
}

function setTaskAutomationLaunchMode(mode) {
  const normalized = mode === 'linuxdo' ? 'linuxdo' : 'media'
  props.config.taskAutomationLaunchMode = normalized
  if (normalized === 'linuxdo') {
    props.config.taskAutomationLaunchTarget = 'preset:linuxdo'
    props.config.taskAutomationFallbackUrl = 'https://linux.do/'
  } else if (String(props.config.taskAutomationLaunchTarget || '').trim().toLowerCase() === 'preset:linuxdo') {
    props.config.taskAutomationLaunchTarget = 'preset:douyin'
    props.config.taskAutomationFallbackUrl = 'https://www.douyin.com'
  }
}

function isTaskAutomationLaunchMode(mode) {
  return taskAutomationLaunchMode() === mode
}

function taskAutomationBrowser() {
  const browser = String(props.config.taskAutomationBrowser || '').trim().toLowerCase().replaceAll('_', '-')
  return taskAutomationBrowserOptions.some((item) => item.key === browser) ? browser : 'default'
}

function setTaskAutomationBrowser(browser) {
  props.config.taskAutomationBrowser = taskAutomationBrowserOptions.some((item) => item.key === browser) ? browser : 'default'
}

function isTaskAutomationBrowser(browser) {
  return taskAutomationBrowser() === browser
}

function isTaskAutomationLinuxDO() {
  return taskAutomationLaunchMode() === 'linuxdo'
}

function applyTaskAutomationTargetPreset(preset) {
  props.config.taskAutomationLaunchMode = preset.mode || 'media'
  props.config.taskAutomationLaunchTarget = preset.target
  props.config.taskAutomationFallbackUrl = preset.fallbackUrl
}

function isTaskAutomationTargetPresetSelected(preset) {
  if ((preset.mode || 'media') !== taskAutomationLaunchMode()) return false
  return String(props.config.taskAutomationLaunchTarget || '').trim().toLowerCase() === preset.target
}

function toggleTaskAutomationClient(key) {
  if (hasTaskAutomationClient(key)) {
    props.config.taskAutomationClients = selectedTaskAutomationClients().filter((item) => item !== key)
  } else {
    props.config.taskAutomationClients = normalizeTaskAutomationClients([...selectedTaskAutomationClients(), key])
  }
}

function hasTaskAutomationClient(key) {
  return selectedTaskAutomationClients().includes(key)
}

function selectedTaskAutomationClients() {
  return normalizeTaskAutomationClients(Array.isArray(props.config.taskAutomationClients) ? props.config.taskAutomationClients : [])
}

function normalizeTaskAutomationClients(clients) {
  const known = new Set(taskAutomationClientOptions.map((item) => item.key))
  const seen = new Set()
  const next = []
  for (const client of clients) {
    const key = String(client || '').trim().toLowerCase().replaceAll('_', '-')
    if (!known.has(key) || seen.has(key)) continue
    seen.add(key)
    next.push(key)
  }
  return next
}

function toggleOutboundProxyProvider(item) {
  if (isOutboundProxyProviderSelected(item)) {
    removeOutboundProxyProviders(item.providers)
  } else {
    addOutboundProxyProviders(item.providers)
  }
}

function addOutboundProxyProviders(providers) {
  props.config.outboundProxyProviders = normalizeOutboundProxyProviders([
    ...(Array.isArray(props.config.outboundProxyProviders) ? props.config.outboundProxyProviders : []),
    ...providers,
  ])
}

function removeOutboundProxyProviders(providers) {
  const keys = new Set(providers.map((provider) => String(provider || '').trim().toLowerCase()).filter(Boolean))
  props.config.outboundProxyProviders = (Array.isArray(props.config.outboundProxyProviders)
    ? props.config.outboundProxyProviders
    : []
  ).filter((item) => !keys.has(String(item || '').trim().toLowerCase()))
}

function isOutboundProxyProviderSelected(item) {
  return item.providers.every((provider) => hasOutboundProxyProvider(provider))
}

function hasOutboundProxyProvider(provider) {
  const key = String(provider || '').trim().toLowerCase()
  return selectedOutboundProxyProviders().some((item) => String(item || '').trim().toLowerCase() === key)
}

function selectedOutboundProxyProviders() {
  return Array.isArray(props.config.outboundProxyProviders) ? props.config.outboundProxyProviders : []
}

function selectedOutboundProxyProviderCount() {
  return selectedOutboundProxyProviders().length
}

function customOutboundProxyProviders() {
  const known = new Set(
    outboundProxyProviderGroups.flatMap((group) => group.items).flatMap((item) => item.providers.map((provider) => provider.toLowerCase())),
  )
  return selectedOutboundProxyProviders().filter((provider) => !known.has(String(provider || '').trim().toLowerCase()))
}

function normalizeOutboundProxyProviders(providers) {
  const seen = new Set()
  const next = []
  for (const provider of providers) {
    const value = String(provider || '').trim().toLowerCase()
    const key = value.toLowerCase()
    if (!value || seen.has(key)) continue
    seen.add(key)
    next.push(value)
  }
  return next
}
</script>

<template>
  <section class="settings-panel">
    <div class="settings-page-toolbar">
      <p>保存后新请求会使用最新配置，端口变更需要重启代理。</p>
      <button type="button" class="primary-button" @click="$emit('persist-config')">保存设置</button>
    </div>
    <div class="settings-stack">
      <div class="settings-columns">
        <div class="settings-primary-column">
          <section class="settings-section settings-maintenance-section">
            <div class="settings-section-head">
              <div>
                <h3>应用维护</h3>
                <p>本地数据目录和后台常驻集中放在这里。</p>
              </div>
            </div>
            <div class="settings-action-list">
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
                  @click="$emit('choose-data-directory')"
                >
                  {{ dataDirChanging ? '选择中' : '更改目录' }}
                </button>
              </div>
              <div class="data-directory-row startup-row">
                <div>
                  <span>常驻后台</span>
                  <strong>系统托盘与开机自启</strong>
                  <small>关闭主窗口时保留托盘入口，可从托盘启动/停止代理、查看端口、打开主界面或退出。</small>
                </div>
                <button
                  type="button"
                  class="ghost-button"
                  :disabled="autoStartChanging"
                  @click="$emit('toggle-auto-start')"
                >
                  {{ autoStartChanging ? '更新中' : autoStartEnabled ? '关闭自启' : '开启自启' }}
                </button>
              </div>
              <div class="data-directory-row maintenance-row">
                <div>
                  <span>账单与请求历史</span>
                  <strong>每日账单汇总保留 {{ config.historyRetentionDays || 14 }} 天</strong>
                  <small>默认保留 14 天；每日汇总只记录最终 Token 用量，不保存完整请求日志。</small>
                  <label class="inline-number-field">
                    <span>保留天数</span>
                    <input v-model="config.historyRetentionDays" type="number" min="1" max="365" />
                  </label>
                </div>
                <div class="maintenance-actions">
                  <button
                    type="button"
                    class="danger-button"
                    :disabled="clearingBillingUsage"
                    @click="$emit('clear-billing-usage')"
                  >
                    {{ clearingBillingUsage ? '清理中' : '清空账单汇总' }}
                  </button>
                  <button
                    type="button"
                    class="danger-button"
                    :disabled="clearingRequestHistory"
                    @click="$emit('clear-request-history')"
                  >
                    {{ clearingRequestHistory ? '清理中' : '清空请求历史' }}
                  </button>
                </div>
              </div>
            </div>
          </section>

          <section class="settings-section settings-service-section">
            <div class="settings-section-head">
              <div>
                <h3>本机服务</h3>
                <p>端口和本地代理能力，端口变更后需要重启代理。</p>
              </div>
            </div>
            <div class="settings-grid compact-settings-grid">
              <label>
                <span>代理端口</span>
                <input v-model="config.proxyPort" type="number" min="1" max="65535" />
              </label>
              <label>
                <span>控制端口</span>
                <input v-model="config.controlPort" type="number" min="1" max="65535" />
              </label>
              <label class="toggle-field">
                <span>启用 Codex WebSocket</span>
                <input
                  v-model="config.websocketMode"
                  class="toggle-input"
                  type="checkbox"
                  true-value="enabled"
                  false-value="disabled"
                />
                <span class="toggle-switch" aria-hidden="true">
                  <span class="toggle-thumb"></span>
                </span>
              </label>
            </div>
          </section>

          <section class="settings-section settings-scheduling-section">
            <div class="settings-section-head">
              <div>
                <h3>调度与保护</h3>
                <p>控制账号轮换、低额度跳过和失败重试。</p>
              </div>
            </div>
            <div class="settings-grid compact-settings-grid">
              <div class="settings-segmented-field">
                <span>账号调度模式</span>
                <div
                  :class="[
                    'settings-segmented',
                    'settings-segmented-two',
                    config.schedulingMode === 'balanced' ? 'active-right' : 'active-left',
                  ]"
                  role="group"
                  aria-label="账号调度模式"
                >
                  <button
                    type="button"
                    :class="{ active: config.schedulingMode === 'queue' }"
                    :aria-pressed="config.schedulingMode === 'queue'"
                    @click="config.schedulingMode = 'queue'"
                  >
                    队列模式
                  </button>
                  <button
                    type="button"
                    :class="{ active: config.schedulingMode === 'balanced' }"
                    :aria-pressed="config.schedulingMode === 'balanced'"
                    @click="config.schedulingMode = 'balanced'"
                  >
                    优先平衡使用
                  </button>
                </div>
              </div>
              <label>
                <span>额度切换阈值</span>
                <input v-model="config.switchThreshold" type="number" min="1" max="100" />
              </label>
              <label>
                <span>自动重试次数</span>
                <input v-model="config.maxRetries" type="number" min="0" max="5" />
              </label>
              <div class="settings-segmented-field">
                <span>MiMo 优先使用</span>
                <div
                  :class="[
                    'settings-segmented',
                    'settings-segmented-two',
                    config.xiaomiCredentialPriority === 'api_key' ? 'active-right' : 'active-left',
                  ]"
                  role="group"
                  aria-label="MiMo 凭据优先级"
                >
                  <button
                    type="button"
                    :class="{ active: config.xiaomiCredentialPriority === 'mimo_token_plan' }"
                    :aria-pressed="config.xiaomiCredentialPriority === 'mimo_token_plan'"
                    @click="config.xiaomiCredentialPriority = 'mimo_token_plan'"
                  >
                    Token Plan
                  </button>
                  <button
                    type="button"
                    :class="{ active: config.xiaomiCredentialPriority === 'api_key' }"
                    :aria-pressed="config.xiaomiCredentialPriority === 'api_key'"
                    @click="config.xiaomiCredentialPriority = 'api_key'"
                  >
                    按量 API
                  </button>
                </div>
              </div>
            </div>
          </section>
        </div>

        <div class="settings-side-column">
          <section class="settings-section settings-task-automation-section">
            <div class="settings-section-head">
              <div>
                <h3>放心刷</h3>
                <p>检测 CLI 请求活动：可以打开短视频应用并在结束时暂停，也可以打开带登录态的浏览器去刷 Linux.do。</p>
              </div>
            </div>
            <div class="settings-grid">
              <label class="toggle-field">
                <span>启用放心刷</span>
                <input v-model="config.taskAutomationEnabled" class="toggle-input" type="checkbox" />
                <span class="toggle-switch" aria-hidden="true">
                  <span class="toggle-thumb"></span>
                </span>
              </label>
              <label class="toggle-field">
                <span>任务结束后切回 CLI</span>
                <input v-model="config.taskAutomationReturnToClient" class="toggle-input" type="checkbox" />
                <span class="toggle-switch" aria-hidden="true">
                  <span class="toggle-thumb"></span>
                </span>
              </label>
              <div class="wide-field settings-chip-field">
                <span>打开方式</span>
                <div class="settings-chip-list">
                  <button
                    v-for="mode in taskAutomationLaunchModes"
                    :key="mode.key"
                    type="button"
                    class="settings-chip-button"
                    :class="{ active: isTaskAutomationLaunchMode(mode.key) }"
                    @click="setTaskAutomationLaunchMode(mode.key)"
                  >
                    {{ mode.label }}
                  </button>
                </div>
                <small>Linux.do 浏览器模式不会发送空格暂停，只负责打开站点并按设置切回 CLI。</small>
              </div>
              <label class="wide-field">
                <span>开始时打开</span>
                <input
                  v-model="config.taskAutomationLaunchTarget"
                  type="text"
                  placeholder="选择预设，或填 exe / lnk 路径、网址"
                  :disabled="isTaskAutomationLinuxDO()"
                />
                <small v-if="isTaskAutomationLinuxDO()">Linux.do 模式固定打开 https://linux.do/，浏览器登录态由下方用户资料决定。</small>
                <small v-else>留空默认使用抖音；预设会优先找本地程序，找不到再打开备用网址。</small>
              </label>
              <div class="wide-field settings-chip-field">
                <span>常用目标</span>
                <div class="settings-chip-list">
                  <button
                    v-for="preset in taskAutomationTargetPresets"
                    :key="preset.key"
                    type="button"
                    class="settings-chip-button"
                    :class="{ active: isTaskAutomationTargetPresetSelected(preset) }"
                    @click="applyTaskAutomationTargetPreset(preset)"
                  >
                    {{ preset.label }}
                  </button>
                </div>
                <small>抖音和哔哩哔哩会优先尝试桌面端；Linux.do 会按浏览器设置打开网页。</small>
              </div>
              <label v-if="!isTaskAutomationLinuxDO()" class="wide-field">
                <span>备用网址</span>
                <input v-model="config.taskAutomationFallbackUrl" type="url" placeholder="https://www.douyin.com" />
              </label>
              <div v-if="isTaskAutomationLinuxDO()" class="wide-field settings-chip-field">
                <span>浏览器类型</span>
                <div class="settings-chip-list">
                  <button
                    v-for="browser in taskAutomationBrowserOptions"
                    :key="browser.key"
                    type="button"
                    class="settings-chip-button"
                    :class="{ active: isTaskAutomationBrowser(browser.key) }"
                    @click="setTaskAutomationBrowser(browser.key)"
                  >
                    {{ browser.label }}
                  </button>
                </div>
                <small>默认浏览器无法指定用户资料；Edge、Chrome、Firefox 支持指定已有登录态的 Profile。</small>
              </div>
              <label v-if="isTaskAutomationLinuxDO()" class="wide-field">
                <span>用户数据目录</span>
                <input
                  v-model="config.taskAutomationBrowserUserDataDir"
                  type="text"
                  placeholder="%LOCALAPPDATA%\\Microsoft\\Edge\\User Data"
                />
                <small>Edge/Chrome 可填 User Data 根目录；Firefox 可留空，直接在用户资料里填 Profile 名或路径。</small>
              </label>
              <label v-if="isTaskAutomationLinuxDO()" class="wide-field">
                <span>用户资料 / Profile</span>
                <input v-model="config.taskAutomationBrowserProfile" type="text" placeholder="Default 或 Profile 1" />
                <small>填已有资料夹才会带登录态，例如 Edge/Chrome 的 Default、Profile 1，或 Firefox 的 profile 名/路径。</small>
              </label>
              <label>
                <span>空闲判定秒数</span>
                <input v-model="config.taskAutomationIdleSeconds" type="number" min="1" max="600" />
              </label>
              <label>
                <span>回切前等待秒数</span>
                <input v-model="config.taskAutomationReturnDelaySeconds" type="number" min="1" max="600" />
              </label>
              <div class="wide-field settings-chip-field">
                <div class="outbound-model-selector-head">
                  <div>
                    <span>触发客户端</span>
                    <small>已选择 {{ selectedTaskAutomationClients().length }} 个 CLI</small>
                  </div>
                  <button type="button" class="ghost-button compact-button" @click="resetTaskAutomationClients">
                    恢复默认
                  </button>
                </div>
                <div class="settings-chip-list">
                  <button
                    v-for="client in taskAutomationClientOptions"
                    :key="client.key"
                    type="button"
                    class="settings-chip-button"
                    :class="{ active: hasTaskAutomationClient(client.key) }"
                    @click="toggleTaskAutomationClient(client.key)"
                  >
                    {{ client.label }}
                  </button>
                </div>
                <small>任务活动来自 OmniProxy 代理请求；未经过本地代理的 CLI 不会触发。</small>
              </div>
            </div>
          </section>

          <section class="settings-section settings-outbound-section">
            <div class="settings-section-head">
              <div>
                <h3>出站代理</h3>
                <p>按接入厂商决定是否走 Clash、v2rayN 等本机代理端口，未选中的厂商继续直连。</p>
              </div>
            </div>
            <div class="settings-grid">
              <label class="toggle-field">
                <span>启用厂商出站代理</span>
                <input v-model="config.outboundProxyEnabled" class="toggle-input" type="checkbox" />
                <span class="toggle-switch" aria-hidden="true">
                  <span class="toggle-thumb"></span>
                </span>
              </label>
              <label>
                <span>本机代理地址</span>
                <input v-model="config.outboundProxyUrl" type="text" placeholder="10808 或 http://127.0.0.1:10808" />
              </label>
              <div class="wide-field settings-chip-field">
                <span>常用端口</span>
                <div class="settings-chip-list">
                  <button
                    v-for="preset in outboundProxyPresets"
                    :key="preset.url"
                    type="button"
                    class="settings-chip-button"
                    :class="{ active: config.outboundProxyUrl === preset.url }"
                    @click="setOutboundProxyUrl(preset.url)"
                  >
                    {{ preset.label }}
                  </button>
                </div>
                <small>Clash mixed-port 和 v2rayN mixed/http 入站可直接用 HTTP 地址；SOCKS 入站使用 socks5://。</small>
              </div>
              <div class="wide-field outbound-model-selector">
                <div class="outbound-model-selector-head">
                  <div>
                    <span>走出站代理的接入厂商</span>
                    <small>已选择 {{ selectedOutboundProxyProviderCount() }} 个厂商</small>
                  </div>
                  <button type="button" class="ghost-button compact-button" @click="resetOutboundProxyProviders">
                    恢复国内推荐
                  </button>
                </div>
                <div
                  v-for="group in outboundProxyProviderGroups"
                  :key="group.title"
                  class="outbound-model-group"
                >
                  <div class="outbound-model-group-head">
                    <strong>{{ group.title }}</strong>
                    <small>{{ group.note }}</small>
                  </div>
                  <div class="outbound-model-options">
                    <button
                      v-for="item in group.items"
                      :key="item.key"
                      type="button"
                      class="outbound-model-option"
                      :class="{ active: isOutboundProxyProviderSelected(item), recommended: item.recommended }"
                      @click="toggleOutboundProxyProvider(item)"
                    >
                      <span class="outbound-model-option-title">
                        <strong>{{ item.label }}</strong>
                        <em>{{ isOutboundProxyProviderSelected(item) ? '走出站' : '直连' }}</em>
                      </span>
                      <small>{{ item.description }}</small>
                      <code>{{ item.providers.join(' / ') }}</code>
                    </button>
                  </div>
                </div>
                <div v-if="customOutboundProxyProviders().length" class="settings-chip-field">
                  <span>未归类厂商</span>
                  <div class="settings-chip-list">
                    <button
                      v-for="provider in customOutboundProxyProviders()"
                      :key="provider"
                      type="button"
                      class="settings-chip-button active"
                      @click="removeOutboundProxyProviders([provider])"
                    >
                      {{ provider }}
                    </button>
                  </div>
                  <small>这些厂商来自旧配置；点击可移除。</small>
                </div>
              </div>
            </div>
          </section>
        </div>
      </div>

      <section class="settings-section settings-url-section settings-url-section-core">
        <div class="settings-section-head">
          <div>
            <h3>OpenAI / Anthropic / Codex</h3>
            <p>常用协议入口和 Codex 额度查询地址。</p>
          </div>
          <button type="button" class="ghost-button compact-button" @click="coreUrlsExpanded = !coreUrlsExpanded">
            {{ coreUrlsExpanded ? '收起地址' : '展开地址' }}
          </button>
        </div>
        <div v-if="coreUrlsExpanded" class="settings-grid">
          <label class="wide-field">
            <span>OpenAI API Base URL</span>
            <input v-model="config.openaiBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>Anthropic API Base URL</span>
            <input v-model="config.anthropicBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>Codex ChatGPT Base URL</span>
            <input v-model="config.codexBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>Codex 限额查询地址</span>
            <input v-model="config.codexUsageEndpoint" type="url" />
          </label>
          <label class="wide-field">
            <span>兼容旧版上游 API Base URL</span>
            <input v-model="config.upstreamBaseUrl" type="url" />
          </label>
        </div>
      </section>

      <section class="settings-section settings-url-section settings-url-section-third-party">
        <div class="settings-section-head">
          <div>
            <h3>第三方路由</h3>
            <p>DeepSeek、Kimi、Zhipu GLM、MiniMax、Gemini、OpenRouter、TokenRouter、sub2api、Zo Computer、Xiaomi MiMo 和自定义网关入口。</p>
          </div>
          <button type="button" class="ghost-button compact-button" @click="thirdPartyUrlsExpanded = !thirdPartyUrlsExpanded">
            {{ thirdPartyUrlsExpanded ? '收起地址' : '展开地址' }}
          </button>
        </div>
        <div v-if="thirdPartyUrlsExpanded" class="settings-grid">
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
            <span>Zhipu GLM OpenAI Base URL</span>
            <input v-model="config.zhipuBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>Zhipu GLM Anthropic Base URL</span>
            <input v-model="config.zhipuAnthropicBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>MiniMax OpenAI Base URL</span>
            <input v-model="config.minimaxBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>MiniMax Anthropic Base URL</span>
            <input v-model="config.minimaxAnthropicBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>Gemini Native Base URL</span>
            <input v-model="config.geminiBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>OpenRouter OpenAI Base URL</span>
            <input v-model="config.openrouterBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>TokenRouter OpenAI Base URL</span>
            <input v-model="config.tokenrouterBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>sub2api 默认 Base URL</span>
            <input v-model="config.sub2apiBaseUrl" type="url" />
            <small>仅作为新增 sub2api 账号的默认填充值，以及旧账号未保存 Base URL 时的回退地址；协议由本地路径决定。</small>
          </label>
          <label class="wide-field">
            <span>Zo Computer Base URL</span>
            <input v-model="config.zoBaseUrl" type="url" />
            <small>Zo 使用 /models/available 与 /zo/ask，上游协议由 OmniProxy 适配为 OpenAI / Anthropic。</small>
          </label>
          <label class="wide-field">
            <span>自定义网关 OpenAI Base URL</span>
            <input v-model="config.customGatewayBaseUrl" type="url" placeholder="https://your-gateway.example/v1" />
          </label>
          <label class="wide-field">
            <span>自定义网关 Anthropic Base URL</span>
            <input v-model="config.customGatewayAnthropicBaseUrl" type="url" placeholder="可选，留空则复用 OpenAI Base URL" />
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
            <span>Xiaomi MiMo Token Plan OpenAI Base URL（中国区）</span>
            <input v-model="config.xiaomiTokenPlanBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>Xiaomi MiMo Token Plan Anthropic Base URL（中国区）</span>
            <input v-model="config.xiaomiTokenPlanAnthropicBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>Xiaomi MiMo Token Plan OpenAI Base URL（海外 SGP）</span>
            <input v-model="config.xiaomiTokenPlanSgpBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>Xiaomi MiMo Token Plan Anthropic Base URL（海外 SGP）</span>
            <input v-model="config.xiaomiTokenPlanSgpAnthropicBaseUrl" type="url" />
          </label>
        </div>
      </section>
    </div>
  </section>
</template>
