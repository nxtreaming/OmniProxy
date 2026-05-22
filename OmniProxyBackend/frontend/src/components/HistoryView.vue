<script setup>
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { Download, Refresh, View } from '@element-plus/icons-vue'
import GeminiSelect from './GeminiSelect.vue'

const props = defineProps({
  entries: {
    type: Array,
    default: () => [],
  },
  summary: {
    type: Object,
    default: null,
  },
  providers: {
    type: Array,
    default: () => [],
  },
  exporting: {
    type: String,
    default: '',
  },
  formatTime: {
    type: Function,
    required: true,
  },
  formatDuration: {
    type: Function,
    required: true,
  },
  providerLabel: {
    type: Function,
    required: true,
  },
  formatNumber: {
    type: Function,
    required: true,
  },
})

const emit = defineEmits(['refresh', 'export', 'diagnose'])

const filters = reactive({
  provider: 'all',
  client: 'all',
  level: 'all',
  status: 'all',
  model: '',
  token: '',
  search: '',
})
const historyPage = ref(1)
const historyPageSize = 200
const historyTrendWindowDays = 14
const modelPieColors = ['#2563eb', '#159a5b', '#e0a11a', '#c43b3b', '#7c3aed', '#0891b2']
let filterRefreshTimer = null

const filteredHistory = computed(() => filterHistory(props.entries, filters))
const localHistorySummary = computed(() => summarizeHistory(filteredHistory.value))
const historySummary = computed(() => normalizeHistorySummary(props.summary) || localHistorySummary.value)
const historyDailyRows = computed(() => {
  if (props.summary) return normalizeDailyRows(props.summary.dailyRows)
  return buildHistoryDailyWindow(aggregateHistoryByDay(filteredHistory.value), historyTrendWindowDays)
})
const historyTrendMax = computed(() => Math.max(1, ...historyDailyRows.value.map((row) => row.totalTokens)))
const historyTrendColumns = computed(() => `repeat(${Math.max(1, historyDailyRows.value.length)}, minmax(0, 1fr))`)
const activeHistoryTrendTooltip = ref(null)
const providerRanks = computed(() => {
  if (props.summary) return normalizeRanks(props.summary.providerRanks, '-').map((item) => ({
    ...item,
    label: item.label ? props.providerLabel(item.label) : '-',
  })).slice(0, 5)
  return rankHistory(filteredHistory.value, (entry) => props.providerLabel(entry.provider) || '-', 'count').slice(0, 5)
})
const clientOptions = computed(() => historyClientOptions(props.entries))
const providerFilterOptions = computed(() => [
  { value: 'all', label: '全部厂商' },
  ...props.providers.map((provider) => ({ value: provider.key, label: provider.label })),
])
const clientFilterOptions = computed(() => [
  { value: 'all', label: '全部工具' },
  ...clientOptions.value.map((client) => ({ value: client.key, label: client.label })),
])
const levelFilterOptions = [
  { value: 'all', label: '全部级别' },
  { value: 'info', label: '正常' },
  { value: 'warn', label: '警告' },
  { value: 'error', label: '错误' },
]
const statusFilterOptions = [
  { value: 'all', label: '全部状态' },
  { value: 'success', label: '成功' },
  { value: 'error', label: '失败' },
  { value: '429', label: '429' },
  { value: '500', label: '500' },
  { value: '502', label: '502' },
  { value: '503', label: '503' },
  { value: '504', label: '504' },
]
const clientRanks = computed(() => {
  if (props.summary) return normalizeRanks(props.summary.clientRanks, '未记录工具', normalizeClientRankLabel).slice(0, 5)
  return rankHistory(filteredHistory.value, (entry) => clientLabel(entry), 'count').slice(0, 5)
})
const allModelRanks = computed(() => {
  if (props.summary) return normalizeRanks(props.summary.modelRanks, '未记录模型')
  return rankHistory(filteredHistory.value, (entry) => entry.model || entry.protocol || '未记录模型', 'totalTokens')
})
const modelRanks = computed(() =>
  allModelRanks.value.slice(0, 5),
)
const modelPieSegments = computed(() => buildModelPieSegments(allModelRanks.value))
const modelPieTotal = computed(() =>
  modelPieSegments.value.reduce((sum, item) => sum + Number(item.totalTokens || 0), 0),
)
const modelPieTotalLabel = computed(() => compactMetricNumber(modelPieTotal.value))
const modelPieGradient = computed(() => {
  const total = modelPieTotal.value
  if (total <= 0 || !modelPieSegments.value.length) return ''

  let cursor = 0
  const parts = modelPieSegments.value.map((item, index) => {
    const start = cursor
    const end = index === modelPieSegments.value.length - 1
      ? 360
      : cursor + (Number(item.totalTokens || 0) / total) * 360
    cursor = end
    return `${item.color} ${start.toFixed(2)}deg ${end.toFixed(2)}deg`
  })
  return `conic-gradient(${parts.join(', ')})`
})
const tokenFailureRanks = computed(() => {
  if (props.summary) return normalizeRanks(props.summary.tokenFailureRanks, '未记录账号').slice(0, 5)
  return rankHistory(
    filteredHistory.value.filter(isFailedHistory),
    (entry) => entry.tokenName || '未记录账号',
    'count',
  ).slice(0, 5)
})
const failureReasonRanks = computed(() => {
  if (props.summary) return normalizeRanks(props.summary.failureReasonRanks, '无状态码', normalizeFailureReasonLabel).slice(0, 5)
  return rankHistory(
    filteredHistory.value.filter(isFailedHistory),
    (entry) => failureReasonLabel(entry),
    'count',
  ).slice(0, 5)
})
const historyTotalPages = computed(() => Math.max(1, Math.ceil(filteredHistory.value.length / historyPageSize)))
const pagedHistory = computed(() => {
  const page = Math.min(historyPage.value, historyTotalPages.value)
  const start = (page - 1) * historyPageSize
  return filteredHistory.value.slice(start, start + historyPageSize)
})

