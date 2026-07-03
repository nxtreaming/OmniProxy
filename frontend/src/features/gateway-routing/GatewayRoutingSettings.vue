<script setup>
import { computed } from 'vue'
import GeminiSelect from '../../components/GeminiSelect.vue'
import { credentialTypes, providers } from '../../constants/app'

const props = defineProps({
  config: {
    type: Object,
    required: true,
  },
})

const providerLabelMap = computed(() => Object.fromEntries(providers.map((item) => [item.key, item.label])))
const credentialLabelMap = computed(() => credentialTypes)

const openAICompatibleProviders = [
  'openai',
  'deepseek',
  'kimi',
  'xiaomi',
  'zhipu',
  'minimax',
  'openrouter',
  'tokenrouter',
  'sub2api',
  'newapi',
  'anyrouter',
  'zo',
  'prem',
  'custom',
]

const routeDefinitions = [
  {
    key: 'codex',
    title: 'Codex',
    protocol: 'OpenAI Responses',
    endpoint: (port) => `http://127.0.0.1:${port}/codex/v1`,
    fallback: { provider: 'openai', credentialType: 'codex_auth_json', model: 'gpt-5.4' },
    providers: openAICompatibleProviders,
    modelPresets: ['gpt-5.4', 'gpt-5.4-high', 'gpt-5.5', 'gpt-5.5-high', 'gpt-5-codex'],
  },
  {
    key: 'claude',
    title: 'Claude Code / Desktop',
    protocol: 'Anthropic Messages',
    endpoint: (port) => `http://127.0.0.1:${port}/anthropic-router`,
    fallback: { provider: 'anthropic', credentialType: 'api_key', model: 'claude-sonnet-4-5-20250929' },
    providers: [
      'anthropic',
      'deepseek',
      'kimi',
      'xiaomi',
      'zhipu',
      'minimax',
      'sub2api',
      'newapi',
      'anyrouter',
      'zo',
      'prem',
      'custom',
    ],
    modelPresets: [
      'claude-sonnet-4-5-20250929',
      'claude-opus-4-7',
      'claude-sonnet-4-6',
      'deepseek-v4-pro[1m]',
      'mimo-v2.5-pro[1m]',
      'kimi-for-coding',
      'glm-5.1',
    ],
  },
  {
    key: 'openai',
    title: 'OpenAI 兼容',
    protocol: 'Chat / Responses',
    endpoint: (port) => `http://127.0.0.1:${port}/opencode-router/v1`,
    fallback: { provider: 'openai', credentialType: 'api_key', model: 'gpt-5.4' },
    providers: openAICompatibleProviders,
    modelPresets: ['gpt-5.4', 'gpt-5.4-high', 'gpt-5.5', 'gpt-5.5-high', 'deepseek-v4-pro[1m]', 'kimi-for-coding', 'glm-5.1', 'MiniMax-M2.7'],
  },
  {
    key: 'gemini',
    title: 'Gemini CLI',
    protocol: 'Gemini Native',
    endpoint: (port) => `http://127.0.0.1:${port}/gemini`,
    fallback: { provider: 'gemini', credentialType: 'api_key', model: 'gemini-3-pro-preview' },
    providers: ['gemini', 'sub2api', 'newapi'],
    modelPresets: ['gemini-3-pro-preview', 'gemini-3-flash-preview', 'gemini-2.5-pro', 'gemini-2.5-flash'],
  },
]

function routeConfig(route) {
  if (!props.config.gatewayRoutes || typeof props.config.gatewayRoutes !== 'object') {
    props.config.gatewayRoutes = {}
  }
  if (!props.config.gatewayRoutes[route.key] || typeof props.config.gatewayRoutes[route.key] !== 'object') {
    props.config.gatewayRoutes[route.key] = { ...route.fallback, fallbacks: [] }
  }
  if (!Array.isArray(props.config.gatewayRoutes[route.key].fallbacks)) {
    props.config.gatewayRoutes[route.key].fallbacks = []
  }
  return props.config.gatewayRoutes[route.key]
}

function routeProviderOptions(route) {
  return route.providers.map((key) => ({
    value: key,
    label: providerLabelMap.value[key] || key,
    description: providerNote(key),
  }))
}

