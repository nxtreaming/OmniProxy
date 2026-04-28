<script setup>
import { computed, reactive } from 'vue'
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

const filteredHistory = computed(() => filterHistory(props.entries, filters))

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
        <p>持久化记录代理请求、重试结果、账号、模型、耗时和 Token 用量</p>
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
            v-for="entry in filteredHistory.slice(0, 300)"
            :key="entry.id"
            class="clickable-history-row"
            @click="openHistoryDiagnosis(entry)"
          >
            <td>{{ formatTime(entry.time) }}</td>
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
    </div>
  </section>
</template>
