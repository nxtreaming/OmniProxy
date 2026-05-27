<script setup>
import { computed } from 'vue'
import { MagicStick, Monitor, RefreshRight } from '@element-plus/icons-vue'

const props = defineProps({
  config: { type: Object, required: true },
  claudeModelOptions: { type: Array, required: true },
  claudeModelSelectionLimit: { type: Number, required: true },
  selectedClaudeModels: { type: Array, required: true },
  selectedClaudeModelLabels: { type: Array, required: true },
  canConfigureClaudeModels: { type: Boolean, required: true },
  isClaudeModelOptionDisabled: { type: Function, required: true },
  codexConfiguring: { type: Boolean, default: false },
  codexSub2apiConfiguring: { type: Boolean, default: false },
  codexNewapiConfiguring: { type: Boolean, default: false },
  codexZoConfiguring: { type: Boolean, default: false },
  codexRestoring: { type: Boolean, default: false },
  claudeModelsConfiguring: { type: Boolean, default: false },
  claudeDesktopConfiguring: { type: Boolean, default: false },
  claudeDesktopRestoring: { type: Boolean, default: false },
  deepSeekClaudeConfiguring: { type: Boolean, default: false },
  mimoClaudeConfiguring: { type: Boolean, default: false },
  kimiClaudeConfiguring: { type: Boolean, default: false },
  zhipuClaudeConfiguring: { type: Boolean, default: false },
  zoClaudeConfiguring: { type: Boolean, default: false },
  mimoClaudeRestoring: { type: Boolean, default: false },
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
  'update:selectedClaudeModels',
  'configure-codex',
  'configure-codex-sub2api',
  'configure-codex-newapi',
  'configure-codex-zo',
  'restore-codex',
  'configure-claude-models',
  'configure-claude-desktop-models',
  'restore-claude-desktop',
  'configure-deepseek-claude',
  'configure-mimo-claude',
  'configure-kimi-claude',
  'configure-zhipu-claude',
  'configure-zo-claude',
  'restore-mimo-claude',
  'configure-gemini',
  'restore-gemini',
  'configure-opencode',
  'restore-opencode',
  'configure-pi',
  'restore-pi',
  'configure-deepseek-tui',
  'restore-deepseek-tui',
])

const selectedModels = computed({
  get: () => props.selectedClaudeModels,
  set: (value) => emit('update:selectedClaudeModels', value),
})
</script>

<template>
  <section class="help-panel quickstart-panel">
    <div class="help-grid">
      <article class="wide-help">
        <strong>Codex</strong>
        <p>本地 Codex 会写入 <code>%USERPROFILE%\.codex\config.toml</code>。OpenAI Codex 使用 auth.json；sub2api 和 new-api 使用账号池里的 API Key。</p>
        <pre class="help-code"><code>OpenAI Codex Base URL: http://127.0.0.1:{{ config.proxyPort }}/backend-api/codex
sub2api OpenAI/Codex: http://127.0.0.1:{{ config.proxyPort }}/sub2api
sub2api Anthropic: http://127.0.0.1:{{ config.proxyPort }}/sub2api/anthropic
sub2api Gemini: http://127.0.0.1:{{ config.proxyPort }}/sub2api/gemini
new-api OpenAI/Codex: http://127.0.0.1:{{ config.proxyPort }}/newapi
new-api Anthropic: http://127.0.0.1:{{ config.proxyPort }}/newapi/anthropic
new-api Gemini: http://127.0.0.1:{{ config.proxyPort }}/newapi/gemini
Zo Computer: http://127.0.0.1:{{ config.proxyPort }}/zo</code></pre>
        <div class="help-actions">
          <el-button type="primary" :icon="MagicStick" :loading="codexConfiguring" @click="$emit('configure-codex')">
            {{ codexConfiguring ? '配置中' : '配置 Codex OpenAI' }}
          </el-button>
          <el-button type="primary" plain :icon="MagicStick" :loading="codexSub2apiConfiguring" @click="$emit('configure-codex-sub2api')">
            {{ codexSub2apiConfiguring ? '配置中' : '配置 Codex sub2api' }}
          </el-button>
          <el-button type="primary" plain :icon="MagicStick" :loading="codexNewapiConfiguring" @click="$emit('configure-codex-newapi')">
            {{ codexNewapiConfiguring ? '配置中' : '配置 Codex new-api' }}
          </el-button>
          <el-button type="primary" plain :icon="MagicStick" :loading="codexZoConfiguring" @click="$emit('configure-codex-zo')">
            {{ codexZoConfiguring ? '配置中' : '配置 Codex Zo' }}
          </el-button>
          <el-button :icon="RefreshRight" :loading="codexRestoring" @click="$emit('restore-codex')">
            {{ codexRestoring ? '恢复中' : '恢复 Codex 配置' }}
          </el-button>
        </div>
      </article>

      <article class="wide-help">
        <strong>Claude Code</strong>
        <p>每次只接入一个 Claude Code 上游，也可以按需选择最多 4 个模型写入模型槽位，并清理 OmniProxy 旧配置。</p>
        <pre class="help-code"><code>Claude Router URL: http://127.0.0.1:{{ config.proxyPort }}/anthropic-router
