<script setup>
import { computed, ref } from 'vue'
import { MagicStick, Monitor, RefreshRight } from '@element-plus/icons-vue'

const props = defineProps({
  config: { type: Object, required: true },
  clientConfigPreviews: { type: Array, default: () => [] },
  codexModelOptions: { type: Array, required: true },
  codexModelSelectionLimit: { type: Number, required: true },
  selectedCodexModels: { type: Array, required: true },
  selectedCodexModelLabels: { type: Array, required: true },
  canConfigureCodexModels: { type: Boolean, required: true },
  isCodexModelOptionDisabled: { type: Function, required: true },
  claudeModelOptions: { type: Array, required: true },
  claudeModelSelectionLimit: { type: Number, required: true },
  selectedClaudeModels: { type: Array, required: true },
  selectedClaudeModelLabels: { type: Array, required: true },
  canConfigureClaudeModels: { type: Boolean, required: true },
  isClaudeModelOptionDisabled: { type: Function, required: true },
  codexConfiguring: { type: Boolean, default: false },
  codexRestoring: { type: Boolean, default: false },
  claudeModelsConfiguring: { type: Boolean, default: false },
  claudeDesktopConfiguring: { type: Boolean, default: false },
  claudeDesktopRestoring: { type: Boolean, default: false },
  claudeCliRestoring: { type: Boolean, default: false },
  geminiConfiguring: { type: Boolean, default: false },
  geminiRestoring: { type: Boolean, default: false },
  opencodeConfiguring: { type: Boolean, default: false },
  opencodeRestoring: { type: Boolean, default: false },
  piConfiguring: { type: Boolean, default: false },
  piRestoring: { type: Boolean, default: false },
  deepSeekTuiConfiguring: { type: Boolean, default: false },
  deepSeekTuiRestoring: { type: Boolean, default: false },
})

const emit = defineEmits([
  'update:selectedCodexModels',
  'update:selectedClaudeModels',
  'configure-codex',
  'restore-codex',
  'configure-claude-models',
  'configure-claude-desktop-models',
  'restore-claude-desktop',
  'restore-claude',
  'configure-gemini',
  'restore-gemini',
  'configure-opencode',
  'restore-opencode',
  'configure-pi',
  'restore-pi',
  'configure-deepseek-tui',
  'restore-deepseek-tui',
])

const customCodexModel = ref('')

const selectedModels = computed({
  get: () => props.selectedClaudeModels,
  set: (value) => emit('update:selectedClaudeModels', value),
})

const selectedCodex = computed({
  get: () => props.selectedCodexModels,
  set: (value) => emit('update:selectedCodexModels', value),
})

const codexRouteModel = computed(() => props.selectedCodexModels[0] || props.config.gatewayRoutes?.codex?.model || 'gpt-5.5')
const claudeRouteModel = computed(() => props.config.gatewayRoutes?.claude?.model || 'default')
const openAIRouteModel = computed(() => props.config.gatewayRoutes?.openai?.model || 'gpt-5.4')
const geminiRouteModel = computed(() => props.config.gatewayRoutes?.gemini?.model || 'gemini-3-pro-preview')

function addCustomCodexModel() {
  const model = customCodexModel.value.trim()
  if (!model || selectedCodex.value.includes(model) || selectedCodex.value.length >= props.codexModelSelectionLimit) return
  emit('update:selectedCodexModels', [...selectedCodex.value, model])
  customCodexModel.value = ''
}

function previewTarget(preview) {
  return [preview.configPath, preview.settingsPath].filter(Boolean).join(' / ')
}

function previewMeta(preview) {
  return [
    preview.baseUrl,
    preview.providerId ? `Provider ${preview.providerId}` : '',
    preview.model ? `模型 ${preview.model}` : '',
    preview.models?.length ? `模型 ${preview.models.join('、')}` : '',
  ].filter(Boolean).join(' · ')
}
</script>

<template>
  <section class="help-panel quickstart-panel">
    <section v-if="clientConfigPreviews.length" class="client-preview-panel">
      <div class="client-preview-head">
        <strong>配置预览</strong>
        <small>一键配置会写入以下本地文件，并保留 `.omniproxy.bak` 备份。</small>
      </div>
      <div class="client-preview-list">
        <div v-for="preview in clientConfigPreviews" :key="preview.client" class="client-preview-row">
          <div>
            <strong>{{ preview.client }}</strong>
            <small>{{ preview.message }}</small>
          </div>
          <code>{{ previewTarget(preview) }}</code>
          <small>{{ previewMeta(preview) }}</small>
        </div>
      </div>
    </section>
    <div class="help-grid">
      <article class="wide-help">
        <strong>Codex</strong>
        <p>本地 Codex 写入 OmniProxy 网关地址和默认模型；该模型使用哪个后端、按什么顺序备用，在「网关路由」页面配置。</p>
        <pre class="help-code"><code>Base URL: http://127.0.0.1:{{ config.proxyPort }}/codex/v1
