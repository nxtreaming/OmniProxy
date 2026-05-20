<script setup>
const props = defineProps({
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

function credentialTypeLocked(form) {
  return !['openai', 'xiaomi', 'zhipu', 'anthropic'].includes(form.provider)
}

function isJSONCredential(form) {
  return ['codex_auth_json', 'claude_oauth_json'].includes(form.credentialType)
}

function credentialValueLabel(form) {
  if (form.credentialType === 'codex_auth_json') return 'auth.json 内容'
  if (form.credentialType === 'claude_oauth_json') return 'Claude OAuth JSON'
  if (form.credentialType === 'mimo_token_plan') return 'Token Plan Key'
  if (form.credentialType === 'coding_plan') return 'Coding Plan Key'
  return 'API Key'
}

function credentialHint() {
  if (props.form.provider === 'sub2api') {
    return 'Base URL 会跟随这个账号保存，支持同一上游的 OpenAI、Anthropic、Gemini 协议入口。'
  }
  if (props.form.provider === 'zo') {
    return '保存 Zo Access Token 后，可通过 /zo/v1/chat/completions 或 /zo/v1/messages 使用本地兼容入口。'
  }
  if (props.form.provider === 'openrouter') {
    return '保存后可在账号管理刷新 OpenRouter 模型列表，模型 ID 会按 provider/model 展示。'
  }
  if (props.form.credentialType === 'claude_oauth_json') {
    return '支持包含 access_token / refresh_token / expired 的 Claude Code OAuth JSON。'
  }
  if (props.form.provider === 'zhipu' && props.form.credentialType === 'coding_plan') {
    return 'Coding Plan 会使用 api.z.ai 的订阅额度接口；普通 API Key 会查询 BigModel API 余额。'
  }
  return ''
}

function autoNameText(form) {
  if (form.provider === 'anthropic' && form.credentialType === 'claude_oauth_json') {
    return 'Claude OAuth JSON 会优先使用 email 作为账号名称。'
  }
  return 'Codex 将自动使用 auth.json 中的邮箱作为账号名称。'
}
</script>

<template>
  <div class="modal-backdrop" @click.self="$emit('close')">
    <form class="modal" @submit.prevent="$emit('submit')">
      <div class="section-heading">
        <div>
          <h2>{{ form.editingId ? '编辑账号' : '添加账号' }}</h2>
          <p>{{ isCodexForm ? autoNameText(form) : '账号名称必填且不可重复' }}</p>
        </div>
        <button type="button" class="icon-button" @click="$emit('close')">×</button>
      </div>
      <label v-if="!isCodexForm">
        <span>账号名称</span>
        <input v-model="form.name" autofocus />
      </label>
      <div v-else class="form-hint">
        {{ autoNameText(form) }}
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
        <select v-model="form.credentialType" :disabled="credentialTypeLocked(form)">
          <option value="api_key">{{ form.provider === 'xiaomi' ? 'MiMo 按量 API Key (sk-)' : 'API Key' }}</option>
          <option v-if="form.provider === 'openai'" value="codex_auth_json">Codex auth.json</option>
          <option v-if="form.provider === 'anthropic'" value="claude_oauth_json">Claude OAuth JSON</option>
          <option v-if="form.provider === 'xiaomi'" value="mimo_token_plan">MiMo Token Plan (tp-)</option>
          <option v-if="form.provider === 'zhipu'" value="coding_plan">GLM Coding Plan</option>
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
      <label v-if="form.provider === 'sub2api'">
        <span>Base URL</span>
        <input
          v-model="form.baseUrl"
          type="url"
          placeholder="https://aiapi.aicnio.com"
          autocomplete="off"
          spellcheck="false"
        />
        <small>保存到当前账号；/sub2api、/sub2api/anthropic、/sub2api/gemini 会转发到这个上游。</small>
      </label>
      <label>
        <span>{{ credentialValueLabel(form) }}</span>
        <textarea
          v-model="form.tokenValue"
          :rows="isJSONCredential(form) ? 9 : 4"
          :placeholder="placeholder"
        ></textarea>
        <small v-if="credentialHint()">{{ credentialHint() }}</small>
        <small v-if="form.editingId">不填写则继续使用当前已保存凭据</small>
      </label>
      <div class="modal-actions">
        <button type="button" class="ghost-button" @click="$emit('close')">取消</button>
        <button type="submit" class="primary-button">保存</button>
      </div>
    </form>
  </div>
</template>
