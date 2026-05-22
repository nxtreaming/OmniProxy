<script setup>
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { Connection, Delete, Refresh } from '@element-plus/icons-vue'
import { sendOpenRouterChat } from '../services/api'

const OPENROUTER_CHAT_STORAGE_KEY = 'omniproxy.openrouter.chat.v1'
const MAX_SAVED_MESSAGES = 80
const MAX_SAVED_MESSAGE_CHARS = 120000

const props = defineProps({
  models: {
    type: Array,
    default: () => [],
  },
  selectedModel: {
    type: String,
    default: '',
  },
  modelsLoading: {
    type: Boolean,
    default: false,
  },
  modelsError: {
    type: String,
    default: '',
  },
  openRouterTokens: {
    type: Array,
    default: () => [],
  },
  validatingIds: {
    type: Object,
    default: () => ({}),
  },
  formatTime: {
    type: Function,
    default: (value) => value || '-',
  },
  formatNumber: {
    type: Function,
    default: (value) => String(value ?? 0),
  },
})

const emit = defineEmits(['update:selected-model', 'refresh-models', 'refresh-token', 'open-create-key'])

const transcriptRef = ref(null)
const draft = ref('')
const messages = ref([])
const sending = ref(false)
const chatError = ref('')
const temperature = ref(0.7)
const maxTokens = ref(1024)
const modelSearch = ref('')
const freeOnly = ref(false)
const typing = ref(false)
let typingTimer = null

const selectedModelId = computed({
  get() {
    return props.selectedModel || props.models[0]?.id || ''
  },
  set(value) {
    emit('update:selected-model', String(value || '').trim())
  },
})

const selectedModelInfo = computed(() =>
  props.models.find((model) => model.id === selectedModelId.value),
)

const filteredModels = computed(() => {
  const source = freeOnly.value ? props.models.filter((model) => isFreeModel(model)) : props.models
  const query = modelSearch.value.trim().toLowerCase()
  if (!query) {
    return source
  }
  return source.filter((model) => {
    const id = String(model.id || '').toLowerCase()
    const name = String(model.name || '').toLowerCase()
    return id.includes(query) || name.includes(query)
  })
})

const freeModelCount = computed(() => props.models.filter((model) => isFreeModel(model)).length)
const draftLength = computed(() => Array.from(draft.value || '').length)
const showModelLoadingSkeleton = computed(() => props.modelsLoading && !props.models.length)
const openRouterQuotaToken = computed(
  () => props.openRouterTokens.find((item) => !item.disabled) || props.openRouterTokens[0] || null,
)
const openRouterQuotaLoading = computed(() => {
  const token = openRouterQuotaToken.value
  return Boolean(token?.id && props.validatingIds[token.id])
})

const canSend = computed(
  () => Boolean(selectedModelId.value && draft.value.trim()) && !sending.value && !typing.value,
)

const modelPriceLine = computed(() => {
  const pricing = selectedModelInfo.value?.pricing
  if (!pricing) return '未提供'
  if (isFreeModel(selectedModelInfo.value)) {
    return '价格 免费'
  }
  const parts = []
  if (pricing.prompt) parts.push(`输入 ${pricing.prompt}`)
  if (pricing.completion) parts.push(`输出 ${pricing.completion}`)
  if (pricing.request) parts.push(`请求 ${pricing.request}`)
  return parts.length ? `价格 ${parts.join(' / ')}` : '价格未提供'
})

const conversationUsage = computed(() =>
  messages.value.reduce(
    (sum, message) => {
      const usage = message.usage || {}
      sum.inputTokens += numberValue(usage.inputTokens)
      sum.outputTokens += numberValue(usage.outputTokens)
      sum.totalTokens += numberValue(usage.totalTokens)
      return sum
    },
    { inputTokens: 0, outputTokens: 0, totalTokens: 0 },
  ),
)

const conversationUsageLine = computed(() => {
  const usage = conversationUsage.value
  if (!usage.totalTokens && !usage.inputTokens && !usage.outputTokens) {
    return '当前对话用量 -'
  }
  return `当前对话 ${props.formatNumber(usage.totalTokens)} tokens · 输入 ${props.formatNumber(usage.inputTokens)} / 输出 ${props.formatNumber(usage.outputTokens)}`
})

const openRouterQuotaUpdatedText = computed(() => {
  const updatedAt = openRouterQuotaToken.value?.usage?.updatedAt
  return updatedAt ? props.formatTime(updatedAt) : '未刷新'
})

