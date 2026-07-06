<script setup>
import { computed, onBeforeUnmount, ref } from 'vue'

const props = defineProps({
  rows: { type: Array, default: () => [] },
  totalTokens: { type: Number, default: 0 },
  windowDays: { type: Number, default: 14 },
  formatNumber: { type: Function, required: true },
})

const activeTooltip = ref(null)
const trendMax = computed(() => Math.max(1, ...props.rows.map((row) => row.totalTokens)))
const trendColumns = computed(() => `repeat(${Math.max(1, props.rows.length)}, minmax(0, 1fr))`)

function trendHeight(row) {
  const value = Number(row.totalTokens || 0)
  if (value <= 0) return '4%'
  return `${Math.max(8, Math.round((value / trendMax.value) * 100))}%`
}

function tooltipPosition(event) {
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

function tooltipData(row) {
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

function showTooltip(row, event) {
  activeTooltip.value = {
    ...tooltipData(row),
    ...tooltipPosition(event),
  }
}

function moveTooltip(row, event) {
  if (!activeTooltip.value || activeTooltip.value.key !== row.date) return
  activeTooltip.value = {
    ...activeTooltip.value,
    ...tooltipPosition(event),
  }
}

function hideTooltip() {
  activeTooltip.value = null
}

function isTooltipActive(row) {
  return activeTooltip.value?.key === row.date
}

onBeforeUnmount(hideTooltip)
</script>

<template>
  <div class="history-insight-panel history-trend-panel">
    <div class="history-insight-head">
      <span>每日用量 · 近 {{ windowDays }} 天</span>
      <strong>{{ formatNumber(totalTokens) }}</strong>
    </div>
    <div v-if="rows.length" class="usage-trend compact-history-trend" :style="{ gridTemplateColumns: trendColumns }">
      <div
        v-for="row in rows"
        :key="row.date"
        :class="['trend-column', { active: isTooltipActive(row) }]"
        role="img"
        tabindex="0"
        :aria-label="`${row.date} · ${formatNumber(row.totalTokens)} Token · ${formatNumber(row.requestCount)} 次请求`"
        :aria-describedby="isTooltipActive(row) ? 'history-trend-tooltip' : undefined"
        @mouseenter="showTooltip(row, $event)"
        @mousemove="moveTooltip(row, $event)"
        @mouseleave="hideTooltip"
        @focus="showTooltip(row, $event)"
        @blur="hideTooltip"
      >
        <div class="trend-bar">
          <span
            :class="{ empty: Number(row.totalTokens || 0) <= 0 }"
            :style="{ height: trendHeight(row) }"
          ></span>
        </div>
        <small>{{ row.date.slice(5) }}</small>
      </div>
    </div>
    <div v-else class="empty compact-empty">暂无趋势数据</div>
    <Teleport to="body">
      <Transition name="trend-tooltip-fade">
        <div
          v-if="activeTooltip"
          id="history-trend-tooltip"
          class="trend-tooltip history-trend-tooltip"
          :class="{ below: activeTooltip.placement === 'below' }"
          :style="{ left: `${activeTooltip.x}px`, top: `${activeTooltip.y}px` }"
          role="tooltip"
        >
          <div class="trend-tooltip-head">
            <span>{{ activeTooltip.date }}</span>
            <strong>{{ activeTooltip.title }}</strong>
          </div>
          <div class="trend-tooltip-primary">
            <strong>{{ formatNumber(activeTooltip.value) }}</strong>
            <span>{{ activeTooltip.valueUnit }}</span>
          </div>
          <div class="trend-tooltip-grid">
            <span>请求 <strong>{{ formatNumber(activeTooltip.requestCount) }}</strong></span>
            <span>成功 <strong>{{ formatNumber(activeTooltip.successCount) }}</strong></span>
            <span>失败 <strong>{{ formatNumber(activeTooltip.failedCount) }}</strong></span>
          </div>
          <p>{{ activeTooltip.statusText }}</p>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>
