<script setup>
import { ref } from 'vue'

const coreUrlsExpanded = ref(false)
const thirdPartyUrlsExpanded = ref(false)

const props = defineProps({
  config: {
    type: Object,
    required: true,
  },
  dataDirectory: {
    type: Object,
    required: true,
  },
  dataDirChanging: {
    type: Boolean,
    required: true,
  },
  autoStartChanging: {
    type: Boolean,
    required: true,
  },
  autoStartEnabled: {
    type: Boolean,
    required: true,
  },
  clearingBillingUsage: {
    type: Boolean,
    required: true,
  },
  clearingRequestHistory: {
    type: Boolean,
    required: true,
  },
})

defineEmits([
  'persist-config',
  'choose-data-directory',
  'toggle-auto-start',
  'clear-billing-usage',
  'clear-request-history',
])

const outboundProxyPresets = [
  { label: '10808 mixed', url: 'http://127.0.0.1:10808' },
  { label: '7890 mixed', url: 'http://127.0.0.1:7890' },
  { label: 'SOCKS5 10808', url: 'socks5://127.0.0.1:10808' },
]
const recommendedOutboundProxyModels = ['gpt-*', 'claude-*', 'gemini-*', '*/*']
const outboundProxyModelGroups = [
  {
    title: '国内网络建议出站',
    note: '这些模型或聚合模型 ID 通常依赖海外 API 入口，默认选中。',
    items: [
      {
        key: 'openai-codex',
        label: 'OpenAI / Codex',
        patterns: ['gpt-*'],
        description: 'gpt-5.5、gpt-5.4、gpt-5.4-high 等 OpenAI/Codex 模型',
        recommended: true,
      },
      {
        key: 'anthropic-claude',
        label: 'Anthropic Claude',
        patterns: ['claude-*'],
        description: 'claude-opus、claude-sonnet，以及 Claude 兼容模型名',
        recommended: true,
      },
      {
        key: 'google-gemini',
        label: 'Google Gemini',
        patterns: ['gemini-*'],
        description: 'gemini-3-pro-preview、gemini-3-flash-preview 等',
        recommended: true,
      },
      {
        key: 'provider-model-id',
        label: 'OpenRouter / 聚合模型 ID',
        patterns: ['*/*'],
        description: 'openai/gpt、anthropic/claude、google/gemini、meta-llama/* 等带 provider/ 前缀的模型',
        recommended: true,
      },
    ],
  },
  {
    title: '国内通常可直连',
    note: '这些是当前内置国内厂商模型，默认不走出站代理。',
    items: [
      {
        key: 'deepseek',
        label: 'DeepSeek',
        patterns: ['deepseek-*'],
        description: 'deepseek-v4-pro、deepseek-v4-flash',
      },
      {
        key: 'kimi',
        label: 'Kimi Code',
        patterns: ['kimi-*'],
        description: 'kimi-for-coding',
      },
      {
        key: 'zhipu',
        label: 'Zhipu GLM',
        patterns: ['glm-*', 'zhipu-*'],
        description: 'glm-5.1、zhipu-*',
      },
      {
        key: 'minimax',
        label: 'MiniMax',
        patterns: ['minimax-*'],
        description: 'MiniMax-M2.7 等',
      },
      {
        key: 'mimo',
        label: 'Xiaomi MiMo',
        patterns: ['mimo-*'],
        description: 'mimo-v2.5-pro、mimo-v2.5',
      },
    ],
  },
  {
    title: '取决于你的上游',
    note: '自定义网关、sub2api、TokenRouter 是否需要出站，取决于你配置的实际服务地址。',
    items: [
      {
        key: 'tokenrouter',
        label: 'TokenRouter 自动路由',
        patterns: ['auto:*', 'tokenrouter:*', 'tokenrouter/*'],
        description: 'auto:balance、auto:quality、tokenrouter/*',
      },
      {
        key: 'custom',
        label: '自定义网关模型',
        patterns: ['custom-*'],
        description: 'custom-model 或自定义兼容网关模型',
      },
    ],
  },
]

