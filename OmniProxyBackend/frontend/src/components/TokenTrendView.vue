<script setup>
import { computed } from 'vue'
import { Lightning, Memo, Refresh, TrendCharts } from '@element-plus/icons-vue'

const CHART_WIDTH = 1000
const CHART_HEIGHT = 240
const CHART_TOP = 22
const CHART_BOTTOM = 202
const CHART_PLOT_HEIGHT = CHART_BOTTOM - CHART_TOP

const props = defineProps({
  dailyUsageRows: {
    type: Array,
    required: true,
  },
  formatNumber: {
    type: Function,
    required: true,
  },
})

const emit = defineEmits(['refresh'])

const trendRows = computed(() =>
  [...(props.dailyUsageRows || [])]
    .filter((row) => row?.date)
    .sort((a, b) => String(a.date).localeCompare(String(b.date))),
)
const tableRows = computed(() => [...trendRows.value].reverse())
const recordedDays = computed(() => trendRows.value.length)
const totalTokens = computed(() => sumField(trendRows.value, 'totalTokens'))
const totalRequests = computed(() => sumField(trendRows.value, 'requestCount'))
const totalInputTokens = computed(() => sumField(trendRows.value, 'inputTokens'))
const totalOutputTokens = computed(() => sumField(trendRows.value, 'outputTokens'))
const tokenMax = computed(() => Math.max(1, ...trendRows.value.map((row) => Number(row.totalTokens || 0))))
const requestMax = computed(() => Math.max(1, ...trendRows.value.map((row) => Number(row.requestCount || 0))))
const firstDate = computed(() => trendRows.value[0]?.date || '-')
const lastDate = computed(() => trendRows.value[trendRows.value.length - 1]?.date || '-')
const tokenPeakDay = computed(() => peakRow(trendRows.value, 'totalTokens'))
const requestPeakDay = computed(() => peakRow(trendRows.value, 'requestCount'))
const tokenSeries = computed(() => buildSeries(trendRows.value, 'totalTokens', tokenMax.value))
const requestSeries = computed(() => buildSeries(trendRows.value, 'requestCount', requestMax.value))
const tokenLinePoints = computed(() => linePoints(tokenSeries.value))
const requestLinePoints = computed(() => linePoints(requestSeries.value))
const tokenAreaPath = computed(() => areaPath(tokenSeries.value))
const requestAreaPath = computed(() => areaPath(requestSeries.value))
const tokenTicks = computed(() => axisTicks(tokenMax.value))
const requestTicks = computed(() => axisTicks(requestMax.value))
const xAxisLabels = computed(() => buildXAxisLabels(trendRows.value))
const recentRows = computed(() => trendRows.value.slice(-7))
const recentTokens = computed(() => sumField(recentRows.value, 'totalTokens'))
const recentRequests = computed(() => sumField(recentRows.value, 'requestCount'))
const previousRows = computed(() => trendRows.value.slice(-14, -7))
const previousTokens = computed(() => sumField(previousRows.value, 'totalTokens'))
const previousRequests = computed(() => sumField(previousRows.value, 'requestCount'))
const tokenDeltaText = computed(() => deltaText(recentTokens.value, previousTokens.value, 'Token'))
const requestDeltaText = computed(() => deltaText(recentRequests.value, previousRequests.value, '次请求'))
const summaryCards = computed(() => [
  {
    label: '记录天数',
    value: props.formatNumber(recordedDays.value),
    detail: `${firstDate.value} 至 ${lastDate.value}`,
    icon: TrendCharts,
  },
  {
    label: '总 Token',
    value: props.formatNumber(totalTokens.value),
    detail: `输入 ${props.formatNumber(totalInputTokens.value)} · 输出 ${props.formatNumber(totalOutputTokens.value)}`,
    icon: Lightning,
  },
  {
    label: '总请求',
    value: props.formatNumber(totalRequests.value),
    detail: `近 7 天 ${props.formatNumber(recentRequests.value)} 次`,
    icon: Memo,
  },
  {
    label: '日均 Token / 请求',
    value: `${props.formatNumber(average(totalTokens.value, recordedDays.value))} / ${props.formatNumber(average(totalRequests.value, recordedDays.value))}`,
    detail: `峰值 ${tokenPeakDay.value?.date || '-'} · ${props.formatNumber(tokenPeakDay.value?.totalTokens || 0)} Token`,
    icon: TrendCharts,
  },
])