const openRouterQuotaMeta = computed(() => {
  const token = openRouterQuotaToken.value
  if (!token) return '请先添加 OpenRouter API Key'
  if (openRouterQuotaLoading.value) return '正在请求 OpenRouter /key...'
  if (token.disabled) return '当前 Key 已停用'
  if (token.lastError) return token.lastError
  const plan = token.usage?.planType || 'OpenRouter API Key'
  return `${plan} · ${openRouterQuotaUpdatedText.value}`
})

const openRouterQuotaRemainingText = computed(() => {
  const token = openRouterQuotaToken.value
  if (!token) return '-'
  return openRouterBalanceRemaining(token)
})

const openRouterQuotaUsedText = computed(() => {
  const token = openRouterQuotaToken.value
  if (!token) return '-'
  return openRouterBalanceValue(token, 'balanceUsed')
})

const openRouterQuotaLimitText = computed(() => {
  const token = openRouterQuotaToken.value
  if (!token) return '-'
  return openRouterBalanceLimit(token)
})

watch(selectedModelId, (nextModel, previousModel) => {
  if (previousModel && previousModel !== nextModel) {
    persistConversation(previousModel)
  }
  if (nextModel && nextModel !== previousModel) {
    loadConversation(nextModel)
  }
}, { immediate: true })

watch(
  () => [messages.value.length, sending.value, typing.value],
  () => {
    scrollTranscript()
  },
)

onBeforeUnmount(() => {
  persistConversation()
  clearTypingTimer()
})

function roleLabel(role) {
  return role === 'user' ? '你' : '模型'
}

function messageModelLabel(message) {
  return message?.role === 'user' ? '' : String(message?.model || '')
}

function requestMessages() {
  return messages.value
    .map((message) => ({
      role: message.role,
      content: message.content,
    }))
    .filter((message) => message.role && message.content)
}

function numberValue(value) {
  const numeric = Number(value || 0)
  return Number.isFinite(numeric) && numeric > 0 ? numeric : 0
}

function tokenUsageLine(usage) {
  const inputTokens = numberValue(usage?.inputTokens)
  const outputTokens = numberValue(usage?.outputTokens)
  const totalTokens = numberValue(usage?.totalTokens) || inputTokens + outputTokens
  if (!totalTokens && !inputTokens && !outputTokens) {
    return ''
  }
  return `用量 ${props.formatNumber(totalTokens)} tokens · 输入 ${props.formatNumber(inputTokens)} / 输出 ${props.formatNumber(outputTokens)}`
}

function balanceNumber(value) {
  const numeric = Number(value)
  return Number.isFinite(numeric) ? numeric : null
}

function formatBalance(value, unit = 'USD') {
  const numeric = balanceNumber(value)
  if (numeric === null) return '-'
  const fractionDigits = Math.abs(numeric) > 0 && Math.abs(numeric) < 1 ? 4 : 2
  return `${new Intl.NumberFormat('zh-CN', {
    minimumFractionDigits: fractionDigits,
    maximumFractionDigits: fractionDigits,
  }).format(numeric)} ${unit}`
}

function hasOpenRouterBalance(token) {
  return Boolean(token?.usage?.balanceUnit || token?.usage?.balanceUnlimited)
}

function openRouterBalanceValue(token, field) {
  if (!hasOpenRouterBalance(token)) {
    return '-'
  }
  return formatBalance(token.usage?.[field], token.usage.balanceUnit)
}

function openRouterBalanceRemaining(token) {
  if (!hasOpenRouterBalance(token)) {
    return '待刷新'
  }
  if (token.usage?.balanceUnlimited) {
    return '不限制'
  }
  if (balanceNumber(token.usage?.balanceTotal) <= 0 && balanceNumber(token.usage?.balanceRemaining) <= 0) {
    return '未返回'
  }
  return openRouterBalanceValue(token, 'balanceRemaining')
}

function openRouterBalanceLimit(token) {
  if (!hasOpenRouterBalance(token)) {
    return '-'
  }
  if (token.usage?.balanceUnlimited) {
    return '不限制'
  }
  if (balanceNumber(token.usage?.balanceTotal) <= 0) {
    return '未设置'
  }
  return openRouterBalanceValue(token, 'balanceTotal')
}

function refreshOpenRouterQuota() {
  if (!openRouterQuotaToken.value) {
    emit('open-create-key')
    return
  }
  emit('refresh-token', openRouterQuotaToken.value)
}