watch(() => props.entries, () => {
  historyPage.value = 1
})

watch(filters, () => {
  historyPage.value = 1
  if (filterRefreshTimer) {
    window.clearTimeout(filterRefreshTimer)
  }
  filterRefreshTimer = window.setTimeout(() => {
    filterRefreshTimer = null
    emit('refresh', { ...filters })
  }, 250)
}, { deep: true })

function exportRequestHistory(format) {
  emit('export', {
    format,
    filters: { ...filters },
    entries: filteredHistory.value,
  })
}

function openHistoryDiagnosis(entry) {
  emit('diagnose', entry)
}

function normalizeHistorySummary(summary) {
  if (!summary) return null
  const total = Number(summary.total || 0)
  const failed = Number(summary.failed || 0)
  const totalTokens = Number(summary.totalTokens || 0)
  const averageDuration = Number(summary.averageDuration || 0)
  return {
    total,
    failed,
    failureRate: total ? Math.round((failed / total) * 100) : 0,
    totalTokens,
    averageDuration,
  }
}

function normalizeDailyRows(rows) {
  return [...(rows || [])].map((row) => ({
    date: row.date,
    requestCount: Number(row.requestCount || 0),
    failedCount: Number(row.failedCount || 0),
    totalTokens: Number(row.totalTokens || 0),
  }))
}

function normalizeRanks(rows, fallbackLabel, labelFn = (value) => value) {
  return [...(rows || [])].map((row) => {
    const rawLabel = String(row.label || '').trim()
    return {
      label: labelFn(rawLabel || fallbackLabel),
      count: Number(row.count || 0),
      totalTokens: Number(row.totalTokens || 0),
      failedCount: Number(row.failedCount || 0),
    }
  })
}

function normalizeClientRankLabel(label) {
  return label === '__status_check__' ? '状态检查' : label
}

function normalizeFailureReasonLabel(label) {
  return label
    .replaceAll('__no_status__', '无状态码')
    .replaceAll('__reason_sep__', ' · ')
}

