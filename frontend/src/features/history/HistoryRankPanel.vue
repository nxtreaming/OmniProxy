<script setup>
defineProps({
  title: { type: String, required: true },
  items: { type: Array, default: () => [] },
  emptyLabel: { type: String, required: true },
  valueKey: { type: String, default: 'count' },
  valueSuffix: { type: String, default: '次' },
  wide: { type: Boolean, default: false },
  formatNumber: { type: Function, required: true },
})
</script>

<template>
  <div :class="['history-insight-panel', { 'wide-history-panel': wide }]">
    <div class="history-insight-head">
      <span>{{ title }}</span>
      <strong>{{ items.length }}</strong>
    </div>
    <div class="rank-list">
      <div v-for="item in items" :key="item.label" class="rank-row">
        <span :title="item.label">{{ item.label }}</span>
        <strong>{{ formatNumber(item[valueKey]) }} {{ valueSuffix }}</strong>
      </div>
      <div v-if="!items.length" class="empty compact-empty">{{ emptyLabel }}</div>
    </div>
  </div>
</template>
