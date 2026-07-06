<script setup>
import { ref } from 'vue'

const props = defineProps({
  entry: {
    type: Object,
    default: null,
  },
  providerLabel: {
    type: Function,
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

const emit = defineEmits(['close'])
const copied = ref(false)

function diagnosticRetryChain(entry) {
  if (entry?.retryChain?.length) {
    return entry.retryChain
  }
  return [
    {
      attempt: 1,
      provider: entry?.provider,
      protocol: entry?.protocol,
      model: entry?.model,
      status: entry?.status,
      durationMs: entry?.durationMs,
      tokenName: entry?.tokenName,
      cooldownTriggered: entry?.cooldownTriggered,
      message: entry?.message,
    },
  ]
}

function historyStatusLabel(entry) {
  if (!entry?.status) return '-'
  return `${entry.status}`
}

function isProblemHistory(entry) {
  return entry?.level === 'error' || entry?.level === 'warn' || Number(entry?.status || 0) >= 400
}

function historyMessageSummary(entry) {
  return entry?.message || historyStatusLabel(entry)
}

function isStatusCheckEntry(entry) {
  return (
    entry?.method === 'CHECK' ||
    entry?.protocol === 'health-check' ||
    String(entry?.path || '').includes('/maintenance/token-health-check')
  )
}

function diagnosticClientLabel(entry) {
  if (isStatusCheckEntry(entry)) return '状态检查'
  return entry?.clientName || entry?.clientKey || '-'
}

function diagnosticSummary(entry) {
  if (!entry) return ''
  const lines = [
    `时间: ${props.formatTime(entry.time)}`,
    `请求: ${entry.method || '-'} ${entry.path || '-'}`,
    `工具: ${diagnosticClientLabel(entry)}`,
    `路由: ${props.providerLabel(entry.provider)} / ${entry.protocol || '-'}`,
    `模型: ${entry.model || '-'}`,
    `账号: ${entry.tokenName || '-'}`,
    `状态: ${entry.status || '-'} / ${props.formatDuration(entry.durationMs)}`,
    `消息: ${historyMessageSummary(entry)}`,
    '链路:',
    ...diagnosticRetryChain(entry).map((attempt) =>
      `  #${attempt.attempt || '-'} ${props.providerLabel(attempt.provider)} ${attempt.model || entry.model || '-'} ${attempt.tokenName || '-'} ${attempt.status || '-'} ${props.formatDuration(attempt.durationMs)}${attempt.cooldownTriggered ? ' 冷却' : ''} ${attempt.message || ''}`.trimEnd(),
    ),
  ]
  return lines.join('\n')
}

async function copyDiagnosticSummary() {
  if (!props.entry || typeof navigator === 'undefined' || !navigator.clipboard) return
  await navigator.clipboard.writeText(diagnosticSummary(props.entry))
  copied.value = true
  window.setTimeout(() => {
    copied.value = false
  }, 1600)
}
</script>

<template>
  <Transition name="diagnostic-panel">
    <div v-if="entry" class="drawer-backdrop" @click.self="emit('close')">
      <aside class="diagnostic-drawer" :aria-label="isProblemHistory(entry) ? '失败请求诊断' : '请求详情'">
        <div class="diagnostic-head">
          <div>
            <h2>{{ isProblemHistory(entry) ? '失败诊断' : '请求详情' }}</h2>
            <p>{{ formatTime(entry.time) }} · {{ entry.method }} {{ entry.path }}</p>
          </div>
          <div class="diagnostic-head-actions">
            <button type="button" @click="copyDiagnosticSummary">
              {{ copied ? '已复制' : '复制摘要' }}
            </button>
            <button type="button" aria-label="关闭诊断面板" @click="emit('close')">×</button>
          </div>
        </div>

        <div class="diagnostic-grid">
          <div>
            <span>路由厂商</span>
            <strong>{{ providerLabel(entry.provider) }}</strong>
          </div>
          <div>
            <span>模型</span>
            <strong>{{ entry.model || '-' }}</strong>
          </div>
          <div>
            <span>账号</span>
            <strong>{{ entry.tokenName || '-' }}</strong>
          </div>
          <div>
            <span>编程工具</span>
            <strong>{{ diagnosticClientLabel(entry) }}</strong>
          </div>
          <div>
            <span>协议</span>
            <strong>{{ entry.protocol || '-' }}</strong>
          </div>
          <div>
            <span>状态码</span>
            <strong>{{ entry.status || '-' }}</strong>
          </div>
          <div>
            <span>耗时</span>
            <strong>{{ formatDuration(entry.durationMs) }}</strong>
          </div>
          <div>
            <span>触发冷却</span>
            <strong>{{ entry.cooldownTriggered ? '是' : '否' }}</strong>
          </div>
        </div>

        <div class="diagnostic-section">
          <span>消息摘要</span>
          <p>{{ historyMessageSummary(entry) }}</p>
        </div>

        <div class="diagnostic-section">
          <span>{{ entry.retryChain?.length ? '重试链路' : '请求链路' }}</span>
          <div class="retry-chain">
            <div
              v-for="attempt in diagnosticRetryChain(entry)"
              :key="`${entry.id}-${attempt.attempt}-${attempt.tokenName || 'none'}`"
              class="retry-step"
            >
              <strong>#{{ attempt.attempt || '-' }}</strong>
              <div>
                <b>{{ providerLabel(attempt.provider) }}</b>
                <small>
                  {{ attempt.model || entry.model || '-' }} ·
                  {{ attempt.tokenName || '-' }} ·
                  {{ attempt.status || '-' }} ·
                  {{ formatDuration(attempt.durationMs) }}
                  <template v-if="attempt.cooldownTriggered"> · 冷却</template>
                </small>
                <p>{{ attempt.message || '-' }}</p>
              </div>
            </div>
          </div>
        </div>
      </aside>
    </div>
  </Transition>
</template>