function localStorageSafe() {
  if (typeof window === 'undefined' || !window.localStorage) {
    return null
  }
  return window.localStorage
}

function readConversationStore() {
  const storage = localStorageSafe()
  if (!storage) {
    return {}
  }
  try {
    const payload = JSON.parse(storage.getItem(OPENROUTER_CHAT_STORAGE_KEY) || '{}')
    return payload && typeof payload.conversations === 'object' ? payload.conversations : {}
  } catch {
    return {}
  }
}

function writeConversationStore(conversations) {
  const storage = localStorageSafe()
  if (!storage) {
    return
  }
  try {
    storage.setItem(OPENROUTER_CHAT_STORAGE_KEY, JSON.stringify({
      version: 1,
      updatedAt: new Date().toISOString(),
      conversations,
    }))
  } catch {
    // Local chat history is best-effort; failures should not block the chat flow.
  }
}

function savedMessagesFor(modelId) {
  const item = readConversationStore()[modelId]
  if (!item || !Array.isArray(item.messages)) {
    return []
  }
  return item.messages
    .map((message) => ({
      role: message.role === 'user' ? 'user' : 'assistant',
      content: String(message.content || ''),
      model: String(message.model || ''),
      usage: message.usage || undefined,
      finishReason: message.finishReason || '',
    }))
    .filter((message) => message.content)
}

function persistConversation(modelId = selectedModelId.value) {
  const normalizedModel = String(modelId || '').trim()
  if (!normalizedModel) {
    return
  }
  const conversations = readConversationStore()
  const savedMessages = messages.value
    .filter((message) => message.role === 'user' || message.role === 'assistant')
    .filter((message) => String(message.content || '').trim())
    .slice(-MAX_SAVED_MESSAGES)
    .map((message) => ({
      role: message.role,
      content: String(message.content || '').slice(0, MAX_SAVED_MESSAGE_CHARS),
      model: message.role === 'user' ? '' : message.model || normalizedModel,
      usage: message.usage,
      finishReason: message.finishReason || '',
    }))

  if (!savedMessages.length) {
    delete conversations[normalizedModel]
  } else {
    conversations[normalizedModel] = {
      model: normalizedModel,
      updatedAt: new Date().toISOString(),
      messages: savedMessages,
    }
  }
  writeConversationStore(conversations)
}

function loadConversation(modelId = selectedModelId.value) {
  clearTypingTimer()
  messages.value = savedMessagesFor(String(modelId || '').trim())
  chatError.value = ''
  scrollTranscript()
}

function deleteConversation(modelId = selectedModelId.value) {
  const normalizedModel = String(modelId || '').trim()
  if (!normalizedModel) {
    return
  }
  const conversations = readConversationStore()
  delete conversations[normalizedModel]
  writeConversationStore(conversations)
}

async function scrollTranscript() {
  await nextTick()
  const node = transcriptRef.value
  if (node) {
    node.scrollTop = node.scrollHeight
  }
}

function isFreeModel(model) {
  const id = String(model?.id || '').toLowerCase()
  const name = String(model?.name || '').toLowerCase()
  const pricing = model?.pricing || {}
  return (
    id.endsWith(':free') ||
    name.includes('(free)') ||
    (isZeroPrice(pricing.prompt) && isZeroPrice(pricing.completion) && isZeroPrice(pricing.request))
  )
}

function isZeroPrice(value) {
  if (value === undefined || value === null || value === '') {
    return false
  }
  const numeric = Number(value)
  return Number.isFinite(numeric) && numeric === 0
}

function clearTypingTimer() {
  if (typingTimer) {
    window.clearTimeout(typingTimer)
    typingTimer = null
  }
  typing.value = false
}

function prefersReducedMotion() {
  return typeof window !== 'undefined' && window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
}

function typeAssistantMessage(message, fullText) {
  clearTypingTimer()
  const chars = Array.from(fullText || '')
  let index = 0
  let lastScrollAt = 0
  message.content = ''
  message.isTyping = true
  typing.value = true

  if (!chars.length || prefersReducedMotion()) {
    message.content = chars.join('')
    finishAssistantTyping(message)
    return
  }

  const tick = () => {
    const step = typingStep(chars.length)
    message.content += chars.slice(index, index + step).join('')
    index += step
    const now = typeof performance !== 'undefined' ? performance.now() : Date.now()
    if (index >= chars.length || now - lastScrollAt > 80) {
      lastScrollAt = now
      scrollTranscript()
    }

    if (index < chars.length) {
      typingTimer = window.setTimeout(tick, typingDelay(chars[index - 1], chars.length))
      return
    }
    finishAssistantTyping(message)
  }

  tick()
}

