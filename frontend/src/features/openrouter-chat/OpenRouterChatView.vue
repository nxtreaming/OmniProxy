<script setup>
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { Connection } from '@element-plus/icons-vue'
import OpenRouterChatHeader from './OpenRouterChatHeader.vue'
import OpenRouterModelSidebar from './OpenRouterModelSidebar.vue'
import { sendOpenRouterChat } from '../../services/api'
import {
  isFreeModel,
  normalizeChatResult,
  numberValue,
  openRouterBalanceLimit,
  openRouterBalanceRemaining,
  openRouterBalanceValue,
  tokenUsageLine,
} from '../../utils/openrouterChat'

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
      <OpenRouterModelSidebar
        :models="models"
        :filtered-models="filteredModels"
        :model-search="modelSearch"
        :free-only="freeOnly"
        :free-model-count="freeModelCount"
        :models-loading="modelsLoading"
        :models-error="modelsError"
        :show-model-loading-skeleton="showModelLoadingSkeleton"
        :selected-model-id="selectedModelId"
        :temperature="temperature"
        :max-tokens="maxTokens"
        :format-number="formatNumber"
        @update:model-search="modelSearch = $event"
        @update:temperature="temperature = $event"
        @update:max-tokens="maxTokens = $event"
        @toggle-free-only="toggleFreeOnly"
        @select-model="selectModel"
        @open-create-key="$emit('open-create-key')"
      />

      <div class="openrouter-chat-main">
        <OpenRouterChatHeader
          :selected-model-info="selectedModelInfo"
          :selected-model-id="selectedModelId"
          :model-price-line="modelPriceLine"
          :conversation-usage-line="conversationUsageLine"
          :models-loading="modelsLoading"
          :open-router-quota-loading="openRouterQuotaLoading"
          :open-router-quota-token="openRouterQuotaToken"
          :open-router-quota-meta="openRouterQuotaMeta"
          :open-router-quota-remaining-text="openRouterQuotaRemainingText"
          :open-router-quota-used-text="openRouterQuotaUsedText"
          :open-router-quota-limit-text="openRouterQuotaLimitText"
          :messages-count="messages.length"
          :sending="sending"
          :typing="typing"
          :format-number="formatNumber"
          @refresh-models="$emit('refresh-models')"
          @refresh-quota="refreshOpenRouterQuota"
          @reset-conversation="resetConversation()"
        />

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
              <small v-if="tokenUsageLine(message.usage, props.formatNumber)">
                {{ tokenUsageLine(message.usage, props.formatNumber) }}
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

<style src="./OpenRouterChatView.css"></style>
