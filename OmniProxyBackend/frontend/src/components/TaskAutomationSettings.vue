<script setup>
const props = defineProps({
  config: {
    type: Object,
    required: true,
  },
  browserProfiles: {
    type: Array,
    default: () => [],
  },
  browserProfilesLoading: {
    type: Boolean,
    default: false,
  },
  browserProfilesError: {
    type: String,
    default: '',
  },
})

const emit = defineEmits(['refresh-task-automation-browser-profiles'])

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
    emit('refresh-task-automation-browser-profiles', taskAutomationBrowser())
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
  emit('refresh-task-automation-browser-profiles', props.config.taskAutomationBrowser)
}

function isTaskAutomationBrowser(browser) {
  return taskAutomationBrowser() === browser
}

function isTaskAutomationLinuxDO() {
  return taskAutomationLaunchMode() === 'linuxdo'
}

function browserProfileKey(profile) {
  return [profile.browser, profile.userDataDir, profile.profile, profile.path].filter(Boolean).join('|')
}

function browserProfileTitle(profile) {
  const browserLabel = profile.browserLabel || taskAutomationBrowserOptions.find((item) => item.key === profile.browser)?.label || profile.browser
  const profileLabel = profile.label || profile.name || profile.profile || profile.path || 'Profile'
  return `${browserLabel} / ${profileLabel}`
}

function browserProfileDetail(profile) {
  if (!profile) return ''
  if (profile.browser === 'firefox') {
    return profile.path || profile.profile || ''
  }
  const parts = [profile.userDataDir, profile.profile].filter(Boolean)
  return parts.join(' / ')
}

function applyTaskAutomationBrowserProfile(profile) {
  if (!profile) return
  props.config.taskAutomationBrowser = profile.browser || taskAutomationBrowser()
  props.config.taskAutomationBrowserUserDataDir = profile.userDataDir || ''
  props.config.taskAutomationBrowserProfile = profile.profile || ''
}

function isTaskAutomationBrowserProfileSelected(profile) {
  if (!profile) return false
  return (
    taskAutomationBrowser() === profile.browser &&
    String(props.config.taskAutomationBrowserUserDataDir || '') === String(profile.userDataDir || '') &&
    String(props.config.taskAutomationBrowserProfile || '') === String(profile.profile || '')
  )
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
</script>

<template>
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
          placeholder="选择预设，或填应用路径、快捷方式、网址"
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
      <div v-if="isTaskAutomationLinuxDO()" class="wide-field browser-profile-field">
        <div class="browser-profile-head">
          <div>
            <span>已检测到的浏览器资料</span>
            <small>只读取浏览器配置和 Profile 目录；选择后会自动填入下方两个字段。</small>
          </div>
          <button
            type="button"
            class="ghost-button compact-button"
            :disabled="browserProfilesLoading"
            @click="$emit('refresh-task-automation-browser-profiles', taskAutomationBrowser())"
          >
            {{ browserProfilesLoading ? '扫描中' : '重新扫描' }}
          </button>
        </div>
        <div v-if="browserProfilesLoading" class="browser-profile-empty">正在扫描本机浏览器资料...</div>
        <div v-else-if="browserProfilesError" class="browser-profile-empty error">
          {{ browserProfilesError }}
        </div>
        <div v-else-if="browserProfiles.length" class="browser-profile-list">
          <button
            v-for="profile in browserProfiles"
            :key="browserProfileKey(profile)"
            type="button"
            class="browser-profile-option"
            :class="{ active: isTaskAutomationBrowserProfileSelected(profile) }"
            @click="applyTaskAutomationBrowserProfile(profile)"
          >
            <span class="browser-profile-main">
              <strong>{{ browserProfileTitle(profile) }}</strong>
              <small>{{ browserProfileDetail(profile) }}</small>
            </span>
            <span v-if="profile.isDefault" class="browser-profile-badge">默认</span>
          </button>
        </div>
        <div v-else class="browser-profile-empty">
          未检测到可用资料。可以手动填写，或先选择 Edge / Chrome / Firefox 后重新扫描。
        </div>
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
</template>