function finishAssistantTyping(message) {
  message.isTyping = false
  typing.value = false
  typingTimer = null
  persistConversation()
  scrollTranscript()
}

function typingStep(total) {
  if (total > 3200) return 8
  if (total > 1800) return 5
  if (total > 900) return 3
  if (total > 420) return 2
  return 1
}

function typingDelay(char, total) {
  if (total > 3200) return 8
  if (total > 1800) return 12
  if (/[。！？!?]/.test(char)) return 88
  if (/[，,；;：:]/.test(char)) return 42
  if (char === '\n') return 72
  return total > 900 ? 18 : 24
}

function resetConversation(clearDraft = true) {
  clearTypingTimer()
  messages.value = []
  chatError.value = ''
  deleteConversation()
  if (clearDraft) {
    draft.value = ''
  }
}

function selectModel(model) {
  if (!model?.id) {
    return
  }
  emit('update:selected-model', model.id)
}

function toggleFreeOnly() {
  freeOnly.value = !freeOnly.value
}

function updateDraft(event) {
  draft.value = event?.target?.value || ''
}

function firstDefined(source, keys) {
  if (!source || typeof source !== 'object') {
    return undefined
  }
  for (const key of keys) {
    if (source[key] !== undefined && source[key] !== null) {
      return source[key]
    }
  }
  return undefined
}

function normalizeChatText(value) {
  if (value === undefined || value === null) {
    return ''
  }
  if (typeof value === 'string') {
    return value.trim()
  }
  if (Array.isArray(value)) {
    return value
      .map((item) => normalizeChatText(
        typeof item === 'object' && item !== null
          ? firstDefined(item, ['text', 'Text', 'content', 'Content', 'value', 'Value'])
          : item,
      ))
      .filter(Boolean)
      .join('\n')
      .trim()
  }
  if (typeof value === 'object') {
    const nested = firstDefined(value, ['text', 'Text', 'content', 'Content', 'value', 'Value'])
    if (nested !== undefined && nested !== value) {
      return normalizeChatText(nested)
    }
    try {
      return JSON.stringify(value)
    } catch {
      return ''
    }
  }
  return String(value).trim()
}

function normalizeChatUsage(usage) {
  const source = usage && typeof usage === 'object' ? usage : {}
  const inputTokens = numberValue(firstDefined(source, ['inputTokens', 'InputTokens', 'prompt_tokens']))
  const outputTokens = numberValue(firstDefined(source, ['outputTokens', 'OutputTokens', 'completion_tokens']))
  const totalTokens = numberValue(firstDefined(source, ['totalTokens', 'TotalTokens', 'total_tokens']))
  return {
    inputTokens,
    outputTokens,
    totalTokens: totalTokens || inputTokens + outputTokens,
  }
}

function normalizeChatResult(result, fallbackModel) {
  if (typeof result === 'string') {
    return {
      role: 'assistant',
      content: result.trim(),
      model: fallbackModel,
      usage: normalizeChatUsage(),
      finishReason: '',
    }
  }

  const choices = Array.isArray(result?.choices) ? result.choices : Array.isArray(result?.Choices) ? result.Choices : []
  const choice = choices[0] || {}
  const choiceMessage = firstDefined(choice, ['message', 'Message', 'delta', 'Delta'])
  const message = firstDefined(result, ['message', 'Message']) || choiceMessage || {}
  const content = normalizeChatText(
    firstDefined(message, ['content', 'Content', 'text', 'Text']) ??
      firstDefined(result, ['content', 'Content', 'text', 'Text']),
  )

  return {
    role: firstDefined(message, ['role', 'Role']) || 'assistant',
    content,
    model: firstDefined(result, ['model', 'Model']) || fallbackModel,
    usage: normalizeChatUsage(firstDefined(result, ['usage', 'Usage'])),
    finishReason: firstDefined(result, ['finishReason', 'FinishReason', 'finish_reason']) ||
      firstDefined(choice, ['finishReason', 'FinishReason', 'finish_reason']) ||
      '',
  }
}

