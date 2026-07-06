<script setup>
import {
  ArrowLeft,
  ArrowRight,
  CircleCheckFilled,
  Lightning,
  Monitor,
  Refresh,
  SwitchButton,
  TrendCharts,
} from '@element-plus/icons-vue'
import DashboardContributionCalendar from './DashboardContributionCalendar.vue'

const props = defineProps({
  proxyStatus: { type: Object, required: true },
  proxyEndpoint: { type: String, required: true },
  dashboardSignals: { type: Array, required: true },
  activeTokens: { type: Array, required: true },
  invalidTokens: { type: Array, required: true },
  lowTokens: { type: Array, required: true },
  coolingTokens: { type: Array, required: true },
  exhaustedTokens: { type: Array, required: true },
  disabledTokens: { type: Array, required: true },
  watchTokens: { type: Array, required: true },
  riskTokens: { type: Array, required: true },
  totalProxyTokens: { type: Number, required: true },
  totalProxyInputTokens: { type: Number, required: true },
  totalProxyOutputTokens: { type: Number, required: true },
  todayProxyTokens: { type: Number, required: true },
  todayProxyRequests: { type: Number, required: true },
  totalProxyRequests: { type: Number, required: true },
  activeRequests: { type: Array, required: true },
  longRequestAlertSeconds: { type: Number, required: true },
  activeTokenIds: { type: Object, required: true },
  toolUsageRows: { type: Array, required: true },
  subscriptionOverviewTokens: { type: Array, required: true },
  subscriptionQuotaPage: { type: Number, required: true },
  subscriptionOverviewPageCount: { type: Number, required: true },
  subscriptionQuotaPageText: { type: String, required: true },
  pagedSubscriptionOverviewTokens: { type: Array, required: true },
  apiOverviewTokens: { type: Array, required: true },
  apiQuotaPage: { type: Number, required: true },
  apiOverviewPageCount: { type: Number, required: true },
  apiQuotaPageText: { type: String, required: true },
  pagedApiOverviewTokens: { type: Array, required: true },
  logs: { type: Array, required: true },
  dailyUsageRows: { type: Array, required: true },
  dashboardTrendRows: { type: Array, required: true },
  dashboardDailyUsageRows: { type: Array, required: true },
  formatNumber: { type: Function, required: true },
  formatTime: { type: Function, required: true },
  clientToolLabel: { type: Function, required: true },
  toolUsageMeta: { type: Function, required: true },
  toolUsageDuration: { type: Function, required: true },
  quotaOverviewRangeText: { type: Function, required: true },
  isTokenActiveNow: { type: Function, required: true },
  weeklyLimitReached: { type: Function, required: true },
  displayStatusClass: { type: Function, required: true },
  displayStatusLabel: { type: Function, required: true },
  providerLabel: { type: Function, required: true },
  quotaPrimaryLabel: { type: Function, required: true },
  quotaPercentValue: { type: Function, required: true },
  quotaPercentText: { type: Function, required: true },
  credentialLabel: { type: Function, required: true },
  apiQuotaDisplay: { type: Function, required: true },
  apiQuotaMeta: { type: Function, required: true },
  trendWidth: { type: Function, required: true },
  requestTrendWidth: { type: Function, required: true },
})

const emit = defineEmits(['toggle-proxy', 'refresh', 'open-settings', 'open-billing', 'open-trends', 'change-quota-page'])

function toggleProxy() {
  emit('toggle-proxy')
}

function refreshAll() {
  emit('refresh')
}

function openSettings() {
  emit('open-settings')
}

function openBilling() {
  emit('open-billing')
}

function openTrends() {
  emit('open-trends')
}

function changeQuotaOverviewPage(type, direction) {
  emit('change-quota-page', type, direction)
}

function requestAgeSeconds(request) {
  const startedAt = Date.parse(request?.startedAt || '')
  if (!Number.isFinite(startedAt)) return 0
  return Math.max(0, Math.round((Date.now() - startedAt) / 1000))
}

