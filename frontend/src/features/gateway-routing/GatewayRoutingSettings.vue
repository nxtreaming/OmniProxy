<script setup>
import { computed, ref, watch } from 'vue'
import GeminiSelect from '../../components/GeminiSelect.vue'
import { credentialTypes, providers } from '../../constants/app'
import { gatewayPlatformPresets, routeDefinitions } from './gatewayRoutePresets'

const props = defineProps({
  config: {
    type: Object,
    required: true,
  },
})

const providerLabelMap = computed(() => Object.fromEntries(providers.map((item) => [item.key, item.label])))
const credentialLabelMap = computed(() => credentialTypes)
const gatewayModelGroups = computed(() => buildGatewayModelGroups())
const selectedModelKey = ref('')
const modelSearchInput = ref('')
const filteredGatewayModelGroups = computed(() => {
  const query = modelKey(modelSearchInput.value)
  if (!query) return gatewayModelGroups.value
  return gatewayModelGroups.value.filter((group) =>
    modelKey(`${group.label} ${group.modelId}`).includes(query),
  )
})
const canAddCustomModel = computed(() => {
  const key = modelKey(modelSearchInput.value)
  return Boolean(key && !gatewayModelGroups.value.some((group) => group.key === key) && filteredGatewayModelGroups.value.length === 0)
})
const activeModelGroup = computed(() => {
  const groups = gatewayModelGroups.value
  return groups.find((group) => group.key === selectedModelKey.value) || groups[0] || emptyModelGroup()
})
const activeModelRoute = computed(() => modelRouteConfig(activeModelGroup.value))

watch(
  gatewayModelGroups,
  (groups) => {
    if (groups.some((group) => group.key === selectedModelKey.value)) return
    selectedModelKey.value = initialModelKey(groups)
  },
  { immediate: true },
)

function emptyModelGroup() {
  return { key: '', modelId: '', label: '选择模型', providers: [] }
}

function modelRoutesConfig() {
  if (!props.config.modelRoutes || typeof props.config.modelRoutes !== 'object') {
    props.config.modelRoutes = {}
  }
  return props.config.modelRoutes
}

function routeConfig(route) {
  if (!props.config.gatewayRoutes || typeof props.config.gatewayRoutes !== 'object') {
    props.config.gatewayRoutes = {}
  }
  if (!props.config.gatewayRoutes[route.key] || typeof props.config.gatewayRoutes[route.key] !== 'object') {
    props.config.gatewayRoutes[route.key] = { ...route.fallback, fallbacks: [] }
  }
  return props.config.gatewayRoutes[route.key]
}

function modelRouteConfig(group) {
  const routes = modelRoutesConfig()
  const key = group.key
  if (!key) return { provider: '', credentialType: '', model: '', fallbacks: [] }
  if (!routes[key] || typeof routes[key] !== 'object') {
    const firstProvider = group.providers[0]?.key || ''
    routes[key] = {
      provider: firstProvider,
      credentialType: defaultCredentialType(firstProvider),
      model: group.modelId,
      fallbacks: [],
    }
  }
  if (!Array.isArray(routes[key].fallbacks)) {
    routes[key].fallbacks = []
  }
  if (!routes[key].model) {
    routes[key].model = group.modelId
  }
  return routes[key]
}

function readModelRoute(group) {
  return modelRoutesConfig()[group.key]
}

function initialModelKey(groups = buildGatewayModelGroups()) {
  return groups.find((group) => {
    const route = readModelRoute(group)
    return route?.provider
  })?.key || groups.find((group) => group.modelId === 'deepseek-v4-pro')?.key || groups[0]?.key || ''
}

function buildGatewayModelGroups() {
  const byKey = new Map()
  for (const platform of gatewayPlatformPresets) {
    for (const model of platform.models || []) {
      for (const modelId of modelRouteModelIDs(model)) {
        const key = modelKey(modelId)
        if (!key) continue
        if (!byKey.has(key)) {
          byKey.set(key, {
            key,
            modelId,
            label: model.label || modelId,
            providers: [],
          })
        }
        const group = byKey.get(key)
        if (!group.providers.some((option) => option.key === platform.key)) {
          group.providers.push({ key: platform.key, platform, modelId })
        }
      }
    }
  }
  for (const [rawModel, route] of Object.entries(modelRoutesConfig())) {
    const modelId = String(rawModel || route?.model || '').trim()
    const key = modelKey(modelId)
    if (!key || byKey.has(key)) continue
    byKey.set(key, {
      key,
      modelId,
      label: modelId,
      providers: modelRouteProviderKeys().map((provider) => ({ key: provider, modelId })),
    })
  }
  return Array.from(byKey.values())
}

