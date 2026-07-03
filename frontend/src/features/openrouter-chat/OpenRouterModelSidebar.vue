<script setup>
import { isFreeModel } from '../../utils/openrouterChat'

defineProps({
  filteredModels: { type: Array, default: () => [] },
  formatNumber: { type: Function, required: true },
  freeModelCount: { type: Number, default: 0 },
  freeOnly: { type: Boolean, default: false },
  maxTokens: { type: Number, default: 1024 },
  modelSearch: { type: String, default: '' },
  models: { type: Array, default: () => [] },
  modelsError: { type: String, default: '' },
  modelsLoading: { type: Boolean, default: false },
  selectedModelId: { type: String, default: '' },
  showModelLoadingSkeleton: { type: Boolean, default: false },
  temperature: { type: Number, default: 0.7 },
})

defineEmits([
  'open-create-key',
  'select-model',
  'toggle-free-only',
  'update:max-tokens',
  'update:model-search',
  'update:temperature',
])
</script>

<template>
  <aside class="openrouter-chat-side">
    <div class="openrouter-chat-search">
      <input
        :value="modelSearch"
        type="search"
        placeholder="搜索模型"
        @input="$emit('update:model-search', $event.target.value)"
      />
      <button
        type="button"
        :class="['openrouter-chat-filter-button', { active: freeOnly }]"
        @click="$emit('toggle-free-only')"
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
              ? models.length
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
            @click="$emit('select-model', model)"
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
        <input
          :value="temperature"
          class="openrouter-chat-number"
          type="number"
          min="0"
          max="2"
          step="0.1"
          @input="$emit('update:temperature', Number($event.target.value))"
        />
      </label>
      <label class="openrouter-chat-field">
        <span>输出上限</span>
        <input
          :value="maxTokens"
          class="openrouter-chat-number"
          type="number"
          min="1"
          max="200000"
          step="256"
          @input="$emit('update:max-tokens', Number($event.target.value))"
        />
      </label>
    </div>
  </aside>
</template>