function setOutboundProxyUrl(url) {
  props.config.outboundProxyUrl = url
  props.config.outboundProxyEnabled = true
}

function resetOutboundProxyModels() {
  props.config.outboundProxyModels = [...recommendedOutboundProxyModels]
}

function toggleOutboundProxyModel(item) {
  if (isOutboundProxyModelSelected(item)) {
    removeOutboundProxyPatterns(item.patterns)
  } else {
    addOutboundProxyPatterns(item.patterns)
  }
}

function addOutboundProxyPatterns(patterns) {
  props.config.outboundProxyModels = normalizeOutboundProxyModels([
    ...(Array.isArray(props.config.outboundProxyModels) ? props.config.outboundProxyModels : []),
    ...patterns,
  ])
}

function removeOutboundProxyPatterns(patterns) {
  const keys = new Set(patterns.map((pattern) => String(pattern || '').trim().toLowerCase()).filter(Boolean))
  props.config.outboundProxyModels = (Array.isArray(props.config.outboundProxyModels)
    ? props.config.outboundProxyModels
    : []
  ).filter((item) => !keys.has(String(item || '').trim().toLowerCase()))
}

function isOutboundProxyModelSelected(item) {
  return item.patterns.every((pattern) => hasOutboundProxyPattern(pattern))
}

function hasOutboundProxyPattern(pattern) {
  const key = String(pattern || '').trim().toLowerCase()
  return selectedOutboundProxyModels().some((item) => String(item || '').trim().toLowerCase() === key)
}

function selectedOutboundProxyModels() {
  return Array.isArray(props.config.outboundProxyModels) ? props.config.outboundProxyModels : []
}

function selectedOutboundProxyRuleCount() {
  return selectedOutboundProxyModels().length
}

function customOutboundProxyModels() {
  const known = new Set(
    outboundProxyModelGroups.flatMap((group) => group.items).flatMap((item) => item.patterns.map((pattern) => pattern.toLowerCase())),
  )
  return selectedOutboundProxyModels().filter((model) => !known.has(String(model || '').trim().toLowerCase()))
}

function normalizeOutboundProxyModels(models) {
  const seen = new Set()
  const next = []
  for (const model of models) {
    const value = String(model || '').trim()
    const key = value.toLowerCase()
    if (!value || seen.has(key)) continue
    seen.add(key)
    next.push(value)
  }
  return next
}
</script>

