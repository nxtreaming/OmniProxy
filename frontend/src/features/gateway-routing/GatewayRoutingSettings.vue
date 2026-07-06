<script setup>
import { computed, ref, watch } from 'vue'
import GeminiSelect from '../../components/GeminiSelect.vue'
import { credentialTypes, providers } from '../../constants/app'
import GatewayModelCatalogSync from './GatewayModelCatalogSync.vue'
import GatewayRouteDiagnostics from './GatewayRouteDiagnostics.vue'
import GatewayRouteStrategyPicker from './GatewayRouteStrategyPicker.vue'
import { gatewayPlatformPresets, inferGatewayProviderForModel, routeDefinitions, routeStrategyChain } from './gatewayRoutePresets'

const props = defineProps({
  config: {
    type: Object,
    required: true,
  },
})
const emit = defineEmits(['route-draft-dirty'])

const providerLabelMap = computed(() => Object.fromEntries(providers.map((item) => [item.key, item.label])))
const credentialLabelMap = computed(() => credentialTypes)
const gatewayModelGroups = computed(() => buildGatewayModelGroups())
const selectedModelKey = ref('')
const modelSearchInput = ref('')
const syncedModelGroups = ref([])
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

function emptyModelGroup() { return { key: '', modelId: '', label: '选择模型', providers: [] } }

function currentModelRoutesConfig() {
  return props.config.modelRoutes && typeof props.config.modelRoutes === 'object' ? props.config.modelRoutes : {}
}

function ensureModelRoutesConfig() {
  if (!props.config.modelRoutes || typeof props.config.modelRoutes !== 'object') {
    props.config.modelRoutes = {}
  }
  return props.config.modelRoutes
}

function routeConfig(route) {
  const config = props.config.gatewayRoutes?.[route.key]
  return config && typeof config === 'object' ? config : { ...route.fallback, fallbacks: [] }
}

function ensureRouteConfig(route) {
  if (!props.config.gatewayRoutes || typeof props.config.gatewayRoutes !== 'object') {
    props.config.gatewayRoutes = {}
  }
  if (!props.config.gatewayRoutes[route.key] || typeof props.config.gatewayRoutes[route.key] !== 'object') {
    props.config.gatewayRoutes[route.key] = { ...route.fallback, fallbacks: [] }
  }
  return props.config.gatewayRoutes[route.key]
}

function modelRouteConfig(group) {
  const route = readModelRoute(group)
  if (!route || typeof route !== 'object') return defaultModelRoute(group)
  return {
    ...route,
    model: route.model || group.modelId,
    fallbacks: Array.isArray(route.fallbacks) ? route.fallbacks : [],
  }
}

function defaultModelRoute(group) {
  const firstProvider = group.providers[0]?.key || ''
  return {
    provider: firstProvider,
    credentialType: defaultCredentialType(firstProvider),
    model: group.modelId || '',
    fallbacks: [],
  }
}