function modelRouteModelIDs(model) {
  return Array.from(new Set(Object.values(model.routeModels || {}).map((value) => String(value || '').trim()).filter(Boolean)))
}

function modelKey(model) {
  return String(model || '').trim().toLowerCase()
}

function selectModelGroup(group) {
  selectedModelKey.value = group.key
}

function confirmModelSearch() {
  const key = modelKey(modelSearchInput.value)
  if (!key) return
  const exact = gatewayModelGroups.value.find((group) => group.key === key)
  if (exact) {
    selectModelGroup(exact)
    modelSearchInput.value = ''
    return
  }
  if (filteredGatewayModelGroups.value.length === 1) {
    selectModelGroup(filteredGatewayModelGroups.value[0])
    modelSearchInput.value = ''
    return
  }
  if (filteredGatewayModelGroups.value.length === 0) {
    addCustomModelRoute()
  }
}

function addCustomModelRoute() {
  const modelId = String(modelSearchInput.value || '').trim()
  const key = modelKey(modelId)
  if (!key) return
  const routes = modelRoutesConfig()
  if (!routes[key]) {
    const provider = inferredProviderForModel(modelId)
    routes[key] = {
      provider,
      credentialType: defaultCredentialType(provider),
      model: modelId,
      fallbacks: [],
    }
  }
  modelSearchInput.value = ''
  selectedModelKey.value = key
}

function providerLabel(provider) {
  return providerLabelMap.value[provider] || provider
}

function providerNote(provider) {
  return providers.find((item) => item.key === provider)?.note || ''
}

function normalizeProviderKey(provider) {
  return String(provider || '').trim().toLowerCase()
}

function defaultCredentialType(provider) {
  const normalized = normalizeProviderKey(provider)
  if (normalized === 'openai' || normalized === 'anthropic') return 'api_key'
  return ''
}

function modelRouteProviderKeys() {
  const keys = []
  for (const platform of gatewayPlatformPresets) {
    if (platform.key) keys.push(platform.key)
  }
  return Array.from(new Set(keys.map(normalizeProviderKey).filter(Boolean)))
}

function inferredProviderForModel(model) {
  const normalized = String(model || '').trim().toLowerCase()
  if (normalized.startsWith('claude-')) return 'anthropic'
  if (normalized.startsWith('deepseek-')) return 'deepseek'
  if (normalized.startsWith('kimi-')) return 'kimi'
  if (normalized.startsWith('mimo-')) return 'xiaomi'
  if (normalized.startsWith('glm-') || normalized.startsWith('zhipu-')) return 'zhipu'
  if (normalized.startsWith('minimax-')) return 'minimax'
  if (normalized.startsWith('gemini-')) return 'gemini'
  if (normalized.startsWith('auto:') || normalized.startsWith('tokenrouter:') || normalized.startsWith('tokenrouter/')) return 'tokenrouter'
  if (normalized.includes('/')) return 'openrouter'
  if (normalized.startsWith('custom-')) return 'custom'
  return 'openai'
}