function credentialOptions(provider) {
  const normalized = String(provider || '').trim().toLowerCase()
  const supported = {
    openai: ['api_key', 'codex_auth_json'],
    anthropic: ['api_key', 'claude_oauth_json'],
    xiaomi: ['api_key', 'mimo_token_plan'],
    zhipu: ['api_key', 'coding_plan'],
  }[normalized] || ['api_key']
  return [
    { value: '', label: '自动匹配' },
    ...supported.map((key) => ({ value: key, label: credentialLabelMap.value[key] || key })),
  ]
}

function setRouteProvider(route, provider) {
  const current = routeConfig(route)
  current.provider = provider
  current.credentialType = ''
  current.fallbacks = fallbackConfigs(route).filter((fallback) => normalizeProviderKey(fallback.provider) !== normalizeProviderKey(provider))
}

function setRouteCredential(route, credentialType) {
  routeConfig(route).credentialType = credentialType
}

function setRouteModel(route, model) {
  routeConfig(route).model = model
}

function isRouteModelSelected(route, model) {
  return String(routeConfig(route).model || '').trim().toLowerCase() === String(model || '').trim().toLowerCase()
}

function routeEndpoint(route) {
  return route.endpoint(Number(props.config.proxyPort) || 3000)
}

function providerNote(provider) {
  return providers.find((item) => item.key === provider)?.note || ''
}

function providerLabel(provider) {
  return providerLabelMap.value[provider] || provider
}

function normalizeProviderKey(provider) {
  return String(provider || '').trim().toLowerCase()
}

function fallbackConfigs(route) {
  return routeConfig(route).fallbacks
}

function routeChain(route) {
  const current = routeConfig(route)
  return [
    { provider: current.provider, label: providerLabel(current.provider), type: '主' },
    ...fallbackConfigs(route).map((fallback, index) => ({
      provider: fallback.provider,
      label: providerLabel(fallback.provider),
      type: `备 ${index + 1}`,
    })),
  ].filter((item) => item.provider)
}

function isPrimaryProvider(route, provider) {
  return normalizeProviderKey(routeConfig(route).provider) === normalizeProviderKey(provider)
}

function isFallbackProviderSelected(route, provider) {
  const key = normalizeProviderKey(provider)
  return fallbackConfigs(route).some((fallback) => normalizeProviderKey(fallback.provider) === key)
}

function toggleFallbackProvider(route, provider) {
  if (isPrimaryProvider(route, provider)) return
  const current = routeConfig(route)
  const key = normalizeProviderKey(provider)
  if (isFallbackProviderSelected(route, provider)) {
    current.fallbacks = current.fallbacks.filter((fallback) => normalizeProviderKey(fallback.provider) !== key)
    return
  }
  current.fallbacks = [
    ...fallbackConfigs(route),
    { provider, credentialType: '', model: '' },
  ]
}

function setFallbackCredential(route, provider, credentialType) {
  const fallback = fallbackConfigs(route).find((item) => normalizeProviderKey(item.provider) === normalizeProviderKey(provider))
  if (!fallback) return
  fallback.credentialType = credentialType
}

function setFallbackModel(route, provider, model) {
  const fallback = fallbackConfigs(route).find((item) => normalizeProviderKey(item.provider) === normalizeProviderKey(provider))
  if (!fallback) return
  fallback.model = model
}

function fallbackModelPlaceholder(route) {
  return `继承主默认模型：${routeConfig(route).model || route.fallback.model}`
}

function removeFallbackProvider(route, provider) {
  const key = normalizeProviderKey(provider)
  routeConfig(route).fallbacks = fallbackConfigs(route).filter((fallback) => normalizeProviderKey(fallback.provider) !== key)
}

function moveFallbackProvider(route, provider, direction) {
  const current = routeConfig(route)
  const index = fallbackConfigs(route).findIndex((fallback) => normalizeProviderKey(fallback.provider) === normalizeProviderKey(provider))
  const nextIndex = index + direction
  if (index < 0 || nextIndex < 0 || nextIndex >= current.fallbacks.length) return
  const next = [...current.fallbacks]
  const [item] = next.splice(index, 1)
  next.splice(nextIndex, 0, item)
  current.fallbacks = next
}
</script>