function refresh() {
  emit('refresh')
}

function sumField(rows, field) {
  return rows.reduce((sum, row) => sum + Number(row?.[field] || 0), 0)
}

function average(value, count) {
  if (!count) return 0
  return Math.round(Number(value || 0) / count)
}

function peakRow(rows, field) {
  return rows.reduce((best, row) => {
    if (!best) return row
    return Number(row?.[field] || 0) > Number(best?.[field] || 0) ? row : best
  }, null)
}

function buildSeries(rows, field, maxValue) {
  const safeRows = rows || []
  const lastIndex = Math.max(1, safeRows.length - 1)
  return safeRows.map((row, index) => {
    const value = Number(row?.[field] || 0)
    const x = safeRows.length === 1 ? CHART_WIDTH / 2 : (index / lastIndex) * CHART_WIDTH
    const y = CHART_BOTTOM - (value / Math.max(1, maxValue)) * CHART_PLOT_HEIGHT
    return {
      row,
      value,
      x: Number(x.toFixed(2)),
      y: Number(y.toFixed(2)),
    }
  })
}

function linePoints(series) {
  return series.map((point) => `${point.x},${point.y}`).join(' ')
}

function areaPath(series) {
  if (!series.length) return ''
  const points = series.map((point) => `L ${point.x} ${point.y}`).join(' ')
  const first = series[0]
  const last = series[series.length - 1]
  return `M ${first.x} ${CHART_BOTTOM} ${points} L ${last.x} ${CHART_BOTTOM} Z`
}

function axisTicks(maxValue) {
  return [1, 0.5, 0].map((ratio) => {
    const value = Math.round(maxValue * ratio)
    return {
      value,
      label: compactNumber(value),
      y: CHART_BOTTOM - ratio * CHART_PLOT_HEIGHT,
    }
  })
}

function buildXAxisLabels(rows) {
  if (!rows.length) return []
  if (rows.length === 1) return [{ key: rows[0].date, label: rows[0].date, left: '50%' }]
  const middle = rows[Math.floor((rows.length - 1) / 2)]
  return [
    { key: rows[0].date, label: rows[0].date, left: '0%' },
    { key: middle.date, label: middle.date, left: '50%' },
    { key: rows[rows.length - 1].date, label: rows[rows.length - 1].date, left: '100%' },
  ]
}

function compactNumber(value) {
  const number = Number(value || 0)
  if (number >= 100000000) return `${(number / 100000000).toFixed(1)}亿`
  if (number >= 10000) return `${(number / 10000).toFixed(1)}万`
  return props.formatNumber(Math.round(number))
}

function deltaText(current, previous, unit) {
  if (!previous) return `近 7 天 ${props.formatNumber(current)} ${unit}`
  const delta = current - previous
  const percent = Math.round((Math.abs(delta) / previous) * 100)
  const direction = delta >= 0 ? '增加' : '减少'
  return `较前 7 天${direction} ${percent}%`
}

function pointTitle(point, label, unit) {
  return `${point.row.date} · ${props.formatNumber(point.value)} ${unit} · 输入 ${props.formatNumber(point.row.inputTokens || 0)} · 输出 ${props.formatNumber(point.row.outputTokens || 0)}`
}
</script>