function credentialOptions(provider) {
  const normalized = normalizeProviderKey(provider)
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

function activeProviderOptions() {
  const route = activeModelRoute.value
  const keys = []
  for (const option of activeModelGroup.value.providers) keys.push(option.key)
  if (route.provider) keys.push(route.provider)
  for (const fallback of route.fallbacks || []) keys.push(fallback.provider)
  return Array.from(new Set(keys.map(normalizeProviderKey).filter(Boolean))).map((key) => ({
    value: key,
    label: providerLabel(key),
    description: providerNote(key),
  }))
}

function applyModelProvider(option) {
  const route = modelRouteConfig(activeModelGroup.value)
  route.provider = option.key
  route.credentialType = defaultCredentialType(option.key)
  route.model = option.modelId
  route.fallbacks = fallbackConfigs().filter(
    (fallback) => normalizeProviderKey(fallback.provider) !== normalizeProviderKey(option.key),
  )
}

function isModelProviderApplied(option) {
  const route = readModelRoute(activeModelGroup.value)
  return normalizeProviderKey(route?.provider) === normalizeProviderKey(option.key) &&
    String(route?.model || '').trim().toLowerCase() === String(option.modelId || '').trim().toLowerCase()
}

function setModelRouteProvider(provider) {
  const route = activeModelRoute.value
  route.provider = provider
  route.credentialType = defaultCredentialType(provider)
  route.fallbacks = fallbackConfigs().filter((fallback) => normalizeProviderKey(fallback.provider) !== normalizeProviderKey(provider))
}

function setModelRouteCredential(credentialType) {
  activeModelRoute.value.credentialType = credentialType
}

function setModelRouteModel(model) {
  activeModelRoute.value.model = model
}

function fallbackConfigs() {
  return activeModelRoute.value.fallbacks || []
}

function fallbackProviderOptions() {
  return activeProviderOptions().map((option) => {
    const primary = isPrimaryProvider(option.value)
    const selected = isFallbackProviderSelected(option.value)
    return {
      ...option,
      disabled: primary || selected,
      description: primary ? '当前主后端' : selected ? '已在备用链中' : option.description,
    }
  })
}

function isPrimaryProvider(provider) {
  return normalizeProviderKey(activeModelRoute.value.provider) === normalizeProviderKey(provider)
}

function isFallbackProviderSelected(provider) {
  const key = normalizeProviderKey(provider)
  return fallbackConfigs().some((fallback) => normalizeProviderKey(fallback.provider) === key)
}

function addFallbackProvider(provider) {
  if (!provider || isPrimaryProvider(provider) || isFallbackProviderSelected(provider)) return
  const route = activeModelRoute.value
  route.fallbacks = [
    ...fallbackConfigs(),
    { provider, credentialType: defaultCredentialType(provider), model: route.model || activeModelGroup.value.modelId },
  ]
}

function setFallbackCredential(provider, credentialType) {
  const fallback = fallbackConfigs().find((item) => normalizeProviderKey(item.provider) === normalizeProviderKey(provider))
  if (!fallback) return
  fallback.credentialType = credentialType
}

function setFallbackModel(provider, model) {
  const fallback = fallbackConfigs().find((item) => normalizeProviderKey(item.provider) === normalizeProviderKey(provider))
  if (!fallback) return
  fallback.model = model
}

function fallbackModelPlaceholder() {
  return `继承模型：${activeModelRoute.value.model || activeModelGroup.value.modelId}`
}

function removeFallbackProvider(provider) {
  const key = normalizeProviderKey(provider)
  activeModelRoute.value.fallbacks = fallbackConfigs().filter((fallback) => normalizeProviderKey(fallback.provider) !== key)
}

function moveFallbackProvider(provider, direction) {
  const route = activeModelRoute.value
  const index = fallbackConfigs().findIndex((fallback) => normalizeProviderKey(fallback.provider) === normalizeProviderKey(provider))
  const nextIndex = index + direction
  if (index < 0 || nextIndex < 0 || nextIndex >= route.fallbacks.length) return
  const next = [...route.fallbacks]
  const [item] = next.splice(index, 1)
  next.splice(nextIndex, 0, item)
  route.fallbacks = next
}

function modelRouteChain() {
  const route = activeModelRoute.value
  return [
    { provider: route.provider, label: providerLabel(route.provider), type: '主' },
    ...fallbackConfigs().map((fallback, index) => ({
      provider: fallback.provider,
      label: providerLabel(fallback.provider),
      type: `备 ${index + 1}`,
    })),
  ].filter((item) => item.provider)
}

function routeEndpoint(route) {
  return route.endpoint(Number(props.config.proxyPort) || 3000)
}

function setClientDefaultModel(route, model) {
  routeConfig(route).model = model
}

function isClientDefaultModelSelected(route, model) {
  return String(routeConfig(route).model || '').trim().toLowerCase() === String(model || '').trim().toLowerCase()
}
</script>

<template>
  <section class="settings-section settings-gateway-section">
    <div class="settings-section-head">
      <div>
        <h3>模型路由</h3>
        <p>客户端发送模型名；这里按模型配置主后端和备用链，例如 DeepSeek 官方 API → Prem。</p>
      </div>
    </div>
    <div class="gateway-route-model-field">
      <span>搜索或添加模型</span>
      <div class="gateway-model-preset-list">
        <input
          v-model="modelSearchInput"
          type="text"
          placeholder="搜索 gpt、deepseek、claude；输入不存在的模型后回车添加"
          @keydown.enter.prevent="confirmModelSearch"
        />
        <button type="button" class="gateway-model-preset" :disabled="!canAddCustomModel" @click="addCustomModelRoute">
          添加为自定义模型
        </button>
      </div>
    </div>
    <div class="gateway-quick-config">
      <div class="gateway-model-index" aria-label="选择模型">
        <button
          v-for="group in filteredGatewayModelGroups"
          :key="group.key"
          type="button"
          :class="['gateway-model-option-card', { active: activeModelGroup.key === group.key }]"
          @click="selectModelGroup(group)"
        >
          <strong>{{ group.label }}</strong>
          <small>{{ group.modelId }}</small>
        </button>
        <div v-if="!filteredGatewayModelGroups.length" class="gateway-empty-state">
          没有匹配模型；可点击“添加为自定义模型”创建。
        </div>
      </div>

      <div class="gateway-model-config-panel">
        <div class="gateway-model-config-head">
          <div>
            <strong>{{ activeModelGroup.label }}</strong>
            <span>{{ activeModelGroup.modelId }}</span>
          </div>
          <small>{{ activeModelGroup.providers.length }} 个可用后端</small>
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
              <small>{{ isModelProviderApplied(option) ? '当前主后端' : '设为主后端' }}</small>
            </span>
          </button>
        </div>
      </div>
    </div>

    <div class="gateway-advanced-head">
      <strong>当前模型路由</strong>
      <span>先选主后端，再按顺序添加备用后端</span>
    </div>
    <article class="gateway-route-row">
      <div class="gateway-route-summary">
        <div>
          <strong>{{ activeModelGroup.label }}</strong>
          <small>按模型名命中</small>
        </div>
        <div class="gateway-route-meta">
          <code>{{ activeModelGroup.modelId }}</code>
          <div class="gateway-route-chain" aria-label="当前模型后端顺序">
            <template v-for="(item, index) in modelRouteChain()" :key="`${activeModelGroup.key}-${item.type}-${item.provider}`">
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
            :model-value="activeModelRoute.provider"
            :options="activeProviderOptions()"
            :aria-label="`${activeModelGroup.label} 主后端厂商`"
            @update:modelValue="setModelRouteProvider($event)"
          />
        </label>
        <label>
          <span>凭据类型</span>
          <GeminiSelect
            :model-value="activeModelRoute.credentialType || ''"
            :options="credentialOptions(activeModelRoute.provider)"
            :aria-label="`${activeModelGroup.label} 凭据类型`"
            @update:modelValue="setModelRouteCredential($event)"
          />
        </label>
        <div class="gateway-route-backend-field">
          <div class="gateway-route-backend-head">
            <span>添加备用后端</span>
            <small>{{ fallbackConfigs().length }} 个备用</small>
          </div>
          <GeminiSelect
            model-value=""
            :options="fallbackProviderOptions()"
            placeholder="选择备用后端，添加后可上下调整顺序"
            :aria-label="`${activeModelGroup.label} 添加备用后端`"
            @update:modelValue="addFallbackProvider($event)"
          />
          <div v-if="fallbackConfigs().length" class="gateway-fallback-list">
            <div v-for="(fallback, index) in fallbackConfigs()" :key="fallback.provider" class="gateway-fallback-row">
              <div class="gateway-fallback-title">
                <strong>{{ providerLabel(fallback.provider) }}</strong>
                <small>备用 {{ index + 1 }}</small>
              </div>
              <GeminiSelect
                :model-value="fallback.credentialType || ''"
                :options="credentialOptions(fallback.provider)"
                :aria-label="`${activeModelGroup.label} ${providerLabel(fallback.provider)} 备用凭据类型`"
                @update:modelValue="setFallbackCredential(fallback.provider, $event)"
              />
              <label class="gateway-fallback-model">
                <span>备用模型</span>
                <input
                  :value="fallback.model || ''"
                  type="text"
                  :placeholder="fallbackModelPlaceholder()"
                  @input="setFallbackModel(fallback.provider, $event.target.value)"
                />
              </label>
              <div class="gateway-fallback-actions" aria-label="调整备用路由顺序">
                <button type="button" :disabled="index === 0" @click="moveFallbackProvider(fallback.provider, -1)">上移</button>
                <button type="button" :disabled="index === fallbackConfigs().length - 1" @click="moveFallbackProvider(fallback.provider, 1)">下移</button>
                <button type="button" class="danger" @click="removeFallbackProvider(fallback.provider)">移除</button>
              </div>
            </div>
          </div>
        </div>
        <div class="gateway-route-model-field">
          <span>上游模型 ID</span>
          <input
            :value="activeModelRoute.model"
            type="text"
            :placeholder="activeModelGroup.modelId"
            @input="setModelRouteModel($event.target.value)"
          />
        </div>
      </div>
    </article>

    <div class="gateway-advanced-head">
      <strong>客户端入口默认模型</strong>
      <span>仅用于客户端未显式发送模型时的兼容默认值</span>
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
              :class="{ active: isClientDefaultModelSelected(route, model) }"
              @click="setClientDefaultModel(route, model)"
            >
              {{ model }}
            </button>
          </div>
          <input
            :value="routeConfig(route).model"
            type="text"
            :placeholder="`自定义模型 ID，例如 ${route.fallback.model}`"
            @input="setClientDefaultModel(route, $event.target.value)"
          />
        </div>
      </article>
    </div>
  </section>
</template>

<style src="./GatewayRoutingSettings.css"></style>