DeepSeek: deepseek-v4-pro[1m] / deepseek-v4-flash
MiMo: MiMo-V2.5-Pro / MiMo-V2.5
Kimi model: kimi-for-coding
GLM model: glm-5.1
Zo models: claude-opus-4-7 / claude-sonnet-4-6</code></pre>
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
            <span>快捷单模型</span>
            <div class="help-actions claude-actions">
              <el-button type="primary" :icon="MagicStick" :loading="deepSeekClaudeConfiguring" @click="$emit('configure-deepseek-claude')">
                {{ deepSeekClaudeConfiguring ? '配置中' : 'DeepSeek' }}
              </el-button>
              <el-button type="primary" plain :icon="MagicStick" :loading="mimoClaudeConfiguring" @click="$emit('configure-mimo-claude')">
                {{ mimoClaudeConfiguring ? '配置中' : 'MiMo' }}
              </el-button>
              <el-button type="primary" plain :icon="MagicStick" :loading="kimiClaudeConfiguring" @click="$emit('configure-kimi-claude')">
                {{ kimiClaudeConfiguring ? '配置中' : 'Kimi' }}
              </el-button>
              <el-button type="primary" plain :icon="MagicStick" :loading="zhipuClaudeConfiguring" @click="$emit('configure-zhipu-claude')">
                {{ zhipuClaudeConfiguring ? '配置中' : 'GLM' }}
              </el-button>
              <el-button type="primary" plain :icon="MagicStick" :loading="zoClaudeConfiguring" @click="$emit('configure-zo-claude')">
                {{ zoClaudeConfiguring ? '配置中' : 'Zo' }}
              </el-button>
              <el-button :icon="RefreshRight" :loading="mimoClaudeRestoring" @click="$emit('restore-mimo-claude')">
                {{ mimoClaudeRestoring ? '恢复中' : '恢复 CLI' }}
              </el-button>
            </div>
          </div>
        </div>
      </article>

      <article class="wide-help">
        <strong>Gemini CLI</strong>
        <p>写入 <code>%USERPROFILE%\.gemini\.env</code> 和 <code>settings.json</code>，使用账号池里的 Gemini API Key。</p>
        <pre class="help-code"><code>GOOGLE_GEMINI_BASE_URL=http://127.0.0.1:{{ config.proxyPort }}/gemini
GEMINI_MODEL=gemini-3-pro-preview</code></pre>
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
        <p>写入 <code>%USERPROFILE%\.config\opencode\opencode.json</code>，添加 OmniProxy、Gemini、OpenRouter、TokenRouter、Zo Computer 和自定义网关 provider。</p>
        <pre class="help-code"><code>OpenAI-compatible Router: http://127.0.0.1:{{ config.proxyPort }}/opencode-router/v1
Gemini Native: http://127.0.0.1:{{ config.proxyPort }}/gemini
OpenRouter: http://127.0.0.1:{{ config.proxyPort }}/openrouter/v1
TokenRouter: http://127.0.0.1:{{ config.proxyPort }}/tokenrouter/v1
Zo Computer: http://127.0.0.1:{{ config.proxyPort }}/zo/v1
Custom Gateway: http://127.0.0.1:{{ config.proxyPort }}/custom/v1</code></pre>
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
        <p>写入 <code>%USERPROFILE%\.pi\agent\models.json</code>，添加 OmniProxy 和 Zo Computer provider，可通过 <code>pi --provider omniproxy --model gpt-5.4</code> 使用。</p>
        <pre class="help-code"><code>Pi Router: http://127.0.0.1:{{ config.proxyPort }}/pi-router/v1
Anthropic Router: http://127.0.0.1:{{ config.proxyPort }}/anthropic-router
Gemini Native: http://127.0.0.1:{{ config.proxyPort }}/gemini/v1beta
OpenRouter: http://127.0.0.1:{{ config.proxyPort }}/openrouter/v1
TokenRouter auto: http://127.0.0.1:{{ config.proxyPort }}/pi-router/v1 + model auto:balance
Zo Computer: http://127.0.0.1:{{ config.proxyPort }}/zo/v1
Custom Gateway: http://127.0.0.1:{{ config.proxyPort }}/custom/v1</code></pre>
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
        <p>写入 <code>%USERPROFILE%\.deepseek\config.toml</code>，使用 DeepSeek-TUI 内置 DeepSeek provider 连接 OmniProxy 的 DeepSeek 账号池。</p>
        <pre class="help-code"><code>provider = "deepseek"
default_text_model = "deepseek-v4-pro"
[providers.deepseek]
base_url = "http://127.0.0.1:{{ config.proxyPort }}/deepseek/v1"
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
