<script setup>
import { computed } from 'vue'
import { ElButton } from 'element-plus'
import {
  CircleCheckFilled,
  Key,
  Lightning,
  Memo,
  Monitor,
  RefreshRight,
  SwitchButton,
  TrendCharts,
} from '@element-plus/icons-vue'
import { helpCredentialGroups, helpTroubleshootingItems, helpWorkflowSteps } from '../constants/help'
import { buildThirdPartyEndpointGroups } from '../utils/helpContent'

const props = defineProps({
  proxyStatus: { type: Object, required: true },
  config: { type: Object, required: true },
  activeTokensCount: { type: Number, default: 0 },
  tokenCount: { type: Number, default: 0 },
  lowTokensCount: { type: Number, default: 0 },
  invalidTokensCount: { type: Number, default: 0 },
  activeRequestsCount: { type: Number, default: 0 },
  todayProxyRequests: { type: Number, default: 0 },
  formatNumber: { type: Function, required: true },
})

const emit = defineEmits(['select-tab', 'copy-endpoint'])

const proxyPort = computed(() => props.proxyStatus?.port || props.config?.proxyPort || 3000)
const proxyEndpoint = computed(() => `127.0.0.1:${proxyPort.value}`)
const thirdPartyEndpointGroups = computed(() => buildThirdPartyEndpointGroups(proxyPort.value))

const helpReadinessCards = computed(() => [
  {
    label: '本地代理',
    value: props.proxyStatus?.running ? '运行中' : '未启动',
    detail: `入口 ${proxyEndpoint.value}`,
    state: props.proxyStatus?.running ? 'ok' : 'warning',
    icon: SwitchButton,
  },
  {
    label: '可用账号',
    value: `${props.activeTokensCount} / ${props.tokenCount}`,
    detail: props.tokenCount
      ? `${props.lowTokensCount} 个低额度，${props.invalidTokensCount} 个无效`
      : '先添加至少一个上游账号',
    state: props.activeTokensCount ? 'ok' : 'warning',
    icon: Key,
  },
  {
    label: '调度策略',
    value: schedulingModeText(props.config?.schedulingMode),
    detail: `低于 ${props.config?.switchThreshold ?? 15}% 跳过，最多重试 ${props.config?.maxRetries ?? 2} 次`,
    state: 'muted',
    icon: TrendCharts,
  },
  {
    label: '请求追踪',
    value: `${props.formatNumber(props.todayProxyRequests)} 次`,
    detail: `保留 ${props.config?.historyRetentionDays ?? 14} 天历史，当前 ${props.activeRequestsCount} 个实时请求`,
    state: props.activeRequestsCount ? 'ok' : 'muted',
    icon: Memo,
  },
])

function schedulingModeText(value) {
  if (value === 'balanced') return '优先平衡使用'
  if (value === 'queue') return '队列模式'
  return value || '-'
}

function selectTab(tab) {
  emit('select-tab', tab)
}

function copyEndpointValue(value, label) {
  emit('copy-endpoint', value, label)
}
</script>

