<script setup>
import { computed } from 'vue'
import {
  Clock,
  Download,
  FolderOpened,
  InfoFilled,
  Link as LinkIcon,
  Monitor,
  RefreshRight,
  Setting,
} from '@element-plus/icons-vue'

const props = defineProps({
  appInfo: {
    type: Object,
    default: () => ({}),
  },
  config: {
    type: Object,
    required: true,
  },
  dataDirectory: {
    type: Object,
    required: true,
  },
  proxyStatus: {
    type: Object,
    required: true,
  },
  autoStartEnabled: {
    type: Boolean,
    required: true,
  },
  updateChecking: {
    type: Boolean,
    required: true,
  },
  updateInfo: {
    type: Object,
    default: null,
  },
  updateCheckedAt: {
    type: String,
    default: '',
  },
  formatTime: {
    type: Function,
    default: null,
  },
})

defineEmits(['manual-check-for-updates', 'open-url'])

const currentVersion = computed(
  () => props.appInfo?.version || props.updateInfo?.currentVersion || 'dev',
)
const isDevelopmentBuild = computed(() => {
  const normalizedVersion = String(currentVersion.value || '').trim().toLowerCase()
  return Boolean(props.appInfo?.isDevelopment) || normalizedVersion === 'dev' || normalizedVersion === 'development'
})
const releaseUrl = computed(() => props.updateInfo?.downloadUrl || props.updateInfo?.releaseUrl || '')
const releaseActionLabel = computed(() => (props.updateInfo?.updateAvailable ? '获取更新' : '打开发布页'))
const releaseActionIcon = computed(() => (props.updateInfo?.updateAvailable ? Download : LinkIcon))
const updateBadge = computed(() => {
  if (isDevelopmentBuild.value) return { type: 'info', label: '开发版本' }
  if (!props.updateInfo) return { type: 'info', label: '未检查' }
  if (props.updateInfo.updateAvailable) return { type: 'warning', label: '有新版本' }
  return { type: 'success', label: '已是最新' }
})
const updateTitle = computed(() => {
  if (isDevelopmentBuild.value) return '开发版本跳过远端更新检测'
  if (!props.updateInfo) return '尚未进行手动检查'
  if (props.updateInfo.updateAvailable) {
    return `发现新版本 ${props.updateInfo.latestVersion || ''}`.trim()
  }
  return `当前已是最新版本 ${props.updateInfo.currentVersion || currentVersion.value}`.trim()
})
const updateDescription = computed(() => {
  if (isDevelopmentBuild.value) return '当前构建用于开发验证，不参与正式发布版本比较。'
  if (!props.updateInfo) return '启动后会自动检查一次；也可以在这里立即检查。'
  if (props.updateInfo.updateAvailable) {
    return `当前版本 ${props.updateInfo.currentVersion || currentVersion.value}，最新版本 ${props.updateInfo.latestVersion || '-'}`
  }
  return '未发现可用更新。'
})
const releaseNotesPreview = computed(() => {
  const text = String(props.updateInfo?.body || '').trim()
  if (!text) return ''
  return text.length > 420 ? `${text.slice(0, 420)}...` : text
})
const serviceRows = computed(() => [
  {
    label: '代理服务',
    value: props.proxyStatus.running ? '运行中' : '已停止',
    detail: `端口 :${props.proxyStatus.port || props.config.proxyPort}`,
  },
  {
    label: '控制 API',
    value: `:${props.config.controlPort}`,
    detail: '仅监听本机地址',
  },
  {
    label: '开机自启',
    value: props.autoStartEnabled ? '已开启' : '未开启',
    detail: '启动参数 --minimized',
  },
  {
    label: 'WebSocket',
    value: props.config.websocketMode === 'enabled' ? '已启用' : '已停用',
    detail: 'Codex 流式代理',
  },
])
const configRows = computed(() => [
  {
    label: '调度模式',
    value: schedulingModeLabel(props.config.schedulingMode),
  },
  {
    label: '切换阈值',
    value: `${props.config.switchThreshold}%`,
  },
  {
    label: '自动重试',
    value: `${props.config.maxRetries} 次`,
  },
  {
    label: '更新源',
    value: props.appInfo?.updateEndpoint || '-',
    mono: true,
  },
])

function schedulingModeLabel(value) {
  const labels = {
    queue: '队列模式',
    balanced: '优先平衡使用',
  }
  return labels[value] || value || '-'
}

function dataSourceLabel(value) {
  const labels = {
    env: '环境变量',
    bootstrap: '指针文件',
    legacy: '旧版目录',
    unconfigured: '默认未配置',
    unknown: '未知',
  }
  return labels[value] || value || '-'
}