function ensureModelRouteConfig(group) {
  const routes = ensureModelRoutesConfig()
  const key = group.key
  if (!key) return { provider: '', credentialType: '', model: '', fallbacks: [] }
  if (!routes[key] || typeof routes[key] !== 'object') {
    routes[key] = defaultModelRoute(group)
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
  return currentModelRoutesConfig()[group.key]
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
  for (const [rawModel, route] of Object.entries(currentModelRoutesConfig())) {
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
  for (const synced of syncedModelGroups.value) {
    const key = modelKey(synced.modelId)
    if (!key) continue
    if (!byKey.has(key)) {
      byKey.set(key, {
        key,
        modelId: synced.modelId,
        label: synced.label || synced.modelId,
        providers: [],
      })
    }
    const group = byKey.get(key)
    for (const option of synced.providers || []) {
      if (!group.providers.some((item) => item.key === option.key)) {
        group.providers.push(option)
      }
    }
  }
  return Array.from(byKey.values())
}

function modelRouteModelIDs(model) {
  return Array.from(new Set(Object.values(model.routeModels || {}).map((value) => String(value || '').trim()).filter(Boolean)))
}

function modelKey(model) { return String(model || '').trim().toLowerCase() }

function selectModelGroup(group) {
  selectedModelKey.value = group.key
}

function markDirty() {
  emit('route-draft-dirty')
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
  const routes = ensureModelRoutesConfig()
  if (!routes[key]) {
    const provider = inferGatewayProviderForModel(modelId)
    routes[key] = {
      provider,
      credentialType: defaultCredentialType(provider),
      model: modelId,
      fallbacks: [],
    }
    markDirty()
  }
  modelSearchInput.value = ''
  selectedModelKey.value = key
}

function mergeSyncedModelGroups(groups) {
  const byKey = new Map(syncedModelGroups.value.map((group) => [group.key, { ...group, providers: [...group.providers] }]))
  for (const group of groups) {
    if (!byKey.has(group.key)) {
      byKey.set(group.key, group)
      continue
    }
    const existing = byKey.get(group.key)
    for (const option of group.providers) {
      if (!existing.providers.some((item) => item.key === option.key)) {
        existing.providers.push(option)
      }
    }
  }
  syncedModelGroups.value = Array.from(byKey.values())
}

function mergeProviderModelResult(result) {
  const provider = normalizeProviderKey(result?.provider)
  const groups = (result?.models || []).map((model) => {
    const modelId = String(model.id || '').trim()
    return {
      key: modelKey(modelId),
      modelId,
      label: model.name || modelId,
      providers: [{ key: provider, modelId }],
    }
  }).filter((group) => group.key && provider)
  mergeSyncedModelGroups(groups)
  if (groups[0]) {
    selectedModelKey.value = groups[0].key
  }
}

function providerLabel(provider) {
  return providerLabelMap.value[provider] || provider
}

function providerNote(provider) {
  return providers.find((item) => item.key === provider)?.note || ''
}

function normalizeProviderKey(provider) { return String(provider || '').trim().toLowerCase() }

function defaultCredentialType() { return '' }

function modelRouteProviderKeys() {
  const keys = []
  for (const platform of gatewayPlatformPresets) {
    if (platform.key) keys.push(platform.key)
  }
  return Array.from(new Set(keys.map(normalizeProviderKey).filter(Boolean)))
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

function supportedCredentialValues(provider) {
  return credentialOptions(provider).map((option) => option.value)
}

function credentialForProvider(provider, currentCredentialType = '') {
  const credentialType = String(currentCredentialType || '').trim()
  if (!credentialType) return ''
  return supportedCredentialValues(provider).includes(credentialType) ? credentialType : ''
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
  const route = ensureModelRouteConfig(activeModelGroup.value)
  const currentProvider = route.provider
  route.provider = option.key
  route.credentialType = normalizeProviderKey(currentProvider) === normalizeProviderKey(option.key)
    ? credentialForProvider(option.key, route.credentialType)
    : ''
  route.model = option.modelId
  route.fallbacks = (route.fallbacks || []).filter(
    (fallback) => normalizeProviderKey(fallback.provider) !== normalizeProviderKey(option.key),
  )
  markDirty()
}

function applyRouteStrategy(strategyKey) {
  const providerKeys = activeModelGroup.value.providers.map((option) => option.key)
  const chain = routeStrategyChain(providerKeys, strategyKey)
  if (!chain.length) return
  const route = ensureModelRouteConfig(activeModelGroup.value)
  route.provider = chain[0]
  route.credentialType = credentialForProvider(route.provider, route.credentialType)
  route.model = modelForProvider(route.provider)
  route.fallbacks = chain.slice(1).map((provider) => ({
    provider,
    credentialType: defaultCredentialType(provider),
    model: modelForProvider(provider),
  }))
  markDirty()
}

function modelForProvider(provider) {
  const key = normalizeProviderKey(provider)
  return activeModelGroup.value.providers.find((option) => normalizeProviderKey(option.key) === key)?.modelId ||
    activeModelRoute.value.model ||
    activeModelGroup.value.modelId
}

function isModelProviderApplied(option) {
  const route = activeModelRoute.value
  return normalizeProviderKey(route?.provider) === normalizeProviderKey(option.key) &&
    String(route?.model || '').trim().toLowerCase() === String(option.modelId || '').trim().toLowerCase()
}

function setModelRouteProvider(provider) {
  const route = ensureModelRouteConfig(activeModelGroup.value)
  const currentProvider = route.provider
  route.provider = provider
  route.credentialType = normalizeProviderKey(currentProvider) === normalizeProviderKey(provider)
    ? credentialForProvider(provider, route.credentialType)
    : ''
  route.fallbacks = (route.fallbacks || []).filter((fallback) => normalizeProviderKey(fallback.provider) !== normalizeProviderKey(provider))
  markDirty()
}

function setModelRouteCredential(credentialType) {
  const route = ensureModelRouteConfig(activeModelGroup.value)
  route.credentialType = credentialForProvider(route.provider, credentialType)
  markDirty()
}

function setModelRouteModel(model) {
  ensureModelRouteConfig(activeModelGroup.value).model = model
  markDirty()
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
  const route = ensureModelRouteConfig(activeModelGroup.value)
  route.fallbacks = [
    ...(route.fallbacks || []),
    { provider, credentialType: defaultCredentialType(provider), model: route.model || activeModelGroup.value.modelId },
  ]
  markDirty()
}

function setFallbackCredential(provider, credentialType) {
  const route = ensureModelRouteConfig(activeModelGroup.value)
  const fallback = (route.fallbacks || []).find((item) => normalizeProviderKey(item.provider) === normalizeProviderKey(provider))
  if (!fallback) return
  fallback.credentialType = credentialForProvider(fallback.provider, credentialType)
  markDirty()
}

function setFallbackModel(provider, model) {
  const route = ensureModelRouteConfig(activeModelGroup.value)
  const fallback = (route.fallbacks || []).find((item) => normalizeProviderKey(item.provider) === normalizeProviderKey(provider))
  if (!fallback) return
  fallback.model = model
  markDirty()
}

function fallbackModelPlaceholder() {
  return `继承模型：${activeModelRoute.value.model || activeModelGroup.value.modelId}`
}

function removeFallbackProvider(provider) {
  const key = normalizeProviderKey(provider)
  const route = ensureModelRouteConfig(activeModelGroup.value)
  route.fallbacks = (route.fallbacks || []).filter((fallback) => normalizeProviderKey(fallback.provider) !== key)
  markDirty()
}

function moveFallbackProvider(provider, direction) {
  const route = ensureModelRouteConfig(activeModelGroup.value)
  const index = (route.fallbacks || []).findIndex((fallback) => normalizeProviderKey(fallback.provider) === normalizeProviderKey(provider))
  const nextIndex = index + direction
  if (index < 0 || nextIndex < 0 || nextIndex >= route.fallbacks.length) return
  const next = [...route.fallbacks]
  const [item] = next.splice(index, 1)
  next.splice(nextIndex, 0, item)
  route.fallbacks = next
  markDirty()
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
  ensureRouteConfig(route).model = model
  markDirty()
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
    <GatewayModelCatalogSync :providers="providers" @synced="mergeProviderModelResult" />
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
    <GatewayRouteStrategyPicker @apply="applyRouteStrategy" />
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

    <GatewayRouteDiagnostics
      :model="activeModelRoute.model || activeModelGroup.modelId"
      :provider-label="providerLabel"
      :credential-labels="credentialLabelMap"
    />

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
