<script setup>
import { computed } from 'vue'
import GeminiSelect from './GeminiSelect.vue'

const props = defineProps({
  form: {
    type: Object,
    required: true,
  },
  providers: {
    type: Array,
    required: true,
  },
  placeholder: {
    type: String,
    required: true,
  },
  importing: {
    type: Boolean,
    required: true,
  },
})

defineEmits(['close', 'submit', 'provider-change'])

const providerOptions = computed(() =>
  props.providers.map((provider) => ({ value: provider.key, label: provider.label })),
)
</script>

<template>
  <div class="modal-backdrop token-editor-backdrop batch-import-backdrop" @click.self="$emit('close')">
    <form class="modal token-editor-modal batch-import-modal" @submit.prevent="$emit('submit')">
      <div class="section-heading">
        <div>
          <h2>批量导入 API Key</h2>
          <p>每行一个 Key，名称自动使用 Key 的前 8 位；行内 # 后的备注会被忽略。</p>
        </div>
        <button type="button" class="icon-button" :disabled="importing" @click="$emit('close')">×</button>
      </div>

      <label>
        <span>厂商</span>
        <GeminiSelect
          v-model="form.provider"
          :options="providerOptions"
          :disabled="importing"
          aria-label="选择厂商"
          @change="$emit('provider-change')"
        />
      </label>

      <label v-if="form.provider === 'sub2api'">
        <span>Base URL</span>
        <input
          v-model="form.baseUrl"
          type="url"
          placeholder="https://aiapi.aicnio.com"
          autocomplete="off"
          spellcheck="false"
          :disabled="importing"
        />
        <small>同一批导入的 sub2api Key 会保存到这个上游 Base URL。</small>
      </label>

      <label>
        <span>API Key 列表</span>
        <textarea
          v-model="form.tokenText"
          rows="10"
          :placeholder="placeholder"
          :disabled="importing"
          spellcheck="false"
        ></textarea>
        <small>支持形如 sk-xxx # balance: 10.14 CNY 的行，只会导入 # 前面的第一个字段。</small>
      </label>

      <div class="modal-actions">
        <button type="button" class="ghost-button" :disabled="importing" @click="$emit('close')">取消</button>
        <button type="submit" class="primary-button" :disabled="importing">
          {{ importing ? '导入中' : '导入' }}
        </button>
      </div>
    </form>
  </div>
</template>