function filterHistory(items, currentFilters) {
  const search = currentFilters.search.trim().toLowerCase()
  const model = currentFilters.model.trim().toLowerCase()
  const tokenName = currentFilters.token.trim().toLowerCase()
  return items
    .filter((item) => currentFilters.provider === 'all' || item.provider === currentFilters.provider)
    .filter((item) => currentFilters.client === 'all' || item.clientKey === currentFilters.client)
    .filter((item) => currentFilters.level === 'all' || item.level === currentFilters.level)
    .filter((item) => currentFilters.status === 'all' || historyStatusMatches(item, currentFilters.status))
    .filter((item) => !model || String(item.model || '').toLowerCase().includes(model))
    .filter((item) => !tokenName || String(item.tokenName || '').toLowerCase().includes(tokenName))
    .filter((item) => {
      if (!search) return true
      return [
        item.method,
        item.path,
        item.provider,
        item.protocol,
        item.clientKey,
        item.clientName,
        item.model,
        item.tokenName,
        item.message,
        String(item.status || ''),
      ]
        .join(' ')
        .toLowerCase()
        .includes(search)
    })
}

function summarizeHistory(items) {
  const total = items.length
  const failed = items.filter(isFailedHistory).length
  const totalTokens = items.reduce((sum, item) => sum + Number(item.totalTokens || 0), 0)
  const totalDuration = items.reduce((sum, item) => sum + Number(item.durationMs || 0), 0)
  return {
    total,
    failed,
    failureRate: total ? Math.round((failed / total) * 100) : 0,
    totalTokens,
    averageDuration: total ? Math.round(totalDuration / total) : 0,
  }
}

function aggregateHistoryByDay(items) {
  const byDay = new Map()
  for (const entry of items) {
    const day = String(entry.time || '').slice(0, 10) || 'unknown'
    const current = byDay.get(day) || { date: day, requestCount: 0, failedCount: 0, totalTokens: 0 }
    current.requestCount += 1
    if (isFailedHistory(entry)) current.failedCount += 1
    current.totalTokens += Number(entry.totalTokens || 0)
    byDay.set(day, current)
  }
  return [...byDay.values()].sort((a, b) => a.date.localeCompare(b.date))
}

function buildHistoryDailyWindow(rows, days) {
  if (!rows.length) return []

  const byDay = new Map(rows.map((row) => [row.date, row]))
  const endDate = parseDateKey(rows[rows.length - 1]?.date) || new Date()
  const startDate = addDays(endDate, -(days - 1))

  return Array.from({ length: days }, (_, index) => {
    const date = formatDateKey(addDays(startDate, index))
    return byDay.get(date) || { date, requestCount: 0, failedCount: 0, totalTokens: 0 }
  })
}

function parseDateKey(value) {
  const match = String(value || '').match(/^(\d{4})-(\d{2})-(\d{2})$/)
  if (!match) return null
  return new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]))
}

function addDays(date, amount) {
  const next = new Date(date)
  next.setDate(next.getDate() + amount)
  return next
}

