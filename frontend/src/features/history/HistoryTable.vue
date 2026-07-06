<script setup>
import { View } from '@element-plus/icons-vue'
import { clientLabel, isFailedHistory } from '../../utils/historyAnalytics'

const props = defineProps({
  entries: { type: Array, default: () => [] },
  filteredCount: { type: Number, default: 0 },
  page: { type: Number, required: true },
  totalPages: { type: Number, required: true },
  pageSize: { type: Number, required: true },
  formatNumber: { type: Function, required: true },
  formatDuration: { type: Function, required: true },
  providerLabel: { type: Function, required: true },
})
const emit = defineEmits(['diagnose', 'previous-page', 'next-page'])

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

function historyStatusLabel(entry) {
  if (!entry.status) return '-'
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
</script>

<template>
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
            v-for="entry in entries"
            :key="entry.id"
            class="clickable-history-row"
            @click="emit('diagnose', entry)"
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
                @click.stop="emit('diagnose', entry)"
              >
                {{ isFailedHistory(entry) ? '诊断' : '详情' }}
              </el-button>
            </td>
          </tr>
        </tbody>
      </table>
      <div v-if="!filteredCount" class="empty">暂无匹配的请求历史</div>
    </div>
    <div v-if="filteredCount" class="history-pagination">
      <span>共 {{ formatNumber(filteredCount) }} 条，每页 {{ pageSize }} 条</span>
      <div>
        <el-button size="small" :disabled="page <= 1" @click="emit('previous-page')">上一页</el-button>
        <strong>{{ page }} / {{ totalPages }}</strong>
        <el-button size="small" :disabled="page >= totalPages" @click="emit('next-page')">下一页</el-button>
      </div>
    </div>
  </div>
</template>
