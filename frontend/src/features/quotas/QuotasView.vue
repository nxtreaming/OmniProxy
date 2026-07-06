<script setup>
import { computed, reactive, watch } from 'vue'
import { ArrowLeft, ArrowRight, CircleCheckFilled, Plus, Refresh, RefreshRight } from '@element-plus/icons-vue'
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
import {
  buildQuotaWorkspaceGroups,
  quotaWorkspaceLabel,
  quotaWorkspaceTitle,
} from './quotaWorkspaceGroups'

const props = defineProps({
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

const workspaceIndexes = reactive({})

const quotaCardGroups = computed(() => buildQuotaWorkspaceGroups(props.activeProviderTokens, workspaceIndexes))

watch(
  quotaCardGroups,
  (groups) => {
    const visibleIds = new Set(groups.map((group) => group.id))
    for (const id of Object.keys(workspaceIndexes)) {
      if (!visibleIds.has(id)) delete workspaceIndexes[id]
    }
  },
  { immediate: true },
)

function changeWorkspace(group, direction) {
  if (!group?.isWorkspaceGroup) return
  workspaceIndexes[group.id] = Math.max(0, Math.min(group.tokens.length - 1, group.index + direction))
}

function selectWorkspace(group, index) {
  if (!group?.isWorkspaceGroup) return
  workspaceIndexes[group.id] = Math.max(0, Math.min(group.tokens.length - 1, index))
}
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
        <p>
          {{ activeProviderEnabledCount }} 启用 / {{ activeProviderTokens.length }} 凭证
          <template v-if="quotaCardGroups.length !== activeProviderTokens.length">
            · {{ quotaCardGroups.length }} 个账号视图
          </template>
          · {{ activeProviderInfo.note }}
        </p>
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
        v-for="group in quotaCardGroups"
        :key="group.id"
        :class="[
          'quota-card',
          {
            'quota-card-disabled': group.current.disabled,
            'quota-workspace-card': group.isWorkspaceGroup,
          },
        ]"
        shadow="hover"
        :body-style="{ padding: '0' }"
      >
        <div class="quota-card-inner">
          <div class="quota-card-head">
            <div class="quota-card-title-row">
              <strong class="account-name">{{ group.accountName || group.current.name }}</strong>
              <div class="quota-status-tags">
                <el-tag
                  v-if="group.isWorkspaceGroup"
                  type="info"
                  effect="plain"
                  class="quota-chip"
                >
                  {{ group.tokens.length }} 工作区
                </el-tag>
                <el-tag
                  v-if="group.current.usage?.subscriptionQuotaAvailable && group.current.usage?.planType"
                  type="primary"
                  effect="plain"
                  class="quota-chip"
                >
                  {{ planLabel(group.current.usage?.planType) }}
                </el-tag>
                <el-tag v-if="weeklyLimitReached(group.current)" type="danger" effect="light">周限额已达</el-tag>
                <el-tag :type="displayStatusType(group.current)" effect="light" class="status-tag quota-chip">
                  {{ displayStatusLabel(group.current) }}
                </el-tag>
              </div>
            </div>
            <div class="quota-card-meta-row">
              <span>
                {{ isCodexToken(group.current) ? 'Codex auth.json' : credentialLabel(group.current) }}
                · {{ providerLabel(group.current.provider) }}
                <template v-if="isCodexToken(group.current) && group.current.accountId && !group.isWorkspaceGroup">
                  · <span class="quota-account-id" :title="quotaWorkspaceTitle(group.current)">account_id: {{ quotaWorkspaceLabel(group.current) }}</span>
                </template>
              </span>
              <div class="quota-card-actions">
                <el-button
                  size="small"
                  :class="['account-action-button', 'account-select-button', { selected: group.current.selected }]"
                  :type="group.current.selected ? 'primary' : ''"
                  :plain="!group.current.selected"
                  :icon="group.current.selected ? CircleCheckFilled : Plus"
                  :loading="switchingOnlyTokenIds[group.current.id]"
                  :disabled="group.current.disabled"
                  @click="$emit('toggle-token-selected', group.current)"
                >
                  {{ group.current.selected ? '已选' : '选择' }}
                </el-button>
                <el-tooltip content="刷新额度" placement="top">
                  <el-button
                    size="small"
                    class="account-action-button"
                    plain
                    :icon="Refresh"
                    :loading="validatingIds[group.current.id]"
                    @click="$emit('refresh-quota', group.current)"
                  >
                    刷新
                  </el-button>
                </el-tooltip>
              </div>
            </div>
            <div class="quota-health-row">
              <small class="health-line">{{ healthSummary(group.current) }}</small>
              <div v-if="group.isWorkspaceGroup" class="quota-workspace-switcher">
                <el-tooltip content="上一个工作区" placement="top">
                  <el-button
                    class="quota-workspace-nav"
                    circle
                    text
                    :icon="ArrowLeft"
                    :disabled="group.index <= 0"
                    aria-label="上一个工作区"
                    @click="changeWorkspace(group, -1)"
                  />
                </el-tooltip>
                <div class="quota-workspace-current">
                  <span>工作区 {{ group.index + 1 }} / {{ group.tokens.length }}</span>
                  <strong :title="quotaWorkspaceTitle(group.current, group.index)">
                    {{ quotaWorkspaceLabel(group.current, group.index) }}
                  </strong>
                </div>
                <el-tooltip content="下一个工作区" placement="top">
                  <el-button
                    class="quota-workspace-nav"
                    circle
                    text
                    :icon="ArrowRight"
                    :disabled="group.index >= group.tokens.length - 1"
                    aria-label="下一个工作区"
                    @click="changeWorkspace(group, 1)"
                  />
                </el-tooltip>
              </div>
            </div>
            <div v-if="group.isWorkspaceGroup" class="quota-workspace-dots" aria-label="工作区列表">
              <button
                v-for="(workspace, workspaceIndex) in group.tokens"
                :key="workspace.id"
                type="button"
                :class="{ active: workspaceIndex === group.index, selected: workspace.selected }"
                :title="quotaWorkspaceTitle(workspace, workspaceIndex)"
                @click="selectWorkspace(group, workspaceIndex)"
              >
                {{ workspaceIndex + 1 }}
              </button>
            </div>
          </div>

          <div
            :class="[
              'quota-layout',
              {
                'codex-layout': isCodexToken(group.current),
                'single-window-layout': quotaWindowCount(group.current) === 1,
                'api-only-layout': !isCodexToken(group.current) && quotaWindowCount(group.current) === 0,
              },
            ]"
          >
            <div v-if="showPrimaryQuotaWindow(group.current)" class="quota-limit">
              <div class="quota-limit-title">
                <span>{{ quotaPrimaryLabel(group.current) }}</span>
                <strong v-if="group.current.usage?.subscriptionQuotaAvailable">{{ quotaPercentText(group.current, 'primaryRemainingPercent') }}</strong>
                <strong v-else>-</strong>
              </div>
              <el-progress
                :percentage="quotaPercentValue(group.current, 'primaryRemainingPercent')"
                :show-text="false"
                :stroke-width="8"
              />
              <small v-if="group.current.usage?.subscriptionQuotaAvailable" class="quota-detail quota-reset-detail">
                <span>已用 <strong>{{ quotaPercentText(group.current, 'primaryUsedPercent') }}</strong></span>
                <span>{{ quotaResetLabel(group.current) }} <strong>{{ formatResetTime(group.current.usage.primaryResetAt) }}</strong></span>
              </small>
              <small v-else>{{ quotaUnavailableText(group.current) }}</small>
            </div>

            <div v-if="showSecondaryQuotaWindow(group.current)" class="quota-limit">
              <div class="quota-limit-title">
                <span>{{ quotaSecondaryLabel(group.current) }}</span>
                <strong v-if="group.current.usage?.subscriptionQuotaAvailable">{{ quotaPercentText(group.current, 'secondaryRemainingPercent') }}</strong>
                <strong v-else>-</strong>
              </div>
              <el-progress
                :percentage="quotaPercentValue(group.current, 'secondaryRemainingPercent')"
                :show-text="false"
                :stroke-width="8"
              />
              <small v-if="group.current.usage?.subscriptionQuotaAvailable" class="quota-detail quota-reset-detail">
                <span>已用 <strong>{{ quotaPercentText(group.current, 'secondaryUsedPercent') }}</strong></span>
                <span>{{ quotaResetLabel(group.current) }} <strong>{{ formatResetTime(group.current.usage.secondaryResetAt) }}</strong></span>
                <span v-if="codexWeeklyQuotaEstimateText(group.current)" :title="codexWeeklyQuotaEstimateMeta(group.current)">
                  预估 <strong>{{ codexWeeklyQuotaEstimateText(group.current) }}</strong>
                </span>
              </small>
              <small v-else>{{ quotaUnavailableText(group.current) }}</small>
            </div>

            <div v-if="!isCodexToken(group.current)" class="quota-stat quota-stat-balance">
              <span>{{ quotaStatLabel(group.current) }}</span>
              <strong>{{ hasBalanceUsage(group.current) ? quotaDisplay(group.current) : `${group.current.usage?.apiRemaining || group.current.remaining}%` }}</strong>
              <small>{{ quotaStatMeta(group.current) }}</small>
            </div>

            <div class="quota-stat quota-stat-usage">
              <span>代理请求</span>
              <strong>{{ formatNumber(group.current.stats?.requestCount) }} 次</strong>
              <small class="quota-detail token-usage-detail">
                <span v-for="metric in tokenUsageMetrics(group.current)" :key="metric.label">
                  {{ metric.label }} <strong>{{ metric.value }}</strong>
                </span>
              </small>
            </div>
          </div>

          <div v-if="balancePackages(group.current).length" class="balance-package-list">
            <div class="balance-package-head">
              <span>资源包明细</span>
              <small>Token 包计入余额，次数包仅展示</small>
            </div>
            <div
              v-for="(pkg, index) in balancePackages(group.current)"
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
      <div v-if="!quotaCardGroups.length" class="empty">
        暂无 {{ activeProviderInfo.label }} 账号
      </div>
    </div>
  </section>
</template>

<style src="./QuotasView.css"></style>