function formatDateKey(date) {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function rankHistory(items, labelFn, mode) {
  const groups = new Map()
  for (const entry of items) {
    const label = labelFn(entry)
    const current = groups.get(label) || { label, count: 0, totalTokens: 0, failedCount: 0 }
    current.count += 1
    current.totalTokens += Number(entry.totalTokens || 0)
    if (isFailedHistory(entry)) current.failedCount += 1
    groups.set(label, current)
  }
  const metric = mode === 'totalTokens' ? 'totalTokens' : 'count'
  return [...groups.values()].sort((a, b) => b[metric] - a[metric] || b.count - a.count)
}

function historyClientOptions(items) {
  const byKey = new Map()
  for (const entry of items || []) {
    const key = String(entry.clientKey || '').trim()
    if (!key) continue
    if (!byKey.has(key)) {
      byKey.set(key, clientLabel(entry))
    }
  }
  return [...byKey.entries()]
    .map(([key, label]) => ({ key, label }))
    .sort((a, b) => a.label.localeCompare(b.label))
}

function clientLabel(entry) {
  if (isStatusCheckEntry(entry)) return '状态检查'
  return entry?.clientName || entry?.clientKey || '未记录工具'
}

function isStatusCheckEntry(entry) {
  return (
    entry?.method === 'CHECK' ||
    entry?.protocol === 'health-check' ||
    String(entry?.path || '').includes('/maintenance/token-health-check')
  )
}

function buildModelPieSegments(rows) {
  const ranked = rows.filter((item) => Number(item.totalTokens || 0) > 0)
  const top = ranked.slice(0, 5)
  const rest = ranked.slice(5)
  const restTotal = rest.reduce((sum, item) => sum + Number(item.totalTokens || 0), 0)
  const restCount = rest.reduce((sum, item) => sum + Number(item.count || 0), 0)
  const segments = restTotal > 0
    ? [...top, { label: '其他模型', count: restCount, totalTokens: restTotal, failedCount: 0 }]
    : top
  const total = segments.reduce((sum, item) => sum + Number(item.totalTokens || 0), 0)

  return segments.map((item, index) => ({
    ...item,
    color: modelPieColors[index % modelPieColors.length],
    percent: total ? Math.round((Number(item.totalTokens || 0) / total) * 1000) / 10 : 0,
  }))
}

function compactMetricNumber(value) {
  const number = Number(value || 0)
  if (number >= 100000000) return `${(number / 100000000).toFixed(number >= 1000000000 ? 1 : 2)}亿`
  if (number >= 10000) return `${(number / 10000).toFixed(number >= 1000000 ? 1 : 2)}万`
  return props.formatNumber(Math.round(number))
}

function failureReasonLabel(entry) {
  const status = entry.status ? `${entry.status}` : '无状态码'
  const message = String(entry.message || '').trim()
  if (!message) return status
  return `${status} · ${message}`
}

function historyTrendHeight(row) {
  const value = Number(row.totalTokens || 0)
  if (value <= 0) return '4%'
  return `${Math.max(8, Math.round((value / historyTrendMax.value) * 100))}%`
}

function historyTrendTooltipPosition(event) {
  const target = event?.currentTarget
  const rect = target?.getBoundingClientRect?.()
  const rawX = rect ? rect.left + rect.width / 2 : event?.clientX || 0
  const rawY = rect ? rect.top + rect.height / 2 : event?.clientY || 0
  const viewportWidth = typeof window === 'undefined' ? 1280 : window.innerWidth
  const tooltipWidth = 260
  const margin = 16
  const x = Math.min(
    Math.max(rawX, tooltipWidth / 2 + margin),
    Math.max(tooltipWidth / 2 + margin, viewportWidth - tooltipWidth / 2 - margin),
  )

  return {
    x,
    y: rawY,
    placement: rawY < 180 ? 'below' : 'above',
  }
}

function historyTrendTooltipData(row) {
  const requestCount = Number(row.requestCount || 0)
  const failedCount = Number(row.failedCount || 0)
  return {
    key: row.date,
    date: row.date,
    title: '每日用量',
    value: Number(row.totalTokens || 0),
    valueUnit: 'Token',
    requestCount,
    failedCount,
    successCount: Math.max(0, requestCount - failedCount),
    statusText: requestCount ? '当天有请求记录' : '当天暂无请求',
  }
}

function showHistoryTrendTooltip(row, event) {
  activeHistoryTrendTooltip.value = {
    ...historyTrendTooltipData(row),
    ...historyTrendTooltipPosition(event),
  }
}

function moveHistoryTrendTooltip(row, event) {
  if (!activeHistoryTrendTooltip.value || activeHistoryTrendTooltip.value.key !== row.date) return
  activeHistoryTrendTooltip.value = {
    ...activeHistoryTrendTooltip.value,
    ...historyTrendTooltipPosition(event),
  }
}

function hideHistoryTrendTooltip() {
  activeHistoryTrendTooltip.value = null
}

function isHistoryTrendTooltipActive(row) {
  return activeHistoryTrendTooltip.value?.key === row.date
}

onBeforeUnmount(() => {
  if (filterRefreshTimer) {
    window.clearTimeout(filterRefreshTimer)
    filterRefreshTimer = null
  }
  hideHistoryTrendTooltip()
})

function historyDate(value) {
  if (!value) return '-'
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return '-'
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(parsed)
}

function historyClock(value) {
  if (!value) return '-'
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return '-'
  return new Intl.DateTimeFormat('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  }).format(parsed)
}

function previousHistoryPage() {
  historyPage.value = Math.max(1, historyPage.value - 1)
}

function nextHistoryPage() {
  historyPage.value = Math.min(historyTotalPages.value, historyPage.value + 1)
}

function historyStatusMatches(entry, status) {
  if (status === 'success') {
    return entry.status >= 200 && entry.status < 400
  }
  if (status === 'error') {
    return !entry.status || entry.status >= 400
  }
  return String(entry.status || '') === status
}

function historyStatusLabel(entry) {
  if (!entry.status) return '-'
  if (entry.status >= 200 && entry.status < 400) return `${entry.status}`
  return `${entry.status}`
}

function historyTagClass(entry) {
  if (entry.level === 'error') return 'danger'
  if (entry.level === 'warn') return 'warning'
  return 'success'
}

function historyUsageTotal(entry) {
  const total = Number(entry.totalTokens || 0)
  if (total <= 0) return '-'
  return props.formatNumber(total)
}

function historyUsageDetail(entry) {
  const total = Number(entry.totalTokens || 0)
  if (total <= 0) return ''
  return `入 ${props.formatNumber(entry.inputTokens)} · 出 ${props.formatNumber(entry.outputTokens)}`
}

function isFailedHistory(entry) {
  return entry?.level === 'error' || entry?.level === 'warn' || Number(entry?.status || 0) >= 400
}
</script>

<template>
  <section class="panel history-page-panel">
    <div class="section-heading">
      <div>
        <h2>请求记录分析</h2>
        <p>筛选代理请求、账号验证、额度刷新、重试结果、耗时和 Token 用量</p>
      </div>
      <div class="section-actions">
        <el-button :icon="Refresh" @click="emit('refresh', { ...filters })">刷新</el-button>
        <el-button :icon="Download" :loading="exporting === 'csv'" @click="exportRequestHistory('csv')">
          导出 CSV
        </el-button>
        <el-button :icon="Download" :loading="exporting === 'json'" @click="exportRequestHistory('json')">
          导出 JSON
        </el-button>
      </div>
    </div>

    <div class="history-filters">
      <label>
        <span>厂商</span>
        <GeminiSelect v-model="filters.provider" :options="providerFilterOptions" aria-label="筛选厂商" />
      </label>
      <label>
        <span>工具</span>
        <GeminiSelect v-model="filters.client" :options="clientFilterOptions" aria-label="筛选工具" />
      </label>
      <label>
        <span>级别</span>
        <GeminiSelect v-model="filters.level" :options="levelFilterOptions" aria-label="筛选级别" />
      </label>
      <label>
        <span>状态</span>
        <GeminiSelect v-model="filters.status" :options="statusFilterOptions" aria-label="筛选状态" />
      </label>
      <label>
        <span>模型</span>
        <input v-model="filters.model" type="search" placeholder="模型名称" />
      </label>
      <label>
        <span>账号</span>
        <input v-model="filters.token" type="search" placeholder="账号名称" />
      </label>
      <label class="history-search">
        <span>搜索</span>
        <input v-model="filters.search" type="search" placeholder="模型、账号、路径或状态码" />
      </label>
    </div>

    <div class="history-summary-grid">
      <div>
        <span>请求数</span>
        <strong>{{ formatNumber(historySummary.total) }}</strong>
      </div>
      <div>
        <span>失败率</span>
        <strong>{{ historySummary.failureRate }}%</strong>
        <small>{{ formatNumber(historySummary.failed) }} 次失败</small>
      </div>
      <div>
        <span>Token</span>
        <strong>{{ formatNumber(historySummary.totalTokens) }}</strong>
      </div>
      <div>
        <span>平均耗时</span>
        <strong>{{ formatDuration(historySummary.averageDuration) }}</strong>
      </div>
    </div>

    <div class="history-insights">
      <div class="history-insight-panel history-trend-panel">
        <div class="history-insight-head">
          <span>每日用量 · 近 {{ historyTrendWindowDays }} 天</span>
          <strong>{{ formatNumber(historySummary.totalTokens) }}</strong>
        </div>
        <div v-if="historyDailyRows.length" class="usage-trend compact-history-trend" :style="{ gridTemplateColumns: historyTrendColumns }">
          <div
            v-for="row in historyDailyRows"
            :key="row.date"
            :class="['trend-column', { active: isHistoryTrendTooltipActive(row) }]"
            role="img"
            tabindex="0"
            :aria-label="`${row.date} · ${formatNumber(row.totalTokens)} Token · ${formatNumber(row.requestCount)} 次请求`"
            :aria-describedby="isHistoryTrendTooltipActive(row) ? 'history-trend-tooltip' : undefined"
            @mouseenter="showHistoryTrendTooltip(row, $event)"
            @mousemove="moveHistoryTrendTooltip(row, $event)"
            @mouseleave="hideHistoryTrendTooltip"
            @focus="showHistoryTrendTooltip(row, $event)"
            @blur="hideHistoryTrendTooltip"
          >
            <div class="trend-bar">
              <span
                :class="{ empty: Number(row.totalTokens || 0) <= 0 }"
                :style="{ height: historyTrendHeight(row) }"
              ></span>
            </div>
            <small>{{ row.date.slice(5) }}</small>
          </div>
        </div>
        <div v-else class="empty compact-empty">暂无趋势数据</div>
        <Teleport to="body">
          <Transition name="trend-tooltip-fade">
            <div
              v-if="activeHistoryTrendTooltip"
              id="history-trend-tooltip"
              class="trend-tooltip history-trend-tooltip"
              :class="{ below: activeHistoryTrendTooltip.placement === 'below' }"
              :style="{ left: `${activeHistoryTrendTooltip.x}px`, top: `${activeHistoryTrendTooltip.y}px` }"
              role="tooltip"
            >
              <div class="trend-tooltip-head">
                <span>{{ activeHistoryTrendTooltip.date }}</span>
                <strong>{{ activeHistoryTrendTooltip.title }}</strong>
              </div>
              <div class="trend-tooltip-primary">
                <strong>{{ formatNumber(activeHistoryTrendTooltip.value) }}</strong>
                <span>{{ activeHistoryTrendTooltip.valueUnit }}</span>
              </div>
              <div class="trend-tooltip-grid">
                <span>请求 <strong>{{ formatNumber(activeHistoryTrendTooltip.requestCount) }}</strong></span>
                <span>成功 <strong>{{ formatNumber(activeHistoryTrendTooltip.successCount) }}</strong></span>
                <span>失败 <strong>{{ formatNumber(activeHistoryTrendTooltip.failedCount) }}</strong></span>
              </div>
              <p>{{ activeHistoryTrendTooltip.statusText }}</p>
            </div>
          </Transition>
        </Teleport>
      </div>

      <div class="history-insight-panel model-insight-panel">
        <div class="history-insight-head">
          <span>模型消耗</span>
          <strong>{{ modelRanks.length }}</strong>
        </div>
        <div v-if="modelPieSegments.length" class="model-pie-layout">
          <div
            class="model-pie-chart"
            :style="{ background: modelPieGradient }"
            role="img"
            :aria-label="`模型 Token 消耗占比，共 ${formatNumber(modelPieTotal)} Token`"
          >
            <div>
              <span :title="`${formatNumber(modelPieTotal)} Token`">{{ modelPieTotalLabel }}</span>
              <small>Token</small>
            </div>
          </div>
          <div class="model-pie-legend">
            <div
              v-for="item in modelPieSegments"
              :key="item.label"
              class="model-pie-item"
              :title="`${item.label} · ${formatNumber(item.totalTokens)} Token · ${item.percent}%`"
            >
              <i :style="{ background: item.color }"></i>
              <span>{{ item.label }}</span>
              <strong>{{ item.percent }}%</strong>
            </div>
          </div>
        </div>
        <div class="rank-list">
          <div v-for="item in modelRanks" :key="item.label" class="rank-row">
            <span :title="item.label">{{ item.label }}</span>
            <strong>{{ formatNumber(item.totalTokens) }}</strong>
          </div>
          <div v-if="!modelRanks.length" class="empty compact-empty">暂无模型数据</div>
        </div>
      </div>

      <div class="history-insight-panel">
        <div class="history-insight-head">
          <span>厂商分布</span>
          <strong>{{ providerRanks.length }}</strong>
        </div>
        <div class="rank-list">
          <div v-for="item in providerRanks" :key="item.label" class="rank-row">
            <span :title="item.label">{{ item.label }}</span>
            <strong>{{ formatNumber(item.count) }} 次</strong>
          </div>
          <div v-if="!providerRanks.length" class="empty compact-empty">暂无厂商数据</div>
        </div>
      </div>

      <div class="history-insight-panel">
        <div class="history-insight-head">
          <span>工具分布</span>
          <strong>{{ clientRanks.length }}</strong>
        </div>
        <div class="rank-list">
          <div v-for="item in clientRanks" :key="item.label" class="rank-row">
            <span :title="item.label">{{ item.label }}</span>
            <strong>{{ formatNumber(item.count) }} 次</strong>
          </div>
          <div v-if="!clientRanks.length" class="empty compact-empty">暂无工具数据</div>
        </div>
      </div>

      <div class="history-insight-panel">
        <div class="history-insight-head">
          <span>失败账号</span>
          <strong>{{ tokenFailureRanks.length }}</strong>
        </div>
        <div class="rank-list">
          <div v-for="item in tokenFailureRanks" :key="item.label" class="rank-row">
            <span :title="item.label">{{ item.label }}</span>
            <strong>{{ formatNumber(item.count) }} 次</strong>
          </div>
          <div v-if="!tokenFailureRanks.length" class="empty compact-empty">暂无失败账号</div>
        </div>
      </div>

      <div class="history-insight-panel wide-history-panel">
        <div class="history-insight-head">
          <span>失败原因</span>
          <strong>{{ failureReasonRanks.length }}</strong>
        </div>
        <div class="rank-list">
          <div v-for="item in failureReasonRanks" :key="item.label" class="rank-row">
            <span :title="item.label">{{ item.label }}</span>
            <strong>{{ formatNumber(item.count) }} 次</strong>
          </div>
          <div v-if="!failureReasonRanks.length" class="empty compact-empty">暂无失败原因</div>
        </div>
      </div>
    </div>

    <div class="table-wrap history-table-wrap">
      <div class="history-table-scroll">
        <table class="account-table history-table">
          <colgroup>
            <col class="history-col-time" />
            <col class="history-col-route" />
            <col class="history-col-client" />
            <col class="history-col-token" />
            <col class="history-col-status" />
            <col class="history-col-duration" />
            <col class="history-col-usage" />
            <col class="history-col-path" />
            <col class="history-col-actions" />
          </colgroup>
          <thead>
            <tr>
              <th>时间</th>
              <th>厂商 / 模型</th>
              <th>工具</th>
              <th>账号</th>
              <th>状态</th>
              <th>耗时</th>
              <th>Token</th>
              <th>路径</th>
              <th>详情</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="entry in pagedHistory"
              :key="entry.id"
              class="clickable-history-row"
              @click="openHistoryDiagnosis(entry)"
            >
              <td>
                <time class="history-time-cell" :datetime="entry.time">
                  <span>{{ historyDate(entry.time) }}</span>
                  <small>{{ historyClock(entry.time) }}</small>
                </time>
              </td>
              <td>
                <strong>{{ providerLabel(entry.provider) }}</strong>
                <small>{{ entry.model || entry.protocol || '-' }}</small>
              </td>
              <td :title="clientLabel(entry)">{{ clientLabel(entry) }}</td>
              <td :title="entry.tokenName || '-'">{{ entry.tokenName || '-' }}</td>
              <td>
                <span :class="['tag', historyTagClass(entry)]">{{ historyStatusLabel(entry) }}</span>
                <small :title="entry.message">{{ entry.message }}</small>
              </td>
              <td>{{ formatDuration(entry.durationMs) }}</td>
              <td>
                <strong>{{ historyUsageTotal(entry) }}</strong>
                <small v-if="historyUsageDetail(entry)">{{ historyUsageDetail(entry) }}</small>
              </td>
              <td class="mono" :title="`${entry.method} ${entry.path}`">{{ entry.method }} {{ entry.path }}</td>
              <td>
                <el-button
                  size="small"
                  :icon="View"
                  @click.stop="openHistoryDiagnosis(entry)"
                >
                  {{ isFailedHistory(entry) ? '诊断' : '详情' }}
                </el-button>
              </td>
            </tr>
          </tbody>
        </table>
        <div v-if="!filteredHistory.length" class="empty">暂无匹配的请求历史</div>
      </div>
      <div v-if="filteredHistory.length" class="history-pagination">
        <span>共 {{ formatNumber(filteredHistory.length) }} 条，每页 {{ historyPageSize }} 条</span>
        <div>
          <el-button size="small" :disabled="historyPage <= 1" @click="previousHistoryPage">上一页</el-button>
          <strong>{{ historyPage }} / {{ historyTotalPages }}</strong>
          <el-button size="small" :disabled="historyPage >= historyTotalPages" @click="nextHistoryPage">下一页</el-button>
        </div>
      </div>
    </div>
  </section>
</template>
