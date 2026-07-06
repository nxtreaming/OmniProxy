<script setup>
import { computed, ref } from 'vue'
import GeminiSelect from '../../components/GeminiSelect.vue'
import { diagnoseGatewayRoute } from '../../services/api'

const props = defineProps({
  model: {
    type: String,
    default: '',
  },
  providerLabel: {
    type: Function,
    required: true,
  },
  credentialLabels: {
    type: Object,
    default: () => ({}),
  },
})

const routeDiagnosticClient = ref('codex')
const routeDiagnosticModel = ref('')
const routeDiagnosticLoading = ref(false)
const routeDiagnosticError = ref('')
const routeDiagnosticResult = ref(null)
const routeDiagnosticClientOptions = [
  { value: 'codex', label: 'Codex', description: 'Codex /codex/v1' },
  { value: 'claude', label: 'Claude Code', description: 'Anthropic router' },
  { value: 'claude-desktop', label: 'Claude Desktop', description: '3P Gateway Profile' },
  { value: 'opencode', label: 'OpenCode', description: 'OpenAI 兼容入口' },
  { value: 'pi', label: 'Pi Coding Agent', description: 'Pi 本地入口' },
  { value: 'gemini', label: 'Gemini CLI', description: 'Gemini router' },
]
const selectedRouteDiagnosticCandidate = computed(() => {
  const result = routeDiagnosticResult.value
  if (!result?.chain?.length || result.selectedIndex < 0) return null
  return result.chain.find((item) => item.index === result.selectedIndex) || null
})

function diagnosticModelValue() {
  return String(routeDiagnosticModel.value || props.model || '').trim()
}

async function runRouteDiagnostic() {
  routeDiagnosticLoading.value = true
  routeDiagnosticError.value = ''
  routeDiagnosticResult.value = null
  try {
    routeDiagnosticResult.value = await diagnoseGatewayRoute({
      client: routeDiagnosticClient.value,
      model: diagnosticModelValue(),
    })
  } catch (error) {
    routeDiagnosticError.value = error.message
  } finally {
    routeDiagnosticLoading.value = false
  }
}

function diagnosticStatusText(candidate) {
  if (!candidate) return '-'
  if (candidate.available) return '可用'
  return candidate.issue || '不可用'
}

function diagnosticCredentialText(candidate) {
  if (!candidate?.credentialType) return '自动匹配'
  return props.credentialLabels[candidate.credentialType] || candidate.credentialType
}

function diagnosticTokenText(candidate) {
  if (!candidate?.tokenName) return '未命中账号'
  const parts = [candidate.tokenName]
  if (candidate.tokenCredentialType) {
    parts.push(props.credentialLabels[candidate.tokenCredentialType] || candidate.tokenCredentialType)
  }
  if (candidate.tokenStatus) {
    parts.push(candidate.tokenStatus)
  }
  return parts.join(' · ')
}
</script>

<template>
  <div class="gateway-advanced-head">
    <strong>路由自检</strong>
    <span>按真实 Router 解析，不会发送上游请求</span>
  </div>
  <article class="gateway-route-row gateway-diagnostic-row">
    <div class="gateway-route-summary">
      <div>
        <strong>测试当前模型</strong>
        <small>检查入口、主备链、账号和目标 URL</small>
      </div>
      <div class="gateway-route-meta">
        <code>{{ diagnosticModelValue() || '-' }}</code>
        <div v-if="selectedRouteDiagnosticCandidate" class="gateway-route-chain">
          <span class="gateway-route-chain-item">
            <small>命中</small>
            {{ providerLabel(selectedRouteDiagnosticCandidate.provider) }}
          </span>
        </div>
      </div>
    </div>
    <div class="gateway-route-controls gateway-diagnostic-controls">
      <label>
        <span>客户端入口</span>
        <GeminiSelect
          v-model="routeDiagnosticClient"
          :options="routeDiagnosticClientOptions"
          aria-label="路由自检客户端入口"
        />
      </label>
      <label>
        <span>模型 ID</span>
        <input
          v-model="routeDiagnosticModel"
          type="text"
          :placeholder="model"
          @keydown.enter.prevent="runRouteDiagnostic"
        />
      </label>
      <div class="gateway-diagnostic-actions">
        <button type="button" class="primary-button" :disabled="routeDiagnosticLoading" @click="runRouteDiagnostic">
          {{ routeDiagnosticLoading ? '检测中' : '运行自检' }}
        </button>
      </div>
    </div>
    <div v-if="routeDiagnosticError" class="gateway-diagnostic-message danger">
      {{ routeDiagnosticError }}
    </div>
    <div v-if="routeDiagnosticResult" class="gateway-diagnostic-result">
      <div class="gateway-diagnostic-result-head">
        <strong>{{ routeDiagnosticResult.ok ? '路由可用' : '路由不可用' }}</strong>
        <span>{{ routeDiagnosticResult.clientName || routeDiagnosticResult.clientKey }} · {{ routeDiagnosticResult.routedModel || '-' }}</span>
      </div>
      <div class="gateway-diagnostic-chain">
        <article
          v-for="candidate in routeDiagnosticResult.chain"
          :key="`${candidate.index}-${candidate.provider}`"
          :class="['gateway-diagnostic-candidate', { available: candidate.available, selected: candidate.index === routeDiagnosticResult.selectedIndex }]"
        >
          <div class="gateway-diagnostic-candidate-head">
            <span>{{ candidate.role }}</span>
            <strong>{{ providerLabel(candidate.provider) }}</strong>
            <small>{{ diagnosticStatusText(candidate) }}</small>
          </div>
          <div class="gateway-diagnostic-grid">
            <div>
              <span>凭据</span>
              <strong>{{ diagnosticCredentialText(candidate) }}</strong>
            </div>
            <div>
              <span>账号</span>
              <strong>{{ diagnosticTokenText(candidate) }}</strong>
            </div>
            <div>
              <span>路径</span>
              <strong class="mono">{{ candidate.path || '-' }}</strong>
            </div>
            <div>
              <span>目标</span>
              <strong class="mono">{{ candidate.targetUrl || candidate.baseUrl || '-' }}</strong>
            </div>
          </div>
        </article>
      </div>
    </div>
  </article>
</template>