async function sendMessage() {
  const content = draft.value.trim()
  const model = selectedModelId.value.trim()
  if (!content || !model || sending.value || typing.value) {
    return
  }

  chatError.value = ''
  const nextMessages = [...requestMessages(), { role: 'user', content }]
  draft.value = ''
  sending.value = true

  try {
    const normalizedTemperature = Number(temperature.value)
    const normalizedMaxTokens = Number(maxTokens.value)
    const requestPromise = sendOpenRouterChat({
      model,
      messages: nextMessages,
      temperature: Number.isFinite(normalizedTemperature) ? normalizedTemperature : 0.7,
      maxTokens: Number.isFinite(normalizedMaxTokens) ? normalizedMaxTokens : 0,
    }).then(
      (result) => ({ result }),
      (error) => ({ error }),
    )
    messages.value.push({ role: 'user', content, justSent: !prefersReducedMotion() })
    persistConversation()
    await scrollTranscript()

    const { result, error } = await requestPromise
    if (error) {
      throw error
    }
    const normalizedResult = normalizeChatResult(result, model)
    const assistantMessage = {
      role: normalizedResult.role || 'assistant',
      content: '',
      model: normalizedResult.model || model,
      usage: normalizedResult.usage,
      finishReason: normalizedResult.finishReason,
      isTyping: true,
    }
    messages.value.push(assistantMessage)
    await scrollTranscript()
    typeAssistantMessage(assistantMessage, normalizedResult.content || 'OpenRouter 未返回文本内容')
  } catch (error) {
    chatError.value = error.message
  } finally {
    sending.value = false
  }
}
</script>