Protocol: OpenAI Responses
默认模型: {{ codexRouteModel }}</code></pre>
        <div class="claude-model-config">
          <div class="claude-model-config-head">
            <span>Codex 模型</span>
            <small>{{ selectedCodex.length }} / {{ codexModelSelectionLimit }}</small>
          </div>
          <div class="claude-model-picker" role="group" aria-label="Codex 可选模型">
            <label
              v-for="option in codexModelOptions"
              :key="option.id"
              :class="[
                'claude-model-choice',
                {
                  selected: selectedCodex.includes(option.id),
                  disabled: isCodexModelOptionDisabled(option.id),
                },
              ]"
            >
              <input
                v-model="selectedCodex"
                type="checkbox"
                :value="option.id"
                :disabled="isCodexModelOptionDisabled(option.id)"
              />
              <span>
                <strong>{{ option.label }}</strong>
                <small>{{ option.description }}</small>
              </span>
            </label>
          </div>
          <small class="claude-model-selection">
            已选：{{ selectedCodexModelLabels.length ? selectedCodexModelLabels.join('、') : '未选择' }}
          </small>
          <label class="gateway-route-model-field">
            <span>自定义模型 ID</span>
            <input
              v-model="customCodexModel"
              type="text"
              placeholder="例如 qwen3.5、custom-model、provider/model"
              @keydown.enter.prevent="addCustomCodexModel"
            />
          </label>
          <button type="button" class="quickstart-model-add" :disabled="selectedCodex.length >= codexModelSelectionLimit" @click="addCustomCodexModel">
            添加 Codex 模型
          </button>
        </div>
        <div class="help-actions">
          <el-button type="primary" :icon="MagicStick" :loading="codexConfiguring" :disabled="!canConfigureCodexModels" @click="$emit('configure-codex')">
            {{ codexConfiguring ? '配置中' : '配置 Codex 网关' }}
          </el-button>
          <el-button :icon="RefreshRight" :loading="codexRestoring" @click="$emit('restore-codex')">
            {{ codexRestoring ? '恢复中' : '恢复 Codex 配置' }}
          </el-button>
        </div>
      </article>

      <article class="wide-help">
        <strong>Claude Code</strong>
        <p>Claude Code 和 Claude Desktop 固定接入本地 Anthropic 网关；模型槽位只控制客户端发送的模型名，后端厂商在网关路由中选择。</p>
        <pre class="help-code"><code>Claude Router URL: http://127.0.0.1:{{ config.proxyPort }}/anthropic-router
默认模型: {{ claudeRouteModel }}</code></pre>
        <div class="claude-model-config">
          <div class="claude-model-config-head">
            <span>可选模型</span>
            <small>{{ selectedModels.length }} / {{ claudeModelSelectionLimit }}</small>
          </div>
          <div class="claude-model-picker" role="group" aria-label="Claude Code 可选模型">
            <label
              v-for="option in claudeModelOptions"
              :key="option.id"
              :class="[
                'claude-model-choice',
                {
                  selected: selectedModels.includes(option.id),
                  disabled: isClaudeModelOptionDisabled(option.id),
                },
              ]"
            >
              <input
                v-model="selectedModels"
                type="checkbox"
                :value="option.id"
                :disabled="isClaudeModelOptionDisabled(option.id)"
              />
              <span>
                <strong>{{ option.label }}</strong>
                <small>{{ option.description }}</small>
              </span>
            </label>
          </div>
          <small class="claude-model-selection">
            已选：{{ selectedClaudeModelLabels.length ? selectedClaudeModelLabels.join('、') : '未选择' }}
          </small>
          <small class="claude-model-selection">
            CLI 写入 <code>%USERPROFILE%\.claude\settings.json</code>；Desktop 写入 Claude 3P Gateway Profile，配置后请完全退出并重启 Claude Desktop。
          </small>
        </div>
        <div class="claude-action-panel">
          <div class="claude-action-row">
            <span>按当前选择写入</span>
            <div class="help-actions claude-actions">
              <el-button
                type="success"
                :icon="MagicStick"
                :loading="claudeModelsConfiguring"
                :disabled="!canConfigureClaudeModels"
                @click="$emit('configure-claude-models')"
              >
                {{ claudeModelsConfiguring ? '配置中' : 'Claude CLI' }}
              </el-button>
              <el-button
                type="success"
                plain
                :icon="Monitor"
                :loading="claudeDesktopConfiguring"
                :disabled="!canConfigureClaudeModels"
                @click="$emit('configure-claude-desktop-models')"
              >
                {{ claudeDesktopConfiguring ? '配置中' : 'Claude Desktop' }}
              </el-button>
              <el-button :icon="RefreshRight" :loading="claudeDesktopRestoring" @click="$emit('restore-claude-desktop')">
                {{ claudeDesktopRestoring ? '恢复中' : '恢复 Desktop' }}
              </el-button>
            </div>
          </div>
          <div class="claude-action-row">
            <span>恢复 CLI</span>
            <div class="help-actions claude-actions">
              <el-button :icon="RefreshRight" :loading="claudeCliRestoring" @click="$emit('restore-claude')">
                {{ claudeCliRestoring ? '恢复中' : '恢复 CLI' }}
              </el-button>
            </div>
          </div>
        </div>
      </article>

      <article class="wide-help">
        <strong>Gemini CLI</strong>
        <p>写入 <code>%USERPROFILE%\.gemini\.env</code> 和 <code>settings.json</code>，固定连接本地 Gemini 网关。</p>
        <pre class="help-code"><code>GOOGLE_GEMINI_BASE_URL=http://127.0.0.1:{{ config.proxyPort }}/gemini