<template>
  <section class="help-page">
    <div class="help-guide">
      <div class="help-readiness-grid" aria-label="当前接入状态">
        <article
          v-for="card in helpReadinessCards"
          :key="card.label"
          :class="['help-readiness-card', card.state]"
        >
          <component :is="card.icon" class="help-card-icon" aria-hidden="true" />
          <div>
            <span>{{ card.label }}</span>
            <strong>{{ card.value }}</strong>
            <small>{{ card.detail }}</small>
          </div>
        </article>
      </div>

      <div class="help-section-block">
        <div class="help-section-title">
          <Lightning class="help-section-icon" aria-hidden="true" />
          <div>
            <strong>推荐工作流</strong>
            <p>从账号准备到请求诊断，按顺序检查更容易定位问题。</p>
          </div>
        </div>
        <div class="help-flow">
          <article v-for="item in helpWorkflowSteps" :key="item.step" class="help-flow-step">
            <span class="help-step-index">{{ item.step }}</span>
            <div>
              <strong>{{ item.title }}</strong>
              <p>{{ item.description }}</p>
              <div class="help-step-actions">
                <el-button
                  v-for="action in item.actions"
                  :key="action.label"
                  size="small"
                  text
                  type="primary"
                  @click="selectTab(action.tab)"
                >
                  {{ action.label }}
                </el-button>
              </div>
            </div>
          </article>
        </div>
      </div>

      <div class="help-section-block">
        <div class="help-section-title">
          <Key class="help-section-icon" aria-hidden="true" />
          <div>
            <strong>账号类型怎么选</strong>
            <p>不同凭据会影响可用路由、额度展示和刷新方式。</p>
          </div>
        </div>
        <div class="help-credential-grid">
          <article v-for="group in helpCredentialGroups" :key="group.title">
            <strong>{{ group.title }}</strong>
            <code>{{ group.summary }}</code>
            <p>{{ group.detail }}</p>
          </article>
        </div>
      </div>

      <div class="help-section-block">
        <div class="help-section-title">
          <Monitor class="help-section-icon" aria-hidden="true" />
          <div>
            <strong>第三方客户端接口</strong>
            <p>Cherry Studio 这类客户端一般选择 OpenAI-compatible provider，填写 Base URL、任意非空 API Key 和模型名即可。</p>
          </div>
        </div>
        <div class="endpoint-reference">
          <div class="endpoint-reference-note">
            <strong>Cherry Studio 推荐</strong>
            <p>
              OpenAI 兼容：Base URL 填 <code>http://127.0.0.1:{{ proxyPort }}/v1</code>；Zo Computer 填
              <code>http://127.0.0.1:{{ proxyPort }}/zo/v1</code>；API Key 填 <code>omniproxy-local</code>。
            </p>
          </div>
          <section v-for="group in thirdPartyEndpointGroups" :key="group.title" class="endpoint-group">
            <div class="endpoint-group-head">
              <div>
                <strong>{{ group.title }}</strong>
                <p>{{ group.note }}</p>
              </div>
            </div>
            <div class="endpoint-table">
              <article v-for="endpoint in group.endpoints" :key="`${group.title}-${endpoint.name}`" class="endpoint-row">
                <div class="endpoint-main">
                  <span class="tag muted">{{ endpoint.protocol }}</span>
                  <strong>{{ endpoint.name }}</strong>
                  <p>{{ endpoint.use }}</p>
                </div>
                <div class="endpoint-fields">
                  <div>
                    <span>Base URL</span>
                    <code>{{ endpoint.baseUrl }}</code>
                    <button type="button" class="ghost-button compact-button" @click="copyEndpointValue(endpoint.baseUrl, 'Base URL')">
                      复制
                    </button>
                  </div>
                  <div>
                    <span>API Key</span>
                    <code>{{ endpoint.apiKey }}</code>
                    <button type="button" class="ghost-button compact-button" @click="copyEndpointValue(endpoint.apiKey, 'API Key')">
                      复制
                    </button>
                  </div>
                  <div>
                    <span>模型</span>
                    <code>{{ endpoint.models }}</code>
                    <button type="button" class="ghost-button compact-button" @click="copyEndpointValue(endpoint.models, '模型')">
                      复制
                    </button>
                  </div>
                </div>
              </article>
            </div>
          </section>
        </div>
      </div>

      <div class="help-section-block">
        <div class="help-section-title">
          <RefreshRight class="help-section-icon" aria-hidden="true" />
          <div>
            <strong>常见排查路径</strong>
            <p>先确认请求是否进入本机代理，再判断账号、额度、模型和上游响应。</p>
          </div>
        </div>
        <div class="help-troubleshooting-list">
          <article v-for="item in helpTroubleshootingItems" :key="item.problem">
            <CircleCheckFilled class="help-check-icon" aria-hidden="true" />
            <div>
              <strong>{{ item.problem }}</strong>
              <p>{{ item.action }}</p>
            </div>
          </article>
        </div>
      </div>
    </div>
  </section>
</template>