function formatDate(value) {
  if (!value) return '-'
  if (props.formatTime) {
    return props.formatTime(value)
  }
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString()
}
</script>

<template>
  <section class="about-page">
    <div class="about-page-column">
      <article class="panel about-version-panel">
        <div class="section-heading">
          <div>
            <h2>应用版本</h2>
            <p>当前客户端、运行环境和更新状态</p>
          </div>
          <el-tag :type="updateBadge.type" effect="light" round>{{ updateBadge.label }}</el-tag>
        </div>

        <div class="version-hero">
          <div>
            <span>当前版本</span>
            <strong>{{ currentVersion }}</strong>
            <small>{{ appInfo.isDevelopment ? '开发构建' : '发布构建' }}</small>
          </div>
          <div class="about-actions">
            <el-button
              type="primary"
              :icon="RefreshRight"
              :disabled="isDevelopmentBuild"
              :loading="updateChecking"
              @click="$emit('manual-check-for-updates')"
            >
              {{ isDevelopmentBuild ? '开发版跳过更新' : updateChecking ? '检查中' : '检查更新' }}
            </el-button>
            <el-button
              v-if="releaseUrl"
              :icon="releaseActionIcon"
              @click="$emit('open-url', releaseUrl)"
            >
              {{ releaseActionLabel }}
            </el-button>
          </div>
        </div>

        <div :class="['update-status-box', updateInfo?.updateAvailable ? 'warning' : '']">
          <InfoFilled class="about-status-icon" aria-hidden="true" />
          <div>
            <strong>{{ updateTitle }}</strong>
            <p>{{ updateDescription }}</p>
            <small v-if="updateCheckedAt">上次检查 {{ formatDate(updateCheckedAt) }}</small>
          </div>
        </div>

        <div v-if="releaseNotesPreview" class="release-notes">
          <span>版本说明</span>
          <p>{{ releaseNotesPreview }}</p>
        </div>
      </article>

      <article class="panel about-info-panel">
        <div class="section-heading">
          <div>
            <h2>数据与配置</h2>
            <p>当前数据目录、启动来源和关键策略</p>
          </div>
        </div>
        <div class="about-path-row">
          <FolderOpened class="about-inline-icon" aria-hidden="true" />
          <span>数据目录</span>
          <strong class="mono">{{ dataDirectory.dataDir || '未加载' }}</strong>
          <small v-if="dataDirectory.pendingDataDir && dataDirectory.restartRequired">
            重启后使用：{{ dataDirectory.pendingDataDir }}
          </small>
          <small v-else>来源：{{ dataSourceLabel(dataDirectory.source) }}</small>
        </div>
        <div class="about-row-list compact">
          <div v-for="row in configRows" :key="row.label">
            <span>{{ row.label }}</span>
            <strong :class="{ mono: row.mono }">{{ row.value }}</strong>
          </div>
        </div>
        <div v-if="appInfo.updateEndpoint" class="about-footer-link">
          <el-button link type="primary" :icon="LinkIcon" @click="$emit('open-url', appInfo.updateEndpoint)">
            打开更新源
          </el-button>
        </div>
      </article>
    </div>

    <div class="about-page-column">
      <article class="panel about-info-panel">
        <div class="section-heading">
          <div>
            <h2>运行信息</h2>
            <p>桌面进程与构建环境</p>
          </div>
        </div>
        <div class="about-info-grid">
          <div>
            <Monitor class="about-info-icon" aria-hidden="true" />
            <span>平台</span>
            <strong>{{ appInfo.platform || '-' }}</strong>
          </div>
          <div>
            <Setting class="about-info-icon" aria-hidden="true" />
            <span>Go 版本</span>
            <strong>{{ appInfo.goVersion || '-' }}</strong>
          </div>
          <div>
            <Clock class="about-info-icon" aria-hidden="true" />
            <span>启动时间</span>
            <strong>{{ formatDate(appInfo.startedAt) }}</strong>
          </div>
        </div>
        <div class="about-path-row">
          <span>程序路径</span>
          <strong class="mono">{{ appInfo.executablePath || '-' }}</strong>
        </div>
      </article>

      <article class="panel about-info-panel">
        <div class="section-heading">
          <div>
            <h2>本机服务</h2>
            <p>代理、控制端口与后台启动状态</p>
          </div>
        </div>
        <div class="about-row-list">
          <div v-for="row in serviceRows" :key="row.label">
            <span>{{ row.label }}</span>
            <strong>{{ row.value }}</strong>
            <small>{{ row.detail }}</small>
          </div>
        </div>
      </article>
    </div>
  </section>
</template>
