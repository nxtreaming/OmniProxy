<script setup>
import { ref } from 'vue'

defineProps({
  config: {
    type: Object,
    required: true,
  },
})

const coreUrlsExpanded = ref(false)
const thirdPartyUrlsExpanded = ref(false)
</script>

<template>
  <section class="settings-section settings-url-section settings-url-section-core">
    <div class="settings-section-head">
      <div>
        <h3>OpenAI / Anthropic / Codex</h3>
        <p>常用协议入口和 Codex 额度查询地址。</p>
      </div>
      <button type="button" class="ghost-button compact-button" @click="coreUrlsExpanded = !coreUrlsExpanded">
        {{ coreUrlsExpanded ? '收起地址' : '展开地址' }}
      </button>
    </div>
    <div v-if="coreUrlsExpanded" class="settings-grid">
      <label class="wide-field">
        <span>OpenAI API Base URL</span>
        <input v-model="config.openaiBaseUrl" type="url" />
      </label>
      <label class="wide-field">
        <span>Anthropic API Base URL</span>
        <input v-model="config.anthropicBaseUrl" type="url" />
      </label>
      <label class="wide-field">
        <span>Codex ChatGPT Base URL</span>
        <input v-model="config.codexBaseUrl" type="url" />
      </label>
      <label class="wide-field">
        <span>Codex 限额查询地址</span>
        <input v-model="config.codexUsageEndpoint" type="url" />
      </label>
      <label class="wide-field">
        <span>兼容旧版上游 API Base URL</span>
        <input v-model="config.upstreamBaseUrl" type="url" />
      </label>
    </div>
  </section>

  <section class="settings-section settings-url-section settings-url-section-third-party">
    <div class="settings-section-head">
      <div>
        <h3>第三方路由</h3>
        <p>DeepSeek、Kimi、Zhipu GLM、MiniMax、Gemini、OpenRouter、TokenRouter、sub2api、new-api、AnyRouter、Forge AI、Zo Computer、Prem、Xiaomi MiMo 和自定义网关入口。</p>
      </div>
      <button type="button" class="ghost-button compact-button" @click="thirdPartyUrlsExpanded = !thirdPartyUrlsExpanded">
        {{ thirdPartyUrlsExpanded ? '收起地址' : '展开地址' }}
      </button>
    </div>
    <div v-if="thirdPartyUrlsExpanded" class="settings-grid">
      <label class="wide-field">
        <span>DeepSeek API Base URL</span>
        <input v-model="config.deepseekBaseUrl" type="url" />
      </label>
      <label class="wide-field">
        <span>DeepSeek Anthropic Base URL</span>
        <input v-model="config.deepseekAnthropicBaseUrl" type="url" />
      </label>
      <label class="wide-field">
        <span>Kimi Code Base URL</span>
        <input v-model="config.kimiBaseUrl" type="url" />
      </label>
      <label class="wide-field">
        <span>Zhipu GLM OpenAI Base URL</span>
        <input v-model="config.zhipuBaseUrl" type="url" />
      </label>
      <label class="wide-field">
        <span>Zhipu GLM Anthropic Base URL</span>
        <input v-model="config.zhipuAnthropicBaseUrl" type="url" />
      </label>
      <label class="wide-field">
        <span>MiniMax OpenAI Base URL</span>
        <input v-model="config.minimaxBaseUrl" type="url" />
      </label>
      <label class="wide-field">
        <span>MiniMax Anthropic Base URL</span>
        <input v-model="config.minimaxAnthropicBaseUrl" type="url" />
      </label>
      <label class="wide-field">
        <span>Gemini Native Base URL</span>
        <input v-model="config.geminiBaseUrl" type="url" />
      </label>
      <label class="wide-field">
        <span>OpenRouter OpenAI Base URL</span>
        <input v-model="config.openrouterBaseUrl" type="url" />
      </label>
      <label class="wide-field">
        <span>TokenRouter OpenAI Base URL</span>
        <input v-model="config.tokenrouterBaseUrl" type="url" />
      </label>
      <label class="wide-field">
        <span>sub2api 默认 Base URL</span>
        <input v-model="config.sub2apiBaseUrl" type="url" />
        <small>仅作为新增 sub2api 账号的默认填充值，以及旧账号未保存 Base URL 时的回退地址；协议由本地路径决定。</small>
      </label>
      <label class="wide-field">
        <span>new-api 默认 Base URL</span>
        <input v-model="config.newapiBaseUrl" type="url" />
        <small>仅作为新增 new-api 账号的默认填充值，以及旧账号未保存 Base URL 时的回退地址；协议由本地路径决定。</small>
      </label>
      <label class="wide-field">
        <span>AnyRouter 默认 Base URL</span>
        <input v-model="config.anyrouterBaseUrl" type="url" />
        <small>仅作为新增 AnyRouter 账号的默认填充值，以及旧账号未保存 Base URL 时的回退地址；OpenAI/Codex 会走 /v1，Claude Code 会走 Anthropic 路径。</small>
      </label>
      <label class="wide-field">
        <span>Forge AI Base URL</span>
        <input v-model="config.forgeBaseUrl" type="url" />
        <small>Forge 的 OpenAI Responses / Chat 和 Anthropic Messages 共用此地址；默认已包含 /v1。</small>
      </label>
      <label class="wide-field">
        <span>Zo Computer Base URL</span>
        <input v-model="config.zoBaseUrl" type="url" />
        <small>Zo 使用 /models/available 与 /zo/ask，上游协议由 OmniProxy 适配为 OpenAI / Anthropic。</small>
      </label>
      <label class="wide-field">
        <span>Prem confidential-proxy Base URL</span>
        <input v-model="config.premBaseUrl" type="url" />
        <small>填写本机 confidential-proxy 根地址，不要带 /v1；OmniProxy 会自动转发到 /openai/v1 或 /anthropic/v1。</small>
      </label>
      <label class="toggle-field wide-field">
        <span>自动启动 Prem confidential-proxy</span>
        <input v-model="config.premAutoStartPcciProxy" class="toggle-input" type="checkbox" />
        <span class="toggle-switch" aria-hidden="true">
          <span class="toggle-thumb"></span>
        </span>
        <small>存在 Prem 账号时，OmniProxy 会用 npx 启动官方 @premai/api-sdk confidential-proxy，并使用上方 Base URL 的本机端口。</small>
      </label>
      <label class="wide-field">
        <span>自定义网关 OpenAI Base URL</span>
        <input v-model="config.customGatewayBaseUrl" type="url" placeholder="https://your-gateway.example/v1" />
      </label>
      <label class="wide-field">
        <span>自定义网关 Anthropic Base URL</span>
        <input v-model="config.customGatewayAnthropicBaseUrl" type="url" placeholder="可选，留空则复用 OpenAI Base URL" />
      </label>
      <label class="wide-field">
        <span>Xiaomi MiMo 按量 OpenAI Base URL</span>
        <input v-model="config.xiaomiApiBaseUrl" type="url" />
      </label>
      <label class="wide-field">
        <span>Xiaomi MiMo 按量 Anthropic Base URL</span>
        <input v-model="config.xiaomiApiAnthropicBaseUrl" type="url" />
      </label>
      <label class="wide-field">
        <span>Xiaomi MiMo Token Plan OpenAI Base URL（中国区）</span>
        <input v-model="config.xiaomiTokenPlanBaseUrl" type="url" />
      </label>
      <label class="wide-field">
        <span>Xiaomi MiMo Token Plan Anthropic Base URL（中国区）</span>
        <input v-model="config.xiaomiTokenPlanAnthropicBaseUrl" type="url" />
      </label>
      <label class="wide-field">
        <span>Xiaomi MiMo Token Plan OpenAI Base URL（新加坡 SGP）</span>
        <input v-model="config.xiaomiTokenPlanSgpBaseUrl" type="url" />
      </label>
      <label class="wide-field">
        <span>Xiaomi MiMo Token Plan Anthropic Base URL（新加坡 SGP）</span>
        <input v-model="config.xiaomiTokenPlanSgpAnthropicBaseUrl" type="url" />
      </label>
      <label class="wide-field">
        <span>Xiaomi MiMo Token Plan OpenAI Base URL（欧洲 AMS）</span>
        <input v-model="config.xiaomiTokenPlanAmsBaseUrl" type="url" />
      </label>
      <label class="wide-field">
        <span>Xiaomi MiMo Token Plan Anthropic Base URL（欧洲 AMS）</span>
        <input v-model="config.xiaomiTokenPlanAmsAnthropicBaseUrl" type="url" />
      </label>
    </div>
  </section>
</template>
