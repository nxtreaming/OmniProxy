<script setup>
import { CircleCheckFilled, Plus, Refresh, RefreshRight } from '@element-plus/icons-vue'
import { formatNumber, formatResetTime } from '../../utils/format'
import {
  apiBalanceSummaryMeta,
  balancePackageAmount,
  balancePackageCounts,
  balancePackageMeta,
  balancePackages,
  balancePackageTypeLabel,
  codexWeeklyQuotaEstimateMeta,
  codexWeeklyQuotaEstimateText,
  displayStatusLabel,
  displayStatusType,
  formatBalance,
  hasBalanceUsage,
  healthSummary,
  isCodexToken,
  openRouterQuotaLimit,
  openRouterQuotaMeta,
  openRouterQuotaRemaining,
  openRouterQuotaValue,
  planLabel,
  quotaDisplay,
  quotaPercentText,
  quotaPercentValue,
  quotaPrimaryLabel,
  quotaResetLabel,
  quotaSecondaryLabel,
  quotaStatLabel,
  quotaStatMeta,
  quotaUnavailableText,
  quotaWindowCount,
  showPrimaryQuotaWindow,
  showSecondaryQuotaWindow,
  tokenUsageMetrics,
  weeklyLimitReached,
} from '../../utils/tokenDisplay'

defineProps({
  providers: { type: Array, required: true },
  activeProvider: { type: String, required: true },
  activeProviderInfo: { type: Object, required: true },
  activeProviderTokens: { type: Array, required: true },
  activeProviderEnabledCount: { type: Number, required: true },
  apiBalanceSummaries: { type: Array, default: () => [] },
  refreshingProvider: { type: Boolean, default: false },
  switchingOnlyTokenIds: { type: Object, required: true },
  validatingIds: { type: Object, required: true },
  providerTokens: { type: Function, required: true },
  credentialLabel: { type: Function, required: true },
  providerLabel: { type: Function, required: true },
})

defineEmits(['refresh', 'refresh-provider-quotas', 'select-provider', 'toggle-token-selected', 'refresh-quota'])
</script>