function requestDurationText(request) {
  const seconds = requestAgeSeconds(request)
  if (seconds < 60) return `${seconds} 秒`
  const minutes = Math.floor(seconds / 60)
  const rest = seconds % 60
  return `${minutes} 分 ${rest} 秒`
}

function isLongRequest(request) {
  return requestAgeSeconds(request) >= Number(props.longRequestAlertSeconds || 120)
}

function longActiveRequests() {
  return props.activeRequests.filter((request) => isLongRequest(request))
}
</script>

<template>
  <section class="view-grid dashboard-grid">
        <article class="metric-card account-status-card">
          <div class="metric-card-head">
            <span>账号状态</span>
            <CircleCheckFilled class="metric-icon success-icon" aria-hidden="true" />
          </div>
          <div class="account-status-metrics">
            <div>
              <strong>{{ activeTokens.length }}</strong>
              <small>正常账号</small>
            </div>
            <div>
              <strong>{{ invalidTokens.length }}</strong>
              <small>无效账号</small>
            </div>
            <div>
              <strong>{{ riskTokens.length }}</strong>
              <small>高风险</small>
            </div>
          </div>
          <small>关注 {{ watchTokens.length }} · 低额度 {{ lowTokens.length }} · 冷却 {{ coolingTokens.length }} · 耗尽 {{ exhaustedTokens.length }} · 停用 {{ disabledTokens.length }}</small>
        </article>
        <article class="metric-card">
          <div class="metric-card-head">
            <span>代理总 Token</span>
            <TrendCharts class="metric-icon" aria-hidden="true" />
          </div>
          <strong>{{ formatNumber(totalProxyTokens) }}</strong>
          <small>输入 {{ formatNumber(totalProxyInputTokens) }} · 输出 {{ formatNumber(totalProxyOutputTokens) }}</small>
        </article>
        <article class="metric-card">
          <div class="metric-card-head">
            <span>今日 Token</span>
            <Lightning class="metric-icon warning-icon" aria-hidden="true" />
          </div>
          <strong>{{ formatNumber(todayProxyTokens) }}</strong>
          <small>累计请求 {{ formatNumber(totalProxyRequests) }} 次</small>
        </article>
        <article class="metric-card">
          <div class="metric-card-head">
            <span>当前连接</span>
            <Monitor class="metric-icon" aria-hidden="true" />
          </div>
          <strong>{{ formatNumber(activeRequests.length) }}</strong>
          <small>正在占用 {{ activeTokenIds.size }} 个账号 · 长请求 {{ longActiveRequests().length }}</small>
        </article>

        <section class="panel full tool-usage-panel">
          <div class="section-heading">
            <div>
              <h2>编程工具账号占用</h2>
              <p>按 Codex、Claude Code、Gemini CLI、OpenCode、Pi Coding Agent、DeepSeek-TUI 等工具区分正在使用的账号</p>
            </div>
          </div>
          <div v-if="toolUsageRows.length" :class="['tool-usage-grid', { 'single-tool': toolUsageRows.length === 1 }]">
            <div
              v-for="row in toolUsageRows"
              :key="row.clientKey"
              :class="['tool-usage-row', { active: row.active }]"
            >
              <div>
                <strong>{{ row.clientName || clientToolLabel(row.clientKey) }}</strong>
                <small>{{ toolUsageMeta(row) }}</small>
              </div>
              <div>
                <span :class="['tag', row.active ? 'success' : 'muted']">
                  {{ row.active ? `使用中 ${row.activeCount}` : '最近使用' }}
                </span>
                <small v-if="toolUsageDuration(row)">{{ toolUsageDuration(row) }}</small>
              </div>
              <div class="tool-account-cell" :title="row.tokenText">
                <span>账号</span>
                <strong>{{ row.tokenText }}</strong>
              </div>
            </div>
          </div>
          <div v-else class="empty">暂无工具使用记录</div>
        </section>

        <section v-if="activeRequests.length" class="panel active-request-panel">
          <div class="section-heading">
            <div>
              <h2>活跃请求</h2>
              <p>长时间占用会按当前阈值标记</p>
            </div>
            <button v-if="longActiveRequests().length" type="button" class="danger-button" @click="toggleProxy">停止代理</button>
          </div>
          <div class="active-request-list">
            <div
              v-for="request in activeRequests.slice(0, 5)"
              :key="request.id"
              :class="['active-request-row', { warning: isLongRequest(request) }]"
            >
              <div>
                <strong>{{ request.clientName || clientToolLabel(request.clientKey) }}</strong>
                <small>{{ providerLabel(request.provider) }} · {{ request.model || '未识别模型' }}</small>
              </div>
              <span :class="['tag', isLongRequest(request) ? 'warning' : 'success']">
                {{ requestDurationText(request) }}
              </span>
              <small>账号 {{ request.tokenName || '-' }} · {{ formatTime(request.startedAt) }}</small>
            </div>
          </div>
        </section>

        <section class="panel wide quota-overview-panel">
          <div class="section-heading">
            <div>
              <h2>额度概览</h2>
              <p>订阅额度和 API / 余额状态分开展示</p>
            </div>
            <button type="button" class="ghost-button" @click="refreshAll">刷新</button>
          </div>
          <div class="quota-overview-grid">
            <section class="quota-overview-block">
              <div class="quota-overview-head">
                <div class="quota-overview-title">
                  <strong>订阅额度</strong>
                  <small>
                    Codex / Token Plan
                    <template v-if="subscriptionOverviewTokens.length">
                      · {{ quotaOverviewRangeText(subscriptionQuotaPage, subscriptionOverviewTokens.length) }}
                    </template>
                  </small>
                </div>
                <div v-if="subscriptionOverviewPageCount > 1" class="quota-overview-pager">
                  <button
                    type="button"
                    aria-label="上一页订阅额度"
                    :disabled="subscriptionQuotaPage <= 0"
                    @click="changeQuotaOverviewPage('subscription', -1)"
                  >
                    <ArrowLeft class="pager-icon" aria-hidden="true" />
                  </button>
                  <span>{{ subscriptionQuotaPageText }}</span>
                  <button
                    type="button"
                    aria-label="下一页订阅额度"
                    :disabled="subscriptionQuotaPage >= subscriptionOverviewPageCount - 1"
                    @click="changeQuotaOverviewPage('subscription', 1)"
                  >
                    <ArrowRight class="pager-icon" aria-hidden="true" />
                  </button>
                </div>
              </div>
              <div class="quota-list compact-quota-list">
                <div
                  v-for="item in pagedSubscriptionOverviewTokens"
                  :key="item.id"
                  :class="['quota-row', 'subscription-quota-row', { 'current-quota-row': isTokenActiveNow(item) }]"
                  :aria-current="isTokenActiveNow(item) ? 'true' : undefined"
                >
                  <div class="quota-account">
                    <div class="quota-account-title">
                      <strong>{{ item.name }}</strong>
                      <span v-if="isTokenActiveNow(item)" class="current-usage-badge">正在使用</span>
                      <span v-if="weeklyLimitReached(item)" class="limit-reached-badge">周限额已达</span>
                      <span :class="['tag', displayStatusClass(item)]">{{ displayStatusLabel(item) }}</span>
                    </div>
                    <small class="current-usage-meta">
                      {{ providerLabel(item.provider) }} · {{ quotaPrimaryLabel(item) }}
                    </small>
                  </div>
                  <div class="progress">
                    <span :style="{ width: `${quotaPercentValue(item, 'primaryRemainingPercent')}%` }"></span>
                  </div>
                  <small class="quota-percent">
                    {{ quotaPercentText(item, 'primaryRemainingPercent') }}
                  </small>
                </div>
                <div v-if="!subscriptionOverviewTokens.length" class="empty">暂无订阅额度账号</div>
              </div>
            </section>

            <section class="quota-overview-block">
              <div class="quota-overview-head">
                <div class="quota-overview-title">
                  <strong>API / 余额状态</strong>
                  <small>
                    API Key 不按百分比展示
                    <template v-if="apiOverviewTokens.length">
                      · {{ quotaOverviewRangeText(apiQuotaPage, apiOverviewTokens.length) }}
                    </template>
                  </small>
                </div>
                <div v-if="apiOverviewPageCount > 1" class="quota-overview-pager">
                  <button
                    type="button"
                    aria-label="上一页 API 余额"
                    :disabled="apiQuotaPage <= 0"
                    @click="changeQuotaOverviewPage('api', -1)"
                  >
                    <ArrowLeft class="pager-icon" aria-hidden="true" />
                  </button>
                  <span>{{ apiQuotaPageText }}</span>
                  <button
                    type="button"
                    aria-label="下一页 API 余额"
                    :disabled="apiQuotaPage >= apiOverviewPageCount - 1"
                    @click="changeQuotaOverviewPage('api', 1)"
                  >
                    <ArrowRight class="pager-icon" aria-hidden="true" />
                  </button>
                </div>
              </div>
              <div class="quota-list compact-quota-list">
                <div
                  v-for="item in pagedApiOverviewTokens"
                  :key="item.id"
                  :class="['quota-row', 'api-quota-row', { 'current-quota-row': isTokenActiveNow(item) }]"
                  :aria-current="isTokenActiveNow(item) ? 'true' : undefined"
                >
                  <div class="quota-account">
                    <div class="quota-account-title">
                      <strong>{{ item.name }}</strong>
                      <span v-if="isTokenActiveNow(item)" class="current-usage-badge">正在使用</span>
                      <span :class="['tag', displayStatusClass(item)]">{{ displayStatusLabel(item) }}</span>
                    </div>
                    <small class="current-usage-meta">{{ providerLabel(item.provider) }} · {{ credentialLabel(item) }}</small>
                  </div>
                  <div class="api-quota-value">
                    <strong>{{ apiQuotaDisplay(item) }}</strong>
                    <small>{{ apiQuotaMeta(item) }}</small>
                  </div>
                </div>
                <div v-if="!apiOverviewTokens.length" class="empty">暂无 API Key 账号</div>
              </div>
            </section>
          </div>
        </section>

        <section class="panel recent-log-panel">
          <div class="section-heading">
            <div>
              <h2>最近日志</h2>
              <p>最新代理转发记录</p>
            </div>
          </div>
          <div class="log-list compact">
            <div v-for="entry in logs.slice(0, 2)" :key="entry.id" class="log-row">
              <span :class="['dot', entry.level]"></span>
              <p>
                <span v-if="entry.clientName" class="log-inline-model">{{ entry.clientName }}</span>
                <span v-if="entry.model" class="log-inline-model">{{ entry.model }}</span>
                {{ entry.message }}
              </p>
              <small>{{ formatTime(entry.time) }}</small>
            </div>
            <div v-if="!logs.length" class="empty">暂无日志</div>
          </div>
        </section>

        <DashboardContributionCalendar :rows="dailyUsageRows" :format-number="formatNumber" />

        <section class="panel full usage-overview-panel">
          <div class="section-heading">
            <div>
              <h2>分天用量统计</h2>
              <p>Token 数来自上游 usage；请求数按成功通过代理返回的请求统计</p>
            </div>
            <div class="section-actions">
              <button type="button" class="ghost-button" @click.stop="openTrends">完整趋势</button>
              <button type="button" class="ghost-button" @click.stop="openBilling">账单明细</button>
            </div>
          </div>

          <div class="dashboard-usage-summary">
            <div>
              <span>今日 Token</span>
              <strong>{{ formatNumber(todayProxyTokens) }}</strong>
              <small>请求 {{ formatNumber(todayProxyRequests) }} 次</small>
            </div>
            <div>
              <span>总 Token</span>
              <strong>{{ formatNumber(totalProxyTokens) }}</strong>
              <small>输入 {{ formatNumber(totalProxyInputTokens) }} · 输出 {{ formatNumber(totalProxyOutputTokens) }}</small>
            </div>
            <div>
              <span>总请求</span>
              <strong>{{ formatNumber(totalProxyRequests) }}</strong>
              <small>最近 {{ formatNumber(dashboardTrendRows.length) }} 天趋势</small>
            </div>
            <div>
              <span>今日输入 / 输出</span>
              <strong>{{ formatNumber(dailyUsageRows[0]?.inputTokens || 0) }} / {{ formatNumber(dailyUsageRows[0]?.outputTokens || 0) }}</strong>
              <small>{{ dailyUsageRows[0]?.date || '暂无日期' }}</small>
            </div>
          </div>

          <div v-if="dashboardTrendRows.length" class="compact-trend-panels">
            <div class="compact-trend-panel" aria-label="最近 Token 趋势">
              <div class="trend-panel-head">
                <span>Token 趋势</span>
                <strong>{{ formatNumber(totalProxyTokens) }}</strong>
              </div>
              <div class="compact-trend-list">
                <div
                  v-for="row in dashboardTrendRows"
                  :key="row.date"
                  class="compact-trend-row"
                  :title="`${row.date} · ${formatNumber(row.totalTokens)} Token`"
                >
                  <span>{{ row.date.slice(5) }}</span>
                  <div class="compact-trend-track">
                    <i :style="{ width: trendWidth(row) }"></i>
                  </div>
                  <strong>{{ formatNumber(row.totalTokens) }}</strong>
                </div>
              </div>
            </div>
            <div class="compact-trend-panel" aria-label="最近请求次数趋势">
              <div class="trend-panel-head">
                <span>请求次数趋势</span>
                <strong>{{ formatNumber(totalProxyRequests) }}</strong>
              </div>
              <div class="compact-trend-list request-trend">
                <div
                  v-for="row in dashboardTrendRows"
                  :key="`requests-${row.date}`"
                  class="compact-trend-row"
                  :title="`${row.date} · ${formatNumber(row.requestCount)} 次请求`"
                >
                  <span>{{ row.date.slice(5) }}</span>
                  <div class="compact-trend-track">
                    <i :style="{ width: requestTrendWidth(row) }"></i>
                  </div>
                  <strong>{{ formatNumber(row.requestCount) }}</strong>
                </div>
              </div>
            </div>
          </div>
          <div class="usage-table compact-dashboard-usage-table">
            <div class="usage-row header">
              <span>日期</span>
              <span>总 Token</span>
              <span>输入</span>
              <span>输出</span>
              <span>请求</span>
            </div>
            <div v-for="row in dashboardDailyUsageRows" :key="row.date" class="usage-row">
              <span>{{ row.date }}</span>
              <strong>{{ formatNumber(row.totalTokens) }}</strong>
              <span>{{ formatNumber(row.inputTokens) }}</span>
              <span>{{ formatNumber(row.outputTokens) }}</span>
              <span>{{ formatNumber(row.requestCount) }}</span>
            </div>
            <div v-if="!dailyUsageRows.length" class="empty">暂无代理 Token 用量</div>
            <div v-else-if="dailyUsageRows.length > dashboardDailyUsageRows.length" class="usage-table-footer">
              <span>仅显示最近 {{ dashboardDailyUsageRows.length }} 天</span>
              <button type="button" @click.stop="openTrends">查看全部趋势</button>
            </div>
          </div>
        </section>
      </section>
</template>

<style src="./DashboardView.css"></style>