<template>
  <section class="token-trend-page">
    <div class="token-trend-toolbar">
      <p>展示全部已记录天数的 Token 与请求次数变化</p>
      <el-button :icon="Refresh" @click="refresh">刷新</el-button>
    </div>

    <div class="token-trend-summary">
      <article v-for="card in summaryCards" :key="card.label">
        <component :is="card.icon" class="token-trend-summary-icon" aria-hidden="true" />
        <span>{{ card.label }}</span>
        <strong>{{ card.value }}</strong>
        <small>{{ card.detail }}</small>
      </article>
    </div>

    <div v-if="trendRows.length" class="token-trend-chart-grid">
      <article class="token-trend-chart-card token-line-card">
        <div class="trend-panel-head">
          <span>Token 折线</span>
          <strong>{{ formatNumber(totalTokens) }}</strong>
        </div>
        <p class="token-trend-card-note">{{ tokenDeltaText }}</p>
        <div class="token-trend-chart-frame">
          <div class="token-trend-y-axis">
            <span v-for="tick in tokenTicks" :key="`token-tick-${tick.y}`">{{ tick.label }}</span>
          </div>
          <div class="token-trend-svg-wrap">
            <svg class="token-trend-svg" viewBox="0 0 1000 240" role="img" aria-label="全部天数 Token 折线图">
              <line
                v-for="tick in tokenTicks"
                :key="`token-grid-${tick.y}`"
                class="token-trend-grid-line"
                x1="0"
                x2="1000"
                :y1="tick.y"
                :y2="tick.y"
              />
              <path class="token-trend-area token-area" :d="tokenAreaPath" />
              <polyline class="token-trend-line token-line" :points="tokenLinePoints" />
              <circle
                v-for="point in tokenSeries"
                :key="`token-point-${point.row.date}`"
                class="token-trend-point token-point"
                :cx="point.x"
                :cy="point.y"
                r="3"
              >
                <title>{{ pointTitle(point, 'Token', 'Token') }}</title>
              </circle>
            </svg>
            <div class="token-trend-x-axis">
              <span v-for="label in xAxisLabels" :key="`token-x-${label.key}`" :style="{ left: label.left }">
                {{ label.label }}
              </span>
            </div>
          </div>
        </div>
      </article>

      <article class="token-trend-chart-card request-line-card">
        <div class="trend-panel-head">
          <span>请求次数折线</span>
          <strong>{{ formatNumber(totalRequests) }}</strong>
        </div>
        <p class="token-trend-card-note">{{ requestDeltaText }}</p>
        <div class="token-trend-chart-frame">
          <div class="token-trend-y-axis">
            <span v-for="tick in requestTicks" :key="`request-tick-${tick.y}`">{{ tick.label }}</span>
          </div>
          <div class="token-trend-svg-wrap">
            <svg class="token-trend-svg" viewBox="0 0 1000 240" role="img" aria-label="全部天数请求次数折线图">
              <line
                v-for="tick in requestTicks"
                :key="`request-grid-${tick.y}`"
                class="token-trend-grid-line"
                x1="0"
                x2="1000"
                :y1="tick.y"
                :y2="tick.y"
              />
              <path class="token-trend-area request-area" :d="requestAreaPath" />
              <polyline class="token-trend-line request-line" :points="requestLinePoints" />
              <circle
                v-for="point in requestSeries"
                :key="`request-point-${point.row.date}`"
                class="token-trend-point request-point"
                :cx="point.x"
                :cy="point.y"
                r="3"
              >
                <title>{{ point.row.date }} · {{ formatNumber(point.value) }} 次请求 · {{ formatNumber(point.row.totalTokens || 0) }} Token</title>
              </circle>
            </svg>
            <div class="token-trend-x-axis">
              <span v-for="label in xAxisLabels" :key="`request-x-${label.key}`" :style="{ left: label.left }">
                {{ label.label }}
              </span>
            </div>
          </div>
        </div>
      </article>
    </div>
    <div v-else class="empty token-trend-empty">暂无代理 Token 用量</div>

    <section class="panel token-trend-table-panel">
      <div class="section-heading">
        <div>
          <h2>每日明细</h2>
          <p>按日期倒序展示全部已记录天数</p>
        </div>
        <div class="token-trend-peak">
          <span>请求峰值</span>
          <strong>{{ requestPeakDay?.date || '-' }} · {{ formatNumber(requestPeakDay?.requestCount || 0) }} 次</strong>
        </div>
      </div>
      <div class="usage-table token-trend-table">
        <div class="usage-row header">
          <span>日期</span>
          <span>总 Token</span>
          <span>输入</span>
          <span>输出</span>
          <span>请求</span>
        </div>
        <div v-for="row in tableRows" :key="row.date" class="usage-row">
          <span>{{ row.date }}</span>
          <strong>{{ formatNumber(row.totalTokens) }}</strong>
          <span>{{ formatNumber(row.inputTokens) }}</span>
          <span>{{ formatNumber(row.outputTokens) }}</span>
          <span>{{ formatNumber(row.requestCount) }}</span>
        </div>
        <div v-if="!tableRows.length" class="empty">暂无代理 Token 用量</div>
      </div>
    </section>
  </section>
</template>
