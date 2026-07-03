<script setup>
import { Refresh } from '@element-plus/icons-vue'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

const props = defineProps({
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

const recentLogWindowMs = 5 * 60 * 1000
const now = ref(Date.now())
let nowTimer = null

const recentLogs = computed(() => {
  const cutoff = now.value - recentLogWindowMs

  return props.logs.filter((entry) => {
    const timestamp = Date.parse(entry.time || '')
    return !Number.isFinite(timestamp) || timestamp >= cutoff
  })
})

onMounted(() => {
  nowTimer = window.setInterval(() => {
    now.value = Date.now()
  }, 15000)
})

onBeforeUnmount(() => {
  if (nowTimer) {
    window.clearInterval(nowTimer)
  }
})
</script>

<template>
  <section class="panel logs-page-panel">
    <div class="section-heading">
      <div>
        <h2>代理事件流</h2>
        <p>仅显示最近 5 分钟 · 每 3 秒自动刷新</p>
      </div>
      <el-button :icon="Refresh" @click="$emit('refresh')">刷新</el-button>
    </div>
    <div class="log-list">
      <div v-for="entry in recentLogs" :key="entry.id" :class="['log-row', entry.level || 'info']">
        <span :class="['dot', entry.level]"></span>
        <div class="log-content">
          <div class="log-main">
            <strong class="log-title">
              {{ entry.method || 'SYSTEM' }}<span v-if="entry.path"> {{ entry.path }}</span>
            </strong>
            <p>{{ entry.message }}</p>
          </div>
          <div class="log-meta">
            <small v-if="entry.clientName && entry.clientName !== '-'" class="log-client" :title="entry.clientName">{{ entry.clientName }}</small>
            <small v-if="entry.model && entry.model !== '-'" class="log-model" :title="entry.model">{{ entry.model }}</small>
            <small
              v-if="entry.status"
              :class="[
                'log-status',
                {
                  success: Number(entry.status) >= 200 && Number(entry.status) < 300,
                  error: Number(entry.status) >= 400,
                },
              ]"
            >
              {{ entry.status }}
            </small>
            <small v-if="entry.durationMs !== undefined && entry.durationMs !== null" class="log-duration">
              {{ formatDuration(entry.durationMs) }}
            </small>
            <small v-if="entry.tokenName && entry.tokenName !== '-'" class="log-token" :title="entry.tokenName">{{ entry.tokenName }}</small>
            <time class="log-time">{{ formatTime(entry.time) }}</time>
          </div>
        </div>
      </div>
      <div v-if="!recentLogs.length" class="empty">最近 5 分钟暂无日志</div>
    </div>
  </section>
</template>

<style src="./LogsView.css"></style>
