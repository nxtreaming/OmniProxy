<script setup>
defineProps({
  form: {
    type: Object,
    required: true,
  },
  providers: {
    type: Array,
    required: true,
  },
  isCodexForm: {
    type: Boolean,
    required: true,
  },
  placeholder: {
    type: String,
    required: true,
  },
})

defineEmits(['close', 'submit', 'provider-change'])
</script>

<template>
  <div class="modal-backdrop" @click.self="$emit('close')">
    <form class="modal" @submit.prevent="$emit('submit')">
      <div class="section-heading">
        <div>
          <h2>{{ form.editingId ? '编辑账号' : '添加账号' }}</h2>
          <p>{{ isCodexForm ? 'Codex 将自动使用 auth.json 中的邮箱作为账号名称' : '账号名称必填且不可重复' }}</p>
        </div>
        <button type="button" class="icon-button" @click="$emit('close')">×</button>
      </div>
      <label v-if="!isCodexForm">
        <span>账号名称</span>
        <input v-model="form.name" autofocus />
      </label>
      <div v-else class="form-hint">
        账号名称会从 `tokens.id_token` 自动解析邮箱，无需手动填写。
      </div>
      <label>
        <span>厂商</span>
        <select v-model="form.provider" @change="$emit('provider-change')">
          <option v-for="provider in providers" :key="provider.key" :value="provider.key">
            {{ provider.label }}
          </option>
        </select>
      </label>
      <label>
        <span>凭据类型</span>
        <select v-model="form.credentialType" :disabled="form.provider !== 'openai' && form.provider !== 'xiaomi'">
          <option value="api_key">{{ form.provider === 'xiaomi' ? 'MiMo 按量 API Key (sk-)' : 'API Key' }}</option>
          <option v-if="form.provider === 'openai'" value="codex_auth_json">Codex auth.json</option>
          <option v-if="form.provider === 'xiaomi'" value="mimo_token_plan">MiMo Token Plan (tp-)</option>
        </select>
      </label>
      <label v-if="form.provider === 'xiaomi' && form.credentialType === 'mimo_token_plan'">
        <span>Token Plan 区域</span>
        <select v-model="form.region">
          <option value="cn">中国区</option>
          <option value="sgp">海外 SGP</option>
        </select>
        <small>海外账号会使用 token-plan-sgp.xiaomimimo.com。</small>
      </label>
      <label>
        <span>{{ form.credentialType === 'codex_auth_json' ? 'auth.json 内容' : form.credentialType === 'mimo_token_plan' ? 'Token Plan Key' : 'API Key' }}</span>
        <textarea
          v-model="form.tokenValue"
          :rows="form.credentialType === 'codex_auth_json' ? 9 : 4"
          :placeholder="placeholder"
        ></textarea>
        <small v-if="form.editingId">不填写则继续使用当前已保存凭据</small>
      </label>
      <div class="modal-actions">
        <button type="button" class="ghost-button" @click="$emit('close')">取消</button>
        <button type="submit" class="primary-button">保存</button>
      </div>
    </form>
  </div>
</template>
