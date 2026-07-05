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
const gatewayModelGroups = computed(() => buildGatewayModelGroups())
const selectedModelKey = ref(initialModelKey())
const activeModelGroup = computed(() => {
  const groups = gatewayModelGroups.value
  return groups.find((group) => group.key === selectedModelKey.value) || groups[0] || { providers: [], routeModels: {} }
})
const directClaudeProviders = new Set(['anthropic', 'deepseek', 'kimi', 'xiaomi', 'zhipu', 'minimax', 'zo'])
const directOpenAIProviders = new Set(['openai', 'deepseek', 'kimi', 'xiaomi', 'zhipu', 'minimax', 'openrouter', 'tokenrouter', 'zo', 'custom'])

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

function initialModelKey() {
  const groups = buildGatewayModelGroups()
  return groups.find((group) => group.providers.some((option) => isModelProviderApplied(option)))?.key ||
    groups.find((group) => group.providers.some((option) => option.key === 'deepseek'))?.key ||
    groups[0]?.key ||
    ''
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

function buildGatewayModelGroups() {
  const byKey = new Map()
  for (const platform of gatewayPlatformPresets) {
    for (const model of platform.models || []) {
      const key = modelGroupKey(model)
      if (!key) continue
      if (!byKey.has(key)) {
        byKey.set(key, {
          key,
          label: model.label || modelGroupSummary(model),
          routeModels: { ...model.routeModels },
          providers: [],
        })
      }
      const group = byKey.get(key)
      if (!group.providers.some((option) => option.key === platform.key)) {
        group.providers.push({ key: platform.key, platform, model })
      }
    }
  }
  return Array.from(byKey.values())
}

function modelGroupKey(model) {
  return Object.entries(model.routeModels || {})
    .map(([routeKey, modelValue]) => `${routeKey}:${String(modelValue || '').trim().toLowerCase()}`)
    .sort()
    .join('|')
}

function modelGroupSummary(model) {
  return Array.from(new Set(Object.values(model.routeModels || {}).map((value) => String(value || '').trim()).filter(Boolean))).join(' / ')
}

function selectModelGroup(group) {
  selectedModelKey.value = group.key
}

function applyModelProvider(option) {
  const platform = option.platform
  for (const entry of modelRouteEntries(option.model)) {
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

function isModelProviderApplied(option) {
  const entries = modelRouteEntries(option.model)
  return entries.length > 0 && entries.every((entry) => {
    const route = routeDefinitionFor(entry.key)
    const current = routeConfig(route)
    return normalizeProviderKey(current.provider) === option.key &&
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

function inferredProviderForRouteModel(route) {
  const model = String(routeConfig(route).model || '').trim().toLowerCase()
  if (!model) return ''
  if (route.key === 'claude') {
    return inferredClaudeProvider(model)
  }
  if (route.key === 'codex' || route.key === 'openai') {
    return inferredOpenAIProvider(model)
  }
  return ''
}

function inferredClaudeProvider(model) {
  if (model.startsWith('mimo-')) return 'xiaomi'
  if (model.startsWith('deepseek-')) return 'deepseek'
  if (model.startsWith('kimi-')) return 'kimi'
  if (model.startsWith('glm-') || model.startsWith('zhipu-')) return 'zhipu'
  if (model.startsWith('minimax-')) return 'minimax'
  if (model === 'claude-opus-4-7' || model === 'claude-sonnet-4-6') return 'zo'
  return 'anthropic'
}

function inferredOpenAIProvider(model) {
  if (model.startsWith('mimo-')) return 'xiaomi'
  if (model.startsWith('deepseek-')) return 'deepseek'
  if (model.startsWith('kimi-')) return 'kimi'
  if (model.startsWith('glm-') || model.startsWith('zhipu-')) return 'zhipu'
  if (model.startsWith('minimax-')) return 'minimax'
  if (model.startsWith('auto:') || model.startsWith('tokenrouter:') || model.startsWith('tokenrouter/')) return 'tokenrouter'
  if (model.includes('/')) return 'openrouter'
  if (model.startsWith('custom-')) return 'custom'
  return 'openai'
}

function effectivePrimaryProvider(route) {
  const current = normalizeProviderKey(routeConfig(route).provider)
  const inferred = inferredProviderForRouteModel(route)
  if (!inferred || inferred === current) return current
  if (route.key === 'claude') {
    if (inferred === 'anthropic' && current !== 'anthropic') return current
    return directClaudeProviders.has(current) && directClaudeProviders.has(inferred) ? inferred : current
  }
  if (route.key === 'codex' || route.key === 'openai') {
    if (inferred === 'openai' && current !== 'openai') return current
    return directOpenAIProviders.has(current) && directOpenAIProviders.has(inferred) ? inferred : current
  }
  return current
}

function routeModelHint(route) {
  const current = normalizeProviderKey(routeConfig(route).provider)
  const effective = effectivePrimaryProvider(route)
  if (!current || !effective || current === effective) return ''
  return `当前模型命中 ${providerLabel(effective)}，配置主后端为 ${providerLabel(current)}`
}

function fallbackConfigs(route) {
  return routeConfig(route).fallbacks
}

function routeChain(route) {
  const current = routeConfig(route)
  const effectiveProvider = effectivePrimaryProvider(route)
  return [
    {
      provider: effectiveProvider || current.provider,
      label: providerLabel(effectiveProvider || current.provider),
      type: normalizeProviderKey(current.provider) === effectiveProvider ? '主' : '模型',
    },
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
        <h3>模型路由</h3>
        <p>先选择模型，再选择提供该模型的网关；本地网关负责转发、记录和按备用链重试。</p>
      </div>
    </div>
    <div class="gateway-quick-config">
      <div class="gateway-model-index" aria-label="选择模型">
        <button
          v-for="group in gatewayModelGroups"
          :key="group.key"
          type="button"
          :class="['gateway-model-option-card', { active: activeModelGroup.key === group.key }]"
          @click="selectModelGroup(group)"
        >
          <strong>{{ group.label }}</strong>
          <small>{{ modelGroupSummary(group) }}</small>
        </button>
      </div>

      <div class="gateway-model-config-panel">
        <div class="gateway-model-config-head">
          <div>
            <strong>{{ activeModelGroup.label || '选择模型' }}</strong>
            <span>{{ modelGroupSummary(activeModelGroup) }}</span>
          </div>
          <small>{{ activeModelGroup.providers.length }} 个可用网关</small>
        </div>
        <div class="gateway-provider-card-list">
          <button
            v-for="option in activeModelGroup.providers"
            :key="`${activeModelGroup.key}-${option.key}`"
            type="button"
            :class="['gateway-provider-card', { active: isModelProviderApplied(option) }]"
            @click="applyModelProvider(option)"
          >
            <span>
              <strong>{{ providerLabel(option.key) }}</strong>
              <small>{{ providerNote(option.key) }}</small>
            </span>
            <span class="gateway-model-targets">
              <small v-for="entry in modelRouteEntries(option.model)" :key="`${option.key}-${entry.key}`">
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
            <small v-if="routeModelHint(route)" class="gateway-route-hint">{{ routeModelHint(route) }}</small>
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
