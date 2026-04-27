<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import {
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
} from './services/api'

const tabs = [
  { key: 'dashboard', label: '仪表盘' },
  { key: 'tokens', label: '账号管理' },
  { key: 'logs', label: '实时日志' },
  { key: 'settings', label: '全局设置' },
]

const activeTab = ref('dashboard')
const sidebarWidth = ref(236)
const isDark = ref(false)
const loading = ref(false)
const errorMessage = ref('')
const successMessage = ref('')
const validatingIds = reactive({})
const tokens = ref([])
const logs = ref([])
const proxyStatus = reactive({ running: false, port: 3000 })
const config = reactive({
  proxyPort: 3000,
  controlPort: 3890,
  upstreamBaseUrl: 'https://api.openai.com',
  switchThreshold: 15,
  maxRetries: 2,
})
const form = reactive({
  visible: false,
  editingId: '',
  name: '',
  provider: 'openai',
  tokenValue: '',
})

const activeTokens = computed(() => tokens.value.filter((item) => item.status === 'active'))
const lowTokens = computed(() => tokens.value.filter((item) => item.status === 'low'))
const exhaustedTokens = computed(() => tokens.value.filter((item) => item.status === 'exhausted'))
const invalidTokens = computed(() => tokens.value.filter((item) => item.status === 'invalid'))
const currentToken = computed(() => activeTokens.value[0] || lowTokens.value[0] || null)

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

async function refreshAll() {
  loading.value = true
  errorMessage.value = ''
  try {
    const [loadedTokens, loadedConfig, loadedLogs, loadedStatus] = await Promise.all([
      getTokens(),
      getConfig(),
      getLogs(),
      getProxyStatus(),
    ])
    tokens.value = loadedTokens
    logs.value = loadedLogs
    Object.assign(config, loadedConfig)
    Object.assign(proxyStatus, loadedStatus)
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

function openCreateForm() {
  Object.assign(form, {
    visible: true,
    editingId: '',
    name: '',
    provider: 'openai',
    tokenValue: '',
  })
}

function openEditForm(token) {
  Object.assign(form, {
    visible: true,
    editingId: token.id,
    name: token.name,
    provider: token.provider,
    tokenValue: token.tokenValue,
  })
}

function closeForm() {
  form.visible = false
}

async function submitForm() {
  errorMessage.value = ''
  successMessage.value = ''
  const name = form.name.trim()
  const tokenValue = form.tokenValue.trim()

  if (!name) {
    errorMessage.value = '账号名称不能为空'
    return
  }
  const duplicate = tokens.value.some(
    (item) => item.id !== form.editingId && item.name.toLowerCase() === name.toLowerCase(),
  )
  if (duplicate) {
    errorMessage.value = '账号名称不可重复'
    return
  }
  if (tokenValue.length < 12) {
    errorMessage.value = 'Token 长度过短'
    return
  }

  const payload = {
    name,
    provider: form.provider.trim() || 'openai',
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
</script>

<template>
  <div class="shell" :class="{ dark: isDark }">
    <aside class="sidebar" :style="{ width: `${sidebarWidth}px` }">
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
        <label>
          <span>侧边栏</span>
          <input v-model="sidebarWidth" type="range" min="196" max="320" />
        </label>
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
          <span class="status-pill" :class="proxyStatus.running ? 'success' : 'muted'">
            代理 {{ proxyStatus.running ? '运行中' : '已停止' }} · :{{ proxyStatus.port }}
          </span>
          <button type="button" class="primary-button" @click="toggleProxy">
            {{ proxyStatus.running ? '停止代理' : '启动代理' }}
          </button>
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
          <span>当前 Token</span>
          <strong>{{ currentToken?.name || '暂无可用账号' }}</strong>
          <small>{{ currentToken ? `${currentToken.provider} · ${currentToken.remaining}%` : '请先添加 Token' }}</small>
        </article>
        <article class="metric-card">
          <span>正常账号</span>
          <strong>{{ activeTokens.length }}</strong>
          <small>低额度 {{ lowTokens.length }} · 耗尽 {{ exhaustedTokens.length }}</small>
        </article>
        <article class="metric-card">
          <span>切换阈值</span>
          <strong>{{ config.switchThreshold }}%</strong>
          <small>最大重试 {{ config.maxRetries }} 次</small>
        </article>
        <article class="metric-card">
          <span>无效账号</span>
          <strong>{{ invalidTokens.length }}</strong>
          <small>请求失败或手动标记</small>
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
      </section>

      <section v-if="activeTab === 'tokens'" class="panel">
        <div class="section-heading">
          <div>
            <h2>账号列表</h2>
            <p>新添加账号默认显示在顶部</p>
          </div>
          <button type="button" class="primary-button" @click="openCreateForm">添加账号</button>
        </div>

        <div class="table-wrap">
          <table class="account-table">
            <colgroup>
              <col class="account-col-name" />
              <col class="account-col-provider" />
              <col class="account-col-token" />
              <col class="account-col-quota" />
              <col class="account-col-status" />
              <col class="account-col-last-used" />
              <col class="account-col-actions" />
            </colgroup>
            <thead>
              <tr>
                <th>账号名称</th>
                <th>厂商</th>
                <th>Token</th>
                <th>额度</th>
                <th>状态</th>
                <th>最后使用</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in tokens" :key="item.id">
                <td>
                  <strong>{{ item.name }}</strong>
                  <small v-if="item.lastError">{{ item.lastError }}</small>
                </td>
                <td>{{ item.provider }}</td>
                <td class="mono">{{ maskToken(item.tokenValue) }}</td>
                <td>{{ item.remaining }}%</td>
                <td><span :class="['tag', statusClass(item.status)]">{{ statusLabel(item.status) }}</span></td>
                <td>{{ formatTime(item.lastUsedAt) }}</td>
                <td>
                  <div class="row-actions">
                    <button type="button" class="ghost-button" :disabled="validatingIds[item.id]" @click="verifyToken(item)">
                      {{ validatingIds[item.id] ? '验证中' : '验证' }}
                    </button>
                    <button type="button" class="ghost-button" @click="openEditForm(item)">编辑</button>
                    <button type="button" class="danger-button" @click="removeToken(item)">删除</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
          <div v-if="!tokens.length" class="empty">暂无账号，添加后即可接入本地代理</div>
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
            <small>{{ entry.status || '-' }}</small>
            <small>{{ entry.durationMs || 0 }}ms</small>
            <small>{{ entry.tokenName || '-' }}</small>
            <time>{{ formatTime(entry.time) }}</time>
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
            <span>上游 API Base URL</span>
            <input v-model="config.upstreamBaseUrl" type="url" />
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

      <div v-if="form.visible" class="modal-backdrop" @click.self="closeForm">
        <form class="modal" @submit.prevent="submitForm">
          <div class="section-heading">
            <div>
              <h2>{{ form.editingId ? '编辑账号' : '添加账号' }}</h2>
              <p>账号名称必填且不可重复</p>
            </div>
            <button type="button" class="icon-button" @click="closeForm">×</button>
          </div>
          <label>
            <span>账号名称</span>
            <input v-model="form.name" autofocus />
          </label>
          <label>
            <span>厂商</span>
            <select v-model="form.provider">
              <option value="openai">OpenAI</option>
              <option value="anthropic">Anthropic</option>
              <option value="codex">Codex</option>
              <option value="custom">Custom</option>
            </select>
          </label>
          <label>
            <span>Token</span>
            <textarea v-model="form.tokenValue" rows="4"></textarea>
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
