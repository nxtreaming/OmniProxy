<script setup>
import { ref } from 'vue'

const coreUrlsExpanded = ref(false)
const thirdPartyUrlsExpanded = ref(false)

defineProps({
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
  mimoCookieImporting: {
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
  'import-mimo-cookie',
  'clear-billing-usage',
  'clear-request-history',
])
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
              type="checkbox"
              true-value="enabled"
              false-value="disabled"
            />
          </label>
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
            <p>DeepSeek、Kimi、Zhipu GLM、MiniMax、Gemini、OpenRouter、TokenRouter、sub2api、Xiaomi MiMo 和自定义网关入口。</p>
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
          <label class="wide-field">
            <span>Xiaomi MiMo 控制台 Cookie</span>
            <textarea
              v-model="config.xiaomiPlatformCookie"
              rows="3"
              placeholder="从 platform.xiaomimimo.com 登录态请求复制 Cookie，用于 Token Plan 额度查询"
              autocomplete="off"
              spellcheck="false"
            />
            <small>用于读取 /api/v1/balance 和 /api/v1/tokenPlan/usage；也可以从 HAR 自动导入。</small>
            <button
              type="button"
              class="ghost-button compact-button"
              :disabled="mimoCookieImporting"
              @click="$emit('import-mimo-cookie')"
            >
              {{ mimoCookieImporting ? '导入中' : '从 HAR 导入 Cookie' }}
            </button>
          </label>
        </div>
      </section>
    </div>
  </section>
</template>