<template>
  <section class="openrouter-chat-view">
    <div class="openrouter-chat-layout">
      <aside class="openrouter-chat-side">
        <div class="openrouter-chat-search">
          <input v-model="modelSearch" type="search" placeholder="搜索模型" />
          <button
            type="button"
            :class="['openrouter-chat-filter-button', { active: freeOnly }]"
            @click="toggleFreeOnly"
          >
            免费 {{ formatNumber(freeModelCount) }}
          </button>
        </div>

        <div v-if="modelsError" class="openrouter-chat-error">
          <div class="inline-error">{{ modelsError }}</div>
          <el-button type="primary" plain @click="$emit('open-create-key')">添加 API Key</el-button>
        </div>

        <div class="openrouter-chat-model-box">
          <div class="openrouter-chat-model-count">
            <span class="openrouter-chat-model-count-text">
              <span v-if="modelsLoading" class="openrouter-loading-dot" aria-hidden="true"></span>
              {{
                modelsLoading
                  ? props.models.length
                    ? `刷新中 · ${formatNumber(filteredModels.length)} / ${formatNumber(models.length)} 个模型`
                    : '加载模型中'
                  : `${formatNumber(filteredModels.length)} / ${formatNumber(models.length)} 个模型`
              }}
            </span>
          </div>
          <div class="openrouter-chat-model-list">
            <div v-if="showModelLoadingSkeleton" class="openrouter-model-skeleton-list" aria-label="模型加载中">
              <div v-for="index in 8" :key="index" class="openrouter-model-skeleton-row">
                <span></span>
                <small></small>
              </div>
            </div>
            <template v-else>
              <button
                v-for="model in filteredModels"
                :key="model.id"
                type="button"
                :class="['openrouter-chat-model-button', { active: model.id === selectedModelId }]"
                @click="selectModel(model)"
              >
                <strong>{{ model.id }}</strong>
                <small>
                  {{ model.name || model.id }}
                  <template v-if="isFreeModel(model)"> · free</template>
                </small>
              </button>
            </template>
            <div v-if="!filteredModels.length && !modelsLoading" class="openrouter-chat-model-empty">
              未找到匹配模型
            </div>
          </div>
        </div>

        <div class="openrouter-chat-controls">
          <label class="openrouter-chat-field">
            <span>温度</span>
            <input v-model.number="temperature" class="openrouter-chat-number" type="number" min="0" max="2" step="0.1" />
          </label>
          <label class="openrouter-chat-field">
            <span>输出上限</span>
            <input v-model.number="maxTokens" class="openrouter-chat-number" type="number" min="1" max="200000" step="256" />
          </label>
        </div>
      </aside>

      <div class="openrouter-chat-main">
        <div class="openrouter-chat-toolbar">
          <div class="openrouter-chat-current">
            <strong>{{ selectedModelInfo?.name || selectedModelId || '选择模型' }}</strong>
            <span>{{ selectedModelId || '未选择模型' }}</span>
          </div>
          <div class="openrouter-chat-metrics">
            <span>{{ selectedModelInfo?.contextLength ? `${formatNumber(selectedModelInfo.contextLength)} ctx` : 'ctx -' }}</span>
            <span>{{ modelPriceLine }}</span>
            <span>{{ conversationUsageLine }}</span>
          </div>
          <div class="openrouter-chat-actions">
            <el-button :icon="Refresh" :loading="modelsLoading" @click="$emit('refresh-models')">
              {{ modelsLoading ? '刷新中' : '刷新' }}
            </el-button>
            <el-button
              :icon="Refresh"
              :loading="openRouterQuotaLoading"
              @click="refreshOpenRouterQuota"
            >
              {{ openRouterQuotaToken ? '额度' : '添加 Key' }}
            </el-button>
            <el-button :icon="Delete" :disabled="!messages.length || sending || typing" @click="resetConversation()">
              清空
            </el-button>
          </div>
        </div>

        <div class="openrouter-chat-quota-strip">
          <div class="openrouter-chat-quota-title">
            <span>OpenRouter 额度</span>
            <strong>{{ openRouterQuotaToken?.name || '未配置 API Key' }}</strong>
            <small>{{ openRouterQuotaMeta }}</small>
          </div>
          <div>
            <span>剩余</span>
            <strong>{{ openRouterQuotaRemainingText }}</strong>
          </div>
          <div>
            <span>已用</span>
            <strong>{{ openRouterQuotaUsedText }}</strong>
          </div>
          <div>
            <span>上限</span>
            <strong>{{ openRouterQuotaLimitText }}</strong>
          </div>
        </div>

        <div ref="transcriptRef" class="openrouter-chat-transcript">
          <div v-if="!messages.length" class="openrouter-chat-empty">
            <strong>{{ selectedModelId || '选择一个模型' }}</strong>
            <span>输入消息后可以直接体验当前模型</span>
          </div>

          <article
            v-for="(message, index) in messages"
            :key="`${message.role}-${index}`"
            :class="[
              'openrouter-chat-message',
              message.role,
              {
                'just-sent': message.justSent,
                streaming: message.role !== 'user' && message.isTyping,
              },
            ]"
          >
            <div class="openrouter-chat-role">
              <strong>{{ roleLabel(message.role) }}</strong>
              <span v-if="messageModelLabel(message)">{{ messageModelLabel(message) }}</span>
            </div>
            <div class="openrouter-chat-bubble">
              <p>
                <span>{{ message.content }}</span>
                <span v-if="message.isTyping" class="openrouter-chat-cursor" aria-hidden="true"></span>
              </p>
              <small v-if="tokenUsageLine(message.usage)">
                {{ tokenUsageLine(message.usage) }}
              </small>
            </div>
          </article>

          <article v-if="sending && messages.length" class="openrouter-chat-message assistant pending">
            <div class="openrouter-chat-role">
              <strong>模型</strong>
              <span>{{ selectedModelId }}</span>
            </div>
            <div class="openrouter-chat-bubble">
              <div class="openrouter-generation">
                <span class="openrouter-generation-pulse" aria-hidden="true"></span>
                <div>
                  <strong>正在生成</strong>
                  <small>等待 OpenRouter 返回</small>
                </div>
                <span class="openrouter-generation-bars" aria-hidden="true">
                  <i></i>
                  <i></i>
                  <i></i>
                </span>
              </div>
            </div>
          </article>
        </div>

        <form class="openrouter-chat-composer" @submit.prevent="sendMessage">
          <textarea
            v-model="draft"
            rows="4"
            placeholder="输入消息，Enter 发送，Shift + Enter 换行"
            :disabled="sending || typing"
            @input="updateDraft"
            @keydown.enter.exact.prevent="sendMessage"
          ></textarea>
          <div class="openrouter-chat-composer-actions">
            <span>{{ formatNumber(draftLength) }} 字</span>
            <el-button
              class="openrouter-chat-send-button"
              type="primary"
              native-type="submit"
              :icon="Connection"
              :loading="sending"
              :disabled="!canSend"
            >
              {{ typing ? '输出中' : '发送' }}
            </el-button>
          </div>
          <div v-if="chatError" class="inline-error">{{ chatError }}</div>
        </form>
      </div>
    </div>
  </section>
</template>
