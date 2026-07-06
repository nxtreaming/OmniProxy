<script setup>
import { computed } from 'vue'

const props = defineProps({
  modelRanks: { type: Array, default: () => [] },
  segments: { type: Array, default: () => [] },
  formatNumber: { type: Function, required: true },
})

const pieTotal = computed(() =>
  props.segments.reduce((sum, item) => sum + Number(item.totalTokens || 0), 0),
)
const pieTotalLabel = computed(() => compactMetricNumber(pieTotal.value))
const pieGradient = computed(() => {
  const total = pieTotal.value
  if (total <= 0 || !props.segments.length) return ''

  let cursor = 0
  const parts = props.segments.map((item, index) => {
    const start = cursor
    const end = index === props.segments.length - 1
      ? 360
      : cursor + (Number(item.totalTokens || 0) / total) * 360
    cursor = end
    return `${item.color} ${start.toFixed(2)}deg ${end.toFixed(2)}deg`
  })
  return `conic-gradient(${parts.join(', ')})`
})

function compactMetricNumber(value) {
  const number = Number(value || 0)
  if (number >= 100000000) return `${(number / 100000000).toFixed(number >= 1000000000 ? 1 : 2)}亿`
  if (number >= 10000) return `${(number / 10000).toFixed(number >= 1000000 ? 1 : 2)}万`
  return props.formatNumber(Math.round(number))
}
</script>

<template>
  <div class="history-insight-panel model-insight-panel">
    <div class="history-insight-head">
      <span>模型消耗</span>
      <strong>{{ modelRanks.length }}</strong>
    </div>
    <div v-if="segments.length" class="model-pie-layout">
      <div
        class="model-pie-chart"
        :style="{ background: pieGradient }"
        role="img"
        :aria-label="`模型 Token 消耗占比，共 ${formatNumber(pieTotal)} Token`"
      >
        <div>
          <span :title="`${formatNumber(pieTotal)} Token`">{{ pieTotalLabel }}</span>
          <small>Token</small>
        </div>
      </div>
      <div class="model-pie-legend">
        <div
          v-for="item in segments"
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
</template>