GEMINI_MODEL={{ geminiRouteModel }}</code></pre>
        <div class="help-actions">
          <el-button type="primary" :icon="MagicStick" :loading="geminiConfiguring" @click="$emit('configure-gemini')">
            {{ geminiConfiguring ? '配置中' : '配置 Gemini CLI' }}
          </el-button>
          <el-button :icon="RefreshRight" :loading="geminiRestoring" @click="$emit('restore-gemini')">
            {{ geminiRestoring ? '恢复中' : '恢复 Gemini 配置' }}
          </el-button>
        </div>
      </article>

      <article class="wide-help">
        <strong>OpenCode</strong>
        <p>写入 <code>%USERPROFILE%\.config\opencode\opencode.json</code>，只添加 OmniProxy provider；后端厂商在网关路由中选择。</p>
        <pre class="help-code"><code>OpenAI-compatible Router: http://127.0.0.1:{{ config.proxyPort }}/opencode-router/v1
Provider ID: omniproxy
默认模型: {{ openAIRouteModel }}</code></pre>
        <div class="help-actions">
          <el-button type="primary" :icon="MagicStick" :loading="opencodeConfiguring" @click="$emit('configure-opencode')">
            {{ opencodeConfiguring ? '配置中' : '配置 OpenCode' }}
          </el-button>
          <el-button :icon="RefreshRight" :loading="opencodeRestoring" @click="$emit('restore-opencode')">
            {{ opencodeRestoring ? '恢复中' : '恢复 OpenCode 配置' }}
          </el-button>
        </div>
      </article>

      <article class="wide-help">
        <strong>Pi Coding Agent</strong>
        <p>写入 <code>%USERPROFILE%\.pi\agent\models.json</code>，只添加 OmniProxy provider，可通过 <code>pi --provider omniproxy --model {{ openAIRouteModel }}</code> 使用。</p>
        <pre class="help-code"><code>Pi Router: http://127.0.0.1:{{ config.proxyPort }}/pi-router/v1
Provider ID: omniproxy
默认模型: {{ openAIRouteModel }}</code></pre>
        <div class="help-actions">
          <el-button type="primary" :icon="MagicStick" :loading="piConfiguring" @click="$emit('configure-pi')">
            {{ piConfiguring ? '配置中' : '配置 Pi Coding Agent' }}
          </el-button>
          <el-button :icon="RefreshRight" :loading="piRestoring" @click="$emit('restore-pi')">
            {{ piRestoring ? '恢复中' : '恢复 Pi 配置' }}
          </el-button>
        </div>
      </article>

      <article class="wide-help">
        <strong>DeepSeek-TUI</strong>
        <p>写入 <code>%USERPROFILE%\.deepseek\config.toml</code>，使用 OmniProxy provider 连接 OpenAI 兼容网关。</p>
        <pre class="help-code"><code>provider = "omniproxy"
default_text_model = "{{ openAIRouteModel }}"
[providers.omniproxy]
base_url = "http://127.0.0.1:{{ config.proxyPort }}/opencode-router/v1"
api_key = "omniproxy-local"</code></pre>
        <div class="help-actions">
          <el-button type="primary" :icon="MagicStick" :loading="deepSeekTuiConfiguring" @click="$emit('configure-deepseek-tui')">
            {{ deepSeekTuiConfiguring ? '配置中' : '配置 DeepSeek-TUI' }}
          </el-button>
          <el-button :icon="RefreshRight" :loading="deepSeekTuiRestoring" @click="$emit('restore-deepseek-tui')">
            {{ deepSeekTuiRestoring ? '恢复中' : '恢复 DeepSeek-TUI 配置' }}
          </el-button>
        </div>
      </article>
    </div>
  </section>
</template>

<style src="./QuickstartView.css"></style>