<template>
  <section class="panel settings-panel">
    <div class="section-heading">
      <div>
        <h2>代理设置</h2>
        <p>保存后新请求会使用最新配置，端口变更需要重启代理</p>
      </div>
      <button type="button" class="primary-button" @click="$emit('persist-config')">保存设置</button>
    </div>
    <div class="settings-stack">
      <section class="settings-section">
        <div class="settings-section-head">
          <div>
            <h3>应用维护</h3>
            <p>本地数据目录和后台常驻集中放在这里。</p>
          </div>
        </div>
        <div class="settings-action-list">
          <div class="data-directory-row">
            <div>
              <span>数据目录</span>
              <strong>{{ dataDirectory.dataDir || '未加载' }}</strong>
              <small v-if="dataDirectory.pendingDataDir && dataDirectory.restartRequired">
                重启后使用：{{ dataDirectory.pendingDataDir }}
              </small>
              <small v-else-if="dataDirectory.envOverride">
                当前由 OMNIPROXY_DATA_DIR 环境变量控制
              </small>
              <small v-else-if="dataDirectory.bootstrapPath">
                指针文件：{{ dataDirectory.bootstrapPath }}
              </small>
            </div>
            <button
              type="button"
              class="ghost-button"
              :disabled="dataDirectory.envOverride || dataDirChanging"
              @click="$emit('choose-data-directory')"
            >
              {{ dataDirChanging ? '选择中' : '更改目录' }}
            </button>
          </div>
          <div class="data-directory-row startup-row">
            <div>
              <span>常驻后台</span>
              <strong>系统托盘与开机自启</strong>
              <small>关闭主窗口时保留托盘入口，可从托盘启动/停止代理、查看端口、打开主界面或退出。</small>
            </div>
            <button
              type="button"
              class="ghost-button"
              :disabled="autoStartChanging"
              @click="$emit('toggle-auto-start')"
            >
              {{ autoStartChanging ? '更新中' : autoStartEnabled ? '关闭自启' : '开启自启' }}
            </button>
          </div>
          <div class="data-directory-row maintenance-row">
            <div>
              <span>账单与请求历史</span>
              <strong>每日账单汇总保留 {{ config.historyRetentionDays || 14 }} 天</strong>
              <small>默认保留 14 天；每日汇总只记录最终 Token 用量，不保存完整请求日志。</small>
              <label class="inline-number-field">
                <span>保留天数</span>
                <input v-model="config.historyRetentionDays" type="number" min="1" max="365" />
              </label>
            </div>
            <div class="maintenance-actions">
              <button
                type="button"
                class="danger-button"
                :disabled="clearingBillingUsage"
                @click="$emit('clear-billing-usage')"
              >
                {{ clearingBillingUsage ? '清理中' : '清空账单汇总' }}
              </button>
              <button
                type="button"
                class="danger-button"
                :disabled="clearingRequestHistory"
                @click="$emit('clear-request-history')"
              >
                {{ clearingRequestHistory ? '清理中' : '清空请求历史' }}
              </button>
            </div>
          </div>
        </div>
      </section>

      <section class="settings-section">
        <div class="settings-section-head">
          <div>
            <h3>本机服务</h3>
            <p>端口和本地代理能力，端口变更后需要重启代理。</p>
          </div>
        </div>
        <div class="settings-grid compact-settings-grid">
          <label>
            <span>代理端口</span>
            <input v-model="config.proxyPort" type="number" min="1" max="65535" />
          </label>
          <label>
            <span>控制端口</span>
            <input v-model="config.controlPort" type="number" min="1" max="65535" />
          </label>
          <label class="toggle-field">
            <span>启用 Codex WebSocket</span>
            <input
              v-model="config.websocketMode"
              class="toggle-input"
              type="checkbox"
              true-value="enabled"
              false-value="disabled"
            />
            <span class="toggle-switch" aria-hidden="true">
              <span class="toggle-thumb"></span>
            </span>
          </label>
        </div>
      </section>

      <section class="settings-section">
        <div class="settings-section-head">
          <div>
            <h3>出站代理</h3>
            <p>只让指定模型请求走 Clash、v2rayN 等本机代理端口，未匹配模型继续直连。</p>
          </div>
        </div>
        <div class="settings-grid">
          <label class="toggle-field">
            <span>启用模型出站代理</span>
            <input v-model="config.outboundProxyEnabled" class="toggle-input" type="checkbox" />
            <span class="toggle-switch" aria-hidden="true">
              <span class="toggle-thumb"></span>
            </span>
          </label>
          <label>
            <span>本机代理地址</span>
            <input v-model="config.outboundProxyUrl" type="text" placeholder="10808 或 http://127.0.0.1:10808" />
          </label>
          <div class="wide-field settings-chip-field">
            <span>常用端口</span>
            <div class="settings-chip-list">
              <button
                v-for="preset in outboundProxyPresets"
                :key="preset.url"
                type="button"
                class="settings-chip-button"
                :class="{ active: config.outboundProxyUrl === preset.url }"
                @click="setOutboundProxyUrl(preset.url)"
              >
                {{ preset.label }}
              </button>
            </div>
            <small>Clash mixed-port 和 v2rayN mixed/http 入站可直接用 HTTP 地址；SOCKS 入站使用 socks5://。</small>
          </div>
          <div class="wide-field outbound-model-selector">
            <div class="outbound-model-selector-head">
              <div>
                <span>走出站代理的模型</span>
                <small>已选择 {{ selectedOutboundProxyRuleCount() }} 条匹配规则</small>
              </div>
              <button type="button" class="ghost-button compact-button" @click="resetOutboundProxyModels">
                恢复国内推荐
              </button>
            </div>
            <div
              v-for="group in outboundProxyModelGroups"
              :key="group.title"
              class="outbound-model-group"
            >
              <div class="outbound-model-group-head">
                <strong>{{ group.title }}</strong>
                <small>{{ group.note }}</small>
              </div>
              <div class="outbound-model-options">
                <button
                  v-for="item in group.items"
                  :key="item.key"
                  type="button"
                  class="outbound-model-option"
                  :class="{ active: isOutboundProxyModelSelected(item), recommended: item.recommended }"
                  @click="toggleOutboundProxyModel(item)"
                >
                  <span class="outbound-model-option-title">
                    <strong>{{ item.label }}</strong>
                    <em>{{ isOutboundProxyModelSelected(item) ? '走出站' : '直连' }}</em>
                  </span>
                  <small>{{ item.description }}</small>
                  <code>{{ item.patterns.join(' / ') }}</code>
                </button>
              </div>
            </div>
            <div v-if="customOutboundProxyModels().length" class="settings-chip-field">
              <span>未归类规则</span>
              <div class="settings-chip-list">
                <button
                  v-for="model in customOutboundProxyModels()"
                  :key="model"
                  type="button"
                  class="settings-chip-button active"
                  @click="removeOutboundProxyPatterns([model])"
                >
                  {{ model }}
                </button>
              </div>
              <small>这些规则来自旧配置；点击可移除。</small>
            </div>
          </div>
        </div>
      </section>

      <section class="settings-section">
        <div class="settings-section-head">
          <div>
            <h3>调度与保护</h3>
            <p>控制账号轮换、低额度跳过和失败重试。</p>
          </div>
        </div>
        <div class="settings-grid compact-settings-grid">
          <label>
            <span>账号调度模式</span>
            <select v-model="config.schedulingMode">
              <option value="queue">队列模式</option>
              <option value="balanced">优先平衡使用</option>
            </select>
          </label>
          <label>
            <span>额度切换阈值</span>
            <input v-model="config.switchThreshold" type="number" min="1" max="100" />
          </label>
          <label>
            <span>自动重试次数</span>
            <input v-model="config.maxRetries" type="number" min="0" max="5" />
          </label>
          <div class="settings-segmented-field">
            <span>MiMo 优先使用</span>
            <div class="settings-segmented" role="group" aria-label="MiMo 凭据优先级">
              <button
                type="button"
                :class="{ active: config.xiaomiCredentialPriority === 'mimo_token_plan' }"
                @click="config.xiaomiCredentialPriority = 'mimo_token_plan'"
              >
                Token Plan
              </button>
              <button
                type="button"
                :class="{ active: config.xiaomiCredentialPriority === 'api_key' }"
                @click="config.xiaomiCredentialPriority = 'api_key'"
              >
                按量 API
              </button>
            </div>
          </div>
        </div>
      </section>

      <section class="settings-section">
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

      <section class="settings-section">
        <div class="settings-section-head">
          <div>
            <h3>第三方路由</h3>
            <p>DeepSeek、Kimi、Zhipu GLM、MiniMax、Gemini、OpenRouter、TokenRouter、sub2api、Zo Computer、Xiaomi MiMo 和自定义网关入口。</p>
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
            <span>Zo Computer Base URL</span>
            <input v-model="config.zoBaseUrl" type="url" />
            <small>Zo 使用 /models/available 与 /zo/ask，上游协议由 OmniProxy 适配为 OpenAI / Anthropic。</small>
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
            <span>Xiaomi MiMo Token Plan OpenAI Base URL（海外 SGP）</span>
            <input v-model="config.xiaomiTokenPlanSgpBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>Xiaomi MiMo Token Plan Anthropic Base URL（海外 SGP）</span>
            <input v-model="config.xiaomiTokenPlanSgpAnthropicBaseUrl" type="url" />
          </label>
        </div>
      </section>
    </div>
  </section>
</template>
