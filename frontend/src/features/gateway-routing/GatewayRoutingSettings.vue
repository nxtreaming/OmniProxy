<script setup>
import { computed, ref } from 'vue'
import GeminiSelect from '../../components/GeminiSelect.vue'
import { credentialTypes, providers } from '../../constants/app'
import { gatewayPlatformPresets, routeDefinitions } from './gatewayRoutePresets'

const props = defineProps({
  config: {
    type: Object,
    required: true,
  },
})

const emit = defineEmits(['persist-config'])

const providerLabelMap = computed(() => Object.fromEntries(providers.map((item) => [item.key, item.label])))
const credentialLabelMap = computed(() => credentialTypes)
const selectedPlatformKey = ref(initialPlatformKey())
const activePlatform = computed(
  () => gatewayPlatformPresets.find((platform) => platform.key === selectedPlatformKey.value) || gatewayPlatformPresets[0],
)

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

function initialPlatformKey() {
  const candidates = [
    props.config.gatewayRoutes?.openai?.provider,
    props.config.gatewayRoutes?.claude?.provider,
    props.config.gatewayRoutes?.codex?.provider,
    props.config.gatewayRoutes?.gemini?.provider,
  ].map(normalizeProviderKey)
  return candidates.find((key) => gatewayPlatformPresets.some((platform) => platform.key === key)) || 'deepseek'
}

function platformLabel(platform) {
  return providerLabel(platform.key)
}

function platformNote(platform) {
  return providerNote(platform.key)
}

function routeDefinitionFor(key) {
  return routeDefinitions.find((route) => route.key === key)
}

function routeTitle(key) {
  return routeDefinitionFor(key)?.title || key
}

function modelRouteEntries(model) {
  return Object.entries(model.routeModels || {})
    .map(([key, modelValue]) => ({ key, model: modelValue, title: routeTitle(key) }))
    .filter((entry) => routeDefinitionFor(entry.key))
}

function applyPlatformModel(model) {
  const platform = activePlatform.value
  for (const entry of modelRouteEntries(model)) {
    const route = routeDefinitionFor(entry.key)
    const current = routeConfig(route)
    current.provider = platform.key
    current.credentialType = platform.routeCredentials?.[entry.key] || ''
    current.model = entry.model
    current.fallbacks = fallbackConfigs(route).filter(
      (fallback) => normalizeProviderKey(fallback.provider) !== normalizeProviderKey(platform.key),
    )
  }
  emit('persist-config')
}

function isModelApplied(model) {
  const platform = activePlatform.value
  const entries = modelRouteEntries(model)
  return entries.length > 0 && entries.every((entry) => {
    const route = routeDefinitionFor(entry.key)
    const current = routeConfig(route)
    return normalizeProviderKey(current.provider) === platform.key &&
      String(current.model || '').trim().toLowerCase() === String(entry.model || '').trim().toLowerCase()
  })
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
        <p>按平台选择模型，一键写入对应网关入口。</p>
      </div>
    </div>
    <div class="gateway-quick-config">
      <div class="gateway-quick-platforms" aria-label="选择后端平台">
        <button
          v-for="platform in gatewayPlatformPresets"
          :key="platform.key"
          type="button"
          :class="['gateway-platform-card', { active: activePlatform.key === platform.key }]"
          @click="selectedPlatformKey = platform.key"
        >
          <strong>{{ platformLabel(platform) }}</strong>
          <small>{{ platformNote(platform) }}</small>
        </button>
      </div>

      <div class="gateway-model-config-panel">
        <div class="gateway-model-config-head">
          <div>
            <strong>{{ platformLabel(activePlatform) }}</strong>
            <span>{{ platformNote(activePlatform) }}</span>
          </div>
          <small>{{ activePlatform.models.length }} 个模型</small>
        </div>
        <div class="gateway-model-card-list">
          <button
            v-for="model in activePlatform.models"
            :key="model.id"
            type="button"
            :class="['gateway-model-card', { active: isModelApplied(model) }]"
            @click="applyPlatformModel(model)"
          >
            <span>
              <strong>{{ model.label }}</strong>
              <small>{{ Object.values(model.routeModels).join(' / ') }}</small>
            </span>
            <span class="gateway-model-targets">
              <small v-for="entry in modelRouteEntries(model)" :key="`${model.id}-${entry.key}`">
                {{ entry.title }}
              </small>
            </span>
          </button>
        </div>
      </div>
    </div>

    <div class="gateway-advanced-head">
      <strong>高级路由微调</strong>
      <span>备用链、凭据类型和自定义模型</span>
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
