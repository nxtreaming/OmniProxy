<script setup>
import { computed, reactive, ref, watch } from 'vue'
import { Download, Refresh, View } from '@element-plus/icons-vue'

const props = defineProps({
  entries: {
    type: Array,
    default: () => [],
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
  level: 'all',
  status: 'all',
  model: '',
  token: '',
  search: '',
})
const historyPage = ref(1)
const historyPageSize = 200
const modelPieColors = ['#2563eb', '#159a5b', '#e0a11a', '#c43b3b', '#7c3aed', '#0891b2']

const filteredHistory = computed(() => filterHistory(props.entries, filters))
const historySummary = computed(() => summarizeHistory(filteredHistory.value))
const historyDailyRows = computed(() => aggregateHistoryByDay(filteredHistory.value).slice(-14))
const historyTrendMax = computed(() => Math.max(1, ...historyDailyRows.value.map((row) => row.totalTokens)))
const historyTrendColumns = computed(() => `repeat(${Math.max(1, historyDailyRows.value.length)}, minmax(0, 1fr))`)
const providerRanks = computed(() =>
  rankHistory(filteredHistory.value, (entry) => props.providerLabel(entry.provider) || '-', 'count').slice(0, 5),
)
const allModelRanks = computed(() =>
  rankHistory(filteredHistory.value, (entry) => entry.model || entry.protocol || '未记录模型', 'totalTokens'),
)
const modelRanks = computed(() =>
  allModelRanks.value.slice(0, 5),
)
const modelPieSegments = computed(() => buildModelPieSegments(allModelRanks.value))
const modelPieTotal = computed(() =>
  modelPieSegments.value.reduce((sum, item) => sum + Number(item.totalTokens || 0), 0),
)
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
const tokenFailureRanks = computed(() =>
  rankHistory(
    filteredHistory.value.filter(isFailedHistory),
    (entry) => entry.tokenName || '未记录账号',
    'count',
  ).slice(0, 5),
)
const failureReasonRanks = computed(() =>
  rankHistory(
    filteredHistory.value.filter(isFailedHistory),
    (entry) => failureReasonLabel(entry),
    'count',
  ).slice(0, 5),
)
const historyTotalPages = computed(() => Math.max(1, Math.ceil(filteredHistory.value.length / historyPageSize)))
const pagedHistory = computed(() => {
  const page = Math.min(historyPage.value, historyTotalPages.value)
  const start = (page - 1) * historyPageSize
  return filteredHistory.value.slice(start, start + historyPageSize)
})

watch([() => props.entries, filters], () => {
  historyPage.value = 1
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

function filterHistory(items, currentFilters) {
  const search = currentFilters.search.trim().toLowerCase()
  const model = currentFilters.model.trim().toLowerCase()
  const tokenName = currentFilters.token.trim().toLowerCase()
  return items
    .filter((item) => currentFilters.provider === 'all' || item.provider === currentFilters.provider)
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
  <section class="panel">
    <div class="section-heading">
      <div>
        <h2>请求历史</h2>
        <p>持久化记录代理请求、账号验证、额度刷新、重试结果、耗时和 Token 用量</p>
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
        <select v-model="filters.provider">
          <option value="all">全部厂商</option>
          <option v-for="provider in providers" :key="provider.key" :value="provider.key">
            {{ provider.label }}
          </option>
        </select>
      </label>
      <label>
        <span>级别</span>
        <select v-model="filters.level">
          <option value="all">全部级别</option>
          <option value="info">正常</option>
          <option value="warn">警告</option>
          <option value="error">错误</option>
        </select>
      </label>
      <label>
        <span>状态</span>
        <select v-model="filters.status">
          <option value="all">全部状态</option>
          <option value="success">成功</option>
          <option value="error">失败</option>
          <option value="429">429</option>
          <option value="500">500</option>
          <option value="502">502</option>
          <option value="503">503</option>
          <option value="504">504</option>
        </select>
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
          <span>每日用量</span>
          <strong>{{ formatNumber(historySummary.totalTokens) }}</strong>
        </div>
        <div v-if="historyDailyRows.length" class="usage-trend compact-history-trend" :style="{ gridTemplateColumns: historyTrendColumns }">
          <div
            v-for="row in historyDailyRows"
            :key="row.date"
            class="trend-column"
            :title="`${row.date} · ${formatNumber(row.totalTokens)} Token · ${formatNumber(row.requestCount)} 次请求`"
          >
            <div class="trend-bar">
              <span :style="{ height: historyTrendHeight(row) }"></span>
            </div>
            <small>{{ row.date.slice(5) }}</small>
          </div>
        </div>
        <div v-else class="empty compact-empty">暂无趋势数据</div>
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
              <span>{{ formatNumber(modelPieTotal) }}</span>
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

    <div class="table-wrap">
      <table class="account-table history-table">
        <colgroup>
          <col class="history-col-time" />
          <col class="history-col-route" />
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
      <div v-else class="history-pagination">
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