<template>
  <section class="panel quotas-page-panel">
    <div class="section-heading">
      <div>
        <h2>账号状态</h2>
        <p>按厂商查看订阅额度、API 剩余额度和代理用量</p>
      </div>
      <div class="section-actions">
        <el-button :icon="Refresh" @click="$emit('refresh')">刷新列表</el-button>
        <el-button
          type="primary"
          plain
          :icon="RefreshRight"
          :loading="refreshingProvider"
          @click="$emit('refresh-provider-quotas')"
        >
          全部刷新
        </el-button>
      </div>
    </div>

    <div class="provider-switch" aria-label="厂商选择">
      <button
        v-for="provider in providers"
        :key="provider.key"
        type="button"
        :class="{ active: activeProvider === provider.key }"
        @click="$emit('select-provider', provider.key)"
      >
        {{ provider.label }}
        <span>{{ providerTokens(provider.key).length }}</span>
      </button>
    </div>

    <div class="provider-summary">
      <div>
        <h3>{{ activeProviderInfo.label }}</h3>
        <p>{{ activeProviderEnabledCount }} 启用 / {{ activeProviderTokens.length }} 总数 · {{ activeProviderInfo.note }}</p>
      </div>
      <div
        v-if="activeProvider === 'openrouter' && activeProviderTokens.length"
        class="provider-api-balance-summary openrouter-provider-summary"
        aria-label="OpenRouter 额度"
      >
        <article v-for="item in activeProviderTokens" :key="`openrouter-provider-summary-${item.id}`">
          <span>{{ item.name }}</span>
          <strong>{{ openRouterQuotaRemaining(item) }}</strong>
          <small>{{ openRouterQuotaMeta(item) }}</small>
          <small>已用 {{ openRouterQuotaValue(item, 'balanceUsed') }} · 上限 {{ openRouterQuotaLimit(item) }}</small>
        </article>
      </div>
      <div
        v-else-if="apiBalanceSummaries.length"
        class="provider-api-balance-summary"
        aria-label="API Key 总额度"
      >
        <article v-for="summary in apiBalanceSummaries" :key="summary.unit">
          <span>API Key 总额度 · {{ summary.unit }}</span>
          <strong>{{ formatBalance(summary.remaining) }} {{ summary.unit }}</strong>
          <small>{{ apiBalanceSummaryMeta(summary) }}</small>
        </article>
      </div>
    </div>

    <div class="quota-card-grid">
      <el-card
        v-for="item in activeProviderTokens"
        :key="item.id"
        :class="['quota-card', { 'quota-card-disabled': item.disabled }]"
        shadow="hover"
        :body-style="{ padding: '0' }"
      >
        <div class="quota-card-inner">
          <div class="quota-card-head">
            <div class="quota-card-title-row">
              <strong class="account-name">{{ item.name }}</strong>
              <div class="quota-status-tags">
                <el-tag
                  v-if="item.usage?.subscriptionQuotaAvailable && item.usage?.planType"
                  type="primary"
                  effect="plain"
                  class="quota-chip"
                >
                  {{ planLabel(item.usage?.planType) }}
                </el-tag>
                <el-tag v-if="weeklyLimitReached(item)" type="danger" effect="light">周限额已达</el-tag>
                <el-tag :type="displayStatusType(item)" effect="light" class="status-tag quota-chip">
                  {{ displayStatusLabel(item) }}
                </el-tag>
              </div>
            </div>
            <div class="quota-card-meta-row">
              <span>{{ isCodexToken(item) ? 'Codex auth.json' : credentialLabel(item) }} · {{ providerLabel(item.provider) }}</span>
              <div class="quota-card-actions">
                <el-button
                  size="small"
                  :class="['account-action-button', 'account-select-button', { selected: item.selected }]"
                  :type="item.selected ? 'primary' : ''"
                  :plain="!item.selected"
                  :icon="item.selected ? CircleCheckFilled : Plus"
                  :loading="switchingOnlyTokenIds[item.id]"
                  :disabled="item.disabled"
                  @click="$emit('toggle-token-selected', item)"
                >
                  {{ item.selected ? '已选' : '选择' }}
                </el-button>
                <el-tooltip content="刷新额度" placement="top">
                  <el-button
                    size="small"
                    class="account-action-button"
                    plain
                    :icon="Refresh"
                    :loading="validatingIds[item.id]"
                    @click="$emit('refresh-quota', item)"
                  >
                    刷新
                  </el-button>
                </el-tooltip>
              </div>
            </div>
            <small class="health-line">{{ healthSummary(item) }}</small>
          </div>

          <div
            :class="[
              'quota-layout',
              {
                'codex-layout': isCodexToken(item),
                'single-window-layout': quotaWindowCount(item) === 1,
                'api-only-layout': !isCodexToken(item) && quotaWindowCount(item) === 0,
              },
            ]"
          >
            <div v-if="showPrimaryQuotaWindow(item)" class="quota-limit">
              <div class="quota-limit-title">
                <span>{{ quotaPrimaryLabel(item) }}</span>
                <strong v-if="item.usage?.subscriptionQuotaAvailable">{{ quotaPercentText(item, 'primaryRemainingPercent') }}</strong>
                <strong v-else>-</strong>
              </div>
              <el-progress
                :percentage="quotaPercentValue(item, 'primaryRemainingPercent')"
                :show-text="false"
                :stroke-width="8"
              />
              <small v-if="item.usage?.subscriptionQuotaAvailable" class="quota-detail quota-reset-detail">
                <span>已用 <strong>{{ quotaPercentText(item, 'primaryUsedPercent') }}</strong></span>
                <span>{{ quotaResetLabel(item) }} <strong>{{ formatResetTime(item.usage.primaryResetAt) }}</strong></span>
              </small>
              <small v-else>{{ quotaUnavailableText(item) }}</small>
            </div>

            <div v-if="showSecondaryQuotaWindow(item)" class="quota-limit">
              <div class="quota-limit-title">
                <span>{{ quotaSecondaryLabel(item) }}</span>
                <strong v-if="item.usage?.subscriptionQuotaAvailable">{{ quotaPercentText(item, 'secondaryRemainingPercent') }}</strong>
                <strong v-else>-</strong>
              </div>
              <el-progress
                :percentage="quotaPercentValue(item, 'secondaryRemainingPercent')"
                :show-text="false"
                :stroke-width="8"
              />
              <small v-if="item.usage?.subscriptionQuotaAvailable" class="quota-detail quota-reset-detail">
                <span>已用 <strong>{{ quotaPercentText(item, 'secondaryUsedPercent') }}</strong></span>
                <span>{{ quotaResetLabel(item) }} <strong>{{ formatResetTime(item.usage.secondaryResetAt) }}</strong></span>
                <span v-if="codexWeeklyQuotaEstimateText(item)" :title="codexWeeklyQuotaEstimateMeta(item)">
                  预估 <strong>{{ codexWeeklyQuotaEstimateText(item) }}</strong>
                </span>
              </small>
              <small v-else>{{ quotaUnavailableText(item) }}</small>
            </div>

            <div v-if="!isCodexToken(item)" class="quota-stat quota-stat-balance">
              <span>{{ quotaStatLabel(item) }}</span>
              <strong>{{ hasBalanceUsage(item) ? quotaDisplay(item) : `${item.usage?.apiRemaining || item.remaining}%` }}</strong>
              <small>{{ quotaStatMeta(item) }}</small>
            </div>

            <div class="quota-stat quota-stat-usage">
              <span>代理请求</span>
              <strong>{{ formatNumber(item.stats?.requestCount) }} 次</strong>
              <small class="quota-detail token-usage-detail">
                <span v-for="metric in tokenUsageMetrics(item)" :key="metric.label">
                  {{ metric.label }} <strong>{{ metric.value }}</strong>
                </span>
              </small>
            </div>
          </div>

          <div v-if="balancePackages(item).length" class="balance-package-list">
            <div class="balance-package-head">
              <span>资源包明细</span>
              <small>Token 包计入余额，次数包仅展示</small>
            </div>
            <div
              v-for="(pkg, index) in balancePackages(item)"
              :key="`${pkg.name || 'package'}-${pkg.consumeType || 'token'}-${pkg.expirationTime || ''}-${index}`"
              :class="['balance-package-row', { muted: !balancePackageCounts(pkg) }]"
              :title="pkg.suitableScene || pkg.suitableModel || pkg.name"
            >
              <div>
                <strong>{{ pkg.name || balancePackageTypeLabel(pkg) }}</strong>
                <small>{{ balancePackageMeta(pkg) }}</small>
              </div>
              <div>
                <span class="package-type">{{ balancePackageTypeLabel(pkg) }}</span>
                <strong>{{ balancePackageAmount(pkg) }}</strong>
              </div>
            </div>
          </div>
        </div>
      </el-card>
      <div v-if="!activeProviderTokens.length" class="empty">
        暂无 {{ activeProviderInfo.label }} 账号
      </div>
    </div>
  </section>
</template>

<style src="./QuotasView.css"></style>
