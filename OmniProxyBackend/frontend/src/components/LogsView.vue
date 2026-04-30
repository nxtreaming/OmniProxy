<script setup>
import { Refresh } from '@element-plus/icons-vue'

defineProps({
  logs: {
    type: Array,
    required: true,
  },
  formatTime: {
    type: Function,
    required: true,
  },
  formatDuration: {
    type: Function,
    required: true,
  },
})

defineEmits(['refresh'])
</script>

<template>
  <section class="panel">
    <div class="section-heading">
      <div>
        <h2>实时日志</h2>
        <p>每 3 秒自动刷新</p>
      </div>
      <el-button :icon="Refresh" @click="$emit('refresh')">刷新</el-button>
    </div>
    <div class="log-list">
      <div v-for="entry in logs" :key="entry.id" class="log-row">
        <span :class="['dot', entry.level]"></span>
        <div class="log-main">
          <strong>{{ entry.method || 'SYSTEM' }} {{ entry.path || '' }}</strong>
          <p>{{ entry.message }}</p>
        </div>
        <div class="log-meta">
          <small class="log-model" :title="entry.model || '-'">{{ entry.model || '-' }}</small>
          <small class="log-status">{{ entry.status || '-' }}</small>
          <small class="log-duration">{{ formatDuration(entry.durationMs) }}</small>
          <small class="log-token" :title="entry.tokenName || '-'">{{ entry.tokenName || '-' }}</small>
          <time class="log-time">{{ formatTime(entry.time) }}</time>
        </div>
      </div>
      <div v-if="!logs.length" class="empty">暂无日志</div>
    </div>
  </section>
</template>
