<script setup>
import { ref } from 'vue'
import GeminiSelect from '../../components/GeminiSelect.vue'
import { syncProviderModels } from '../../services/api'

defineProps({
  providers: {
    type: Array,
    required: true,
  },
})

const emit = defineEmits(['synced'])
const modelSyncProvider = ref('openrouter')
const syncingProviderModels = ref(false)
const providerModelSyncError = ref('')

async function syncCurrentProviderModels() {
  if (syncingProviderModels.value) return
  syncingProviderModels.value = true
  providerModelSyncError.value = ''
  try {
    const result = await syncProviderModels(modelSyncProvider.value)
    emit('synced', result)
  } catch (error) {
    providerModelSyncError.value = error.message
  } finally {
    syncingProviderModels.value = false
  }
}
</script>

<template>
  <div class="gateway-catalog-sync">
    <div>
      <span>同步模型目录</span>
      <small v-if="providerModelSyncError">{{ providerModelSyncError }}</small>
      <small v-else>从已添加凭据的 provider 拉取 /models，结果会临时加入当前列表。</small>
    </div>
    <GeminiSelect
      v-model="modelSyncProvider"
      :options="providers.map((provider) => ({ value: provider.key, label: provider.label, description: provider.note }))"
      aria-label="选择同步模型目录的 provider"
    />
    <button type="button" class="gateway-model-preset" :disabled="syncingProviderModels" @click="syncCurrentProviderModels">
      {{ syncingProviderModels ? '同步中' : '同步目录' }}
    </button>
  </div>
</template>