<template>
  <section class="settings-section settings-gateway-section">
    <div class="settings-section-head">
      <div>
        <h3>网关路由</h3>
        <p>客户端固定接入本地网关；后端厂商、凭据类型和默认模型在这里统一切换。</p>
      </div>
    </div>
    <div class="gateway-route-list">
      <article v-for="route in routeDefinitions" :key="route.key" class="gateway-route-row">
        <div class="gateway-route-summary">
          <div>
            <strong>{{ route.title }}</strong>
            <small>{{ route.protocol }}</small>
          </div>
          <div class="gateway-route-meta">
            <code>{{ routeEndpoint(route) }}</code>
            <div class="gateway-route-chain" aria-label="当前路由顺序">
              <template v-for="(item, index) in routeChain(route)" :key="`${route.key}-${item.type}-${item.provider}`">
                <span v-if="index" class="gateway-route-chain-arrow">→</span>
                <span class="gateway-route-chain-item">
                  <small>{{ item.type }}</small>
                  {{ item.label }}
                </span>
              </template>
            </div>
          </div>
        </div>
        <div class="gateway-route-controls">
          <label>
            <span>主后端厂商</span>
            <GeminiSelect
              :model-value="routeConfig(route).provider"
              :options="routeProviderOptions(route)"
              :aria-label="`${route.title} 主后端厂商`"
              @update:modelValue="setRouteProvider(route, $event)"
            />
          </label>
          <label>
            <span>凭据类型</span>
            <GeminiSelect
              :model-value="routeConfig(route).credentialType || ''"
              :options="credentialOptions(routeConfig(route).provider)"
              :aria-label="`${route.title} 凭据类型`"
              @update:modelValue="setRouteCredential(route, $event)"
            />
          </label>
          <div class="gateway-route-backend-field">
            <div class="gateway-route-backend-head">
              <span>备用后端厂商</span>
              <small>{{ fallbackConfigs(route).length }} 个备用</small>
            </div>
            <div class="gateway-provider-chip-list">
              <button
                v-for="provider in routeProviderOptions(route)"
                :key="provider.value"
                type="button"
                class="gateway-provider-chip"
                :class="{ active: isFallbackProviderSelected(route, provider.value), primary: isPrimaryProvider(route, provider.value) }"
                :disabled="isPrimaryProvider(route, provider.value)"
                @click="toggleFallbackProvider(route, provider.value)"
              >
                <span>{{ provider.label }}</span>
                <small v-if="isPrimaryProvider(route, provider.value)">主后端</small>
              </button>
            </div>
            <div v-if="fallbackConfigs(route).length" class="gateway-fallback-list">
              <div v-for="(fallback, index) in fallbackConfigs(route)" :key="fallback.provider" class="gateway-fallback-row">
                <div class="gateway-fallback-title">
                  <strong>{{ providerLabel(fallback.provider) }}</strong>
                  <small>备用 {{ index + 1 }}</small>
                </div>
                <GeminiSelect
                  :model-value="fallback.credentialType || ''"
                  :options="credentialOptions(fallback.provider)"
                  :aria-label="`${route.title} ${providerLabel(fallback.provider)} 备用凭据类型`"
                  @update:modelValue="setFallbackCredential(route, fallback.provider, $event)"
                />
                <label class="gateway-fallback-model">
                  <span>备用模型</span>
                  <input
                    :value="fallback.model || ''"
                    type="text"
                    :placeholder="fallbackModelPlaceholder(route)"
                    @input="setFallbackModel(route, fallback.provider, $event.target.value)"
                  />
                </label>
                <div class="gateway-fallback-actions" aria-label="调整备用路由顺序">
                  <button type="button" :disabled="index === 0" @click="moveFallbackProvider(route, fallback.provider, -1)">上移</button>
                  <button type="button" :disabled="index === fallbackConfigs(route).length - 1" @click="moveFallbackProvider(route, fallback.provider, 1)">下移</button>
                  <button type="button" class="danger" @click="removeFallbackProvider(route, fallback.provider)">移除</button>
                </div>
              </div>
            </div>
          </div>
          <div class="gateway-route-model-field">
            <span>默认模型</span>
            <div class="gateway-model-preset-list">
              <button
                v-for="model in route.modelPresets"
                :key="model"
                type="button"
                class="gateway-model-preset"
                :class="{ active: isRouteModelSelected(route, model) }"
                @click="setRouteModel(route, model)"
              >
                {{ model }}
              </button>
            </div>
            <input
              :value="routeConfig(route).model"
              type="text"
              :placeholder="`自定义模型 ID，例如 ${route.fallback.model}`"
              @input="setRouteModel(route, $event.target.value)"
            />
          </div>
        </div>
      </article>
    </div>
  </section>
</template>

<style src="./GatewayRoutingSettings.css"></style>
