<script setup>
import { defineAsyncComponent } from 'vue'
import DiagnosticDrawer from '../DiagnosticDrawer.vue'
import appIconUrl from '../../assets/appicon.png'
import { providers } from '../../constants/app'
import { claudeModelOptions, claudeModelSelectionLimit } from '../../constants/claudeModels'
import { formatDuration, formatNumber, formatTime } from '../../utils/format'
import {
  apiQuotaDisplay,
  apiQuotaMeta,
  displayStatusClass,
  displayStatusLabel,
  displayStatusType,
  formatBalance,
  healthSummary,
  quotaDisplay,
  quotaPercentText,
  quotaPercentValue,
  quotaPrimaryLabel,
  weeklyLimitReached,
} from '../../utils/tokenDisplay'
import { openExternalURL } from '../../services/api'
import { Moon, Sunny, SwitchButton } from '@element-plus/icons-vue'
import { useOmniProxyApp } from '../../app/useOmniProxyApp'

const DashboardView = defineAsyncComponent(() => import('../DashboardView.vue'))
const AboutView = defineAsyncComponent(() => import('../AboutView.vue'))
const BillingView = defineAsyncComponent(() => import('../../features/billing/BillingView.vue'))
const FirstUseGuideModal = defineAsyncComponent(() => import('../FirstUseGuideModal.vue'))
const GatewayRoutesView = defineAsyncComponent(() => import('../../features/gateway-routing/GatewayRoutesView.vue'))
const HistoryView = defineAsyncComponent(() => import('../../features/history/HistoryView.vue'))
const HelpView = defineAsyncComponent(() => import('../HelpView.vue'))
const LogsView = defineAsyncComponent(() => import('../LogsView.vue'))
const OpenRouterChatView = defineAsyncComponent(() => import('../../features/openrouter-chat/OpenRouterChatView.vue'))
const QuickstartView = defineAsyncComponent(() => import('../QuickstartView.vue'))
const QuotasView = defineAsyncComponent(() => import('../../features/quotas/QuotasView.vue'))
const SettingsView = defineAsyncComponent(() => import('../../features/settings/SettingsView.vue'))
const TokenBatchImportModal = defineAsyncComponent(() => import('../TokenBatchImportModal.vue'))
const TokenEditorModal = defineAsyncComponent(() => import('../TokenEditorModal.vue'))
const TokenTrendView = defineAsyncComponent(() => import('../TokenTrendView.vue'))
const TokensView = defineAsyncComponent(() => import('../../features/tokens/TokensView.vue'))

const {
  activeProvider, activeProviderAPIBalanceSummaries, activeProviderEnabledCount, activeProviderInfo, activeProviderTokens, activeRequests, activeTab, activeTokenIds,
  activeTokens, afterPageEnter, apiOverviewPageCount, apiOverviewTokens, apiQuotaPage, apiQuotaPageText, appInfo, appThemeLabel,
  autoStartChanging, autoStartEnabled, batchImportForm, batchImportPlaceholder, batchImporting, billingDates, billingUsage, canConfigureClaudeModels,
  changeBillingDate, changeQuotaOverviewPage, chooseDataDirectory, claudeCliRestoring, claudeDesktopConfiguring, claudeDesktopRestoring, claudeModelsConfiguring,
  clearBillingUsageData, clearRequestHistoryData, clearingBillingUsage, clearingRequestHistory, clientToolLabel, closeBatchImport, closeDeleteConfirm, closeFirstUseGuide,
  closeForm, closeHistoryDiagnosis, closeTitlebarUpdatePopover, closeWindow, codexAuthImporting, codexConfiguring, codexRestoring, config,
  confirmRemoveToken, confirmTitlebarUpdatePopover, configureLocalClaudeDesktopModels, configureLocalClaudeModels, configureLocalCodex, configureLocalDeepSeekTUI,
  configureLocalGemini, configureLocalOpenCode, configureLocalPi, coolingTokens, copyEndpointValue, credentialDisplay, credentialLabel, credentialPlaceholder,
  currentFirstUseGuideStep, currentTabLabel, dailyUsageRows, dashboardDailyUsageRows, dashboardSignals, dashboardTrendRows, dataDirChanging, dataDirectory,
  deepSeekTUIConfiguring, deepSeekTUIRestoring, deleteBusy, deleteCandidate, disabledTokens, errorMessage, exhaustedTokens, exportCodexAuthBackups,
  exportRequestHistory, exportTokenBackup, exportingCodexAuth, exportingHistory, exportingTokens, firstUseGuideStepIndex, firstUseGuideSteps, firstUseGuideVisible,
  form, geminiConfiguring, geminiRestoring, hasWailsRuntime, hideWorkspaceScrollbar, importCodexAuthFiles, installReadyUpdateFromUpdateSurface, invalidTokens,
  isAutoNameForm, isClaudeModelOptionDisabled, isDark, isTokenActiveNow, lastUpdateCheckedAt, lastUpdateInfo, loading, logs,
  lowTokens, manualCheckForUpdates, minimiseWindow, mobileSidebarOpen, navSections, nextFirstUseGuideStep, onBatchImportProviderChange, onProviderChange,
  openBatchImport, openBillingView, openCodexAuthFilePicker, openCreateForm, openEditForm, openOpenRouterChat, openRouterModels, openRouterModelsCached,
  openRouterModelsError, openRouterModelsFetchedAt, openRouterModelsLoading, openRouterTokens, opencodeConfiguring, opencodeRestoring, pagedApiOverviewTokens,
  pagedSubscriptionOverviewTokens, piConfiguring, piRestoring, persistConfig, previousFirstUseGuideStep, providerLabel, providerTokens, proxyEndpoint,
  proxyStatus, quotaOverviewRangeText, quotaRefreshProgress, refreshAll, refreshAuthToken, refreshBilling, refreshHistory, refreshOpenRouterModels,
  refreshProviderQuotas, refreshQuota, refreshRealtime, refreshingProvider, refreshingTokenIds, refreshTaskAutomationBrowserProfiles, removeToken, requestHistory,
  requestHistorySummary, requestTrendWidth, restoreActiveWorkspaceScroll, restoreLocalClaude, restoreLocalClaudeDesktop, restoreLocalCodex, restoreLocalDeepSeekTUI,
  restoreLocalGemini, restoreLocalOpenCode, restoreLocalPi, runFirstUseGuideAction, selectOpenRouterChatModel, selectProvider, selectTab, selectedBillingDate,
  selectedClaudeModelLabels, selectedClaudeModels, selectedHistoryEntry, selectedOpenRouterChatModel, startUpdateDownload, startWindowResize, submitBatchImport,
  submitForm, subscriptionOverviewPageCount, subscriptionOverviewTokens, subscriptionQuotaPage, subscriptionQuotaPageText, successMessage, switchingOnlyTokenIds,
  tabIcons, taskAutomationBrowserProfiles, taskAutomationBrowserProfilesError, taskAutomationBrowserProfilesLoading, titlebarUpdatePopoverOpen, titlebarUpdatePrompt,
  titlebarUpdateVisible, todayProxyRequests, todayProxyTokens, toggleAppTheme, toggleAutoStart, toggleProxy, toggleTitlebarUpdatePopover,
  toggleTokenEnabled, toggleTokenSelected, toggleWindowMaximise, togglingTokenIds, tokens, toolUsageDuration, toolUsageMeta, toolUsageRows,
  totalProxyInputTokens, totalProxyOutputTokens, totalProxyRequests, totalProxyTokens, trendWidth, updateChecking, updateDownloadStatus, validatingIds,
  verifyToken, windowMaximised, workspaceRef, workspaceScrollbarVisible, handleWorkspacePointerMove, handleWorkspaceScroll,
} = useOmniProxyApp()
</script>

<template>
  <div class="shell" :class="{ dark: isDark }">
    <header
      class="window-titlebar"
      :class="{ maximised: windowMaximised }"
      aria-label="窗口控制栏"
      @dblclick="toggleWindowMaximise"
    >
      <div class="window-titlebar-drag">
        <img :src="appIconUrl" alt="" />
        <div>
          <strong>OmniProxy</strong>
          <span>{{ appInfo.isDevelopment ? 'Dev' : appInfo.version }} · {{ proxyStatus.running ? '代理运行中' : '代理未启动' }}</span>
        </div>
      </div>
      <div
        v-if="titlebarUpdateVisible"
        class="window-titlebar-actions"
        aria-label="应用状态"
        @pointerdown.stop
      >
        <button
          type="button"
          class="titlebar-update-button"
          :title="titlebarUpdatePrompt.tooltip"
          aria-haspopup="dialog"
          :aria-expanded="titlebarUpdatePopoverOpen"
          @click.stop="toggleTitlebarUpdatePopover"
          @dblclick.stop
        >
          <span class="titlebar-update-mark" aria-hidden="true"></span>
          <span>新版本</span>
        </button>
        <div
          v-if="titlebarUpdatePopoverOpen"
          class="titlebar-update-popover"
          role="dialog"
          aria-label="新版本提示"
          @click.stop
          @dblclick.stop
        >
          <div class="titlebar-update-popover-head">
            <span class="titlebar-update-popover-icon" aria-hidden="true"></span>
            <div>
              <span class="titlebar-update-popover-kicker">{{ titlebarUpdatePrompt.badge }}</span>
              <strong>{{ titlebarUpdatePrompt.title }}</strong>
            </div>
            <button
              type="button"
              class="titlebar-update-popover-close"
              aria-label="关闭更新提示"
              @click="closeTitlebarUpdatePopover"
            >
              <span aria-hidden="true"></span>
            </button>
          </div>
          <p>{{ titlebarUpdatePrompt.description }}</p>
          <div class="titlebar-update-popover-meta">
            <div>
              <span>当前版本</span>
              <strong>{{ titlebarUpdatePrompt.currentVersion }}</strong>
            </div>
            <div>
              <span>最新版本</span>
              <strong>{{ titlebarUpdatePrompt.latestVersion }}</strong>
            </div>
          </div>
          <div class="titlebar-update-popover-actions">
            <button type="button" class="ghost-button compact-button" @click="closeTitlebarUpdatePopover">稍后</button>
            <button type="button" class="primary-button compact-button" @click="confirmTitlebarUpdatePopover">
              {{ titlebarUpdatePrompt.primaryText }}
            </button>
          </div>
        </div>
      </div>
      <div class="window-controls" aria-label="窗口操作">
        <button type="button" class="window-control minimise" aria-label="最小化" @click.stop="minimiseWindow">
          <span class="control-mark" aria-hidden="true"></span>
        </button>
        <button
          type="button"
          :class="['window-control', windowMaximised ? 'restore' : 'maximise']"
          :aria-label="windowMaximised ? '还原窗口' : '最大化'"
          @click.stop="toggleWindowMaximise"
        >
          <span class="control-mark" aria-hidden="true"></span>
        </button>
        <button type="button" class="window-control close" aria-label="关闭窗口" @click.stop="closeWindow">
          <span class="control-mark" aria-hidden="true"></span>
        </button>
      </div>
    </header>
    <div
      v-if="hasWailsRuntime()"
      class="window-resize-edge window-resize-edge-right"
      aria-hidden="true"
      @mousedown.prevent.stop="startWindowResize('e-resize')"
    ></div>

    <div
      v-if="mobileSidebarOpen"
      class="mobile-sidebar-backdrop"
      aria-hidden="true"
      @click="mobileSidebarOpen = false"
    ></div>

    <aside class="sidebar" :class="{ open: mobileSidebarOpen }">
      <div class="brand">
        <div class="brand-mark">
          <img :src="appIconUrl" alt="" />
        </div>
        <div>
          <strong>OmniProxy</strong>
          <span>本地 API 网关</span>
        </div>
      </div>

      <div class="sidebar-status">
        <div class="sidebar-status-main">
          <div :class="['status-light', { online: proxyStatus.running }]"></div>
          <div>
            <strong>{{ proxyStatus.running ? '代理运行中' : '代理未启动' }}</strong>
            <span>{{ proxyEndpoint }} · {{ tokens.length }} 个账号</span>
          </div>
        </div>
        <div class="sidebar-status-meta">
          <div>
            <span>端口</span>
            <strong>{{ proxyStatus.port || config.proxyPort }}</strong>
          </div>
          <div>
            <span>可用账号</span>
            <strong>{{ activeTokens.length }}</strong>
          </div>
          <div>
            <span>状态</span>
            <strong>{{ proxyStatus.running ? '运行中' : '已停止' }}</strong>
          </div>
        </div>
        <button type="button" class="sidebar-proxy-button" @click="toggleProxy">
          <component :is="SwitchButton" class="button-icon" aria-hidden="true" />
          <span>{{ proxyStatus.running ? '停止代理' : '启动代理' }}</span>
        </button>
      </div>

      <nav class="nav-list">
        <section v-for="section in navSections" :key="section.label" class="nav-section">
          <span class="nav-section-label">{{ section.label }}</span>
          <button
            v-for="tab in section.items"
            :key="tab.key"
            type="button"
            :class="{ active: activeTab === tab.key }"
            @click="selectTab(tab.key)"
          >
            <component :is="tabIcons[tab.key]" class="nav-icon" aria-hidden="true" />
            <span>{{ tab.label }}</span>
          </button>
        </section>
      </nav>

      <div class="sidebar-tools">
        <button type="button" class="ghost-button" @click="toggleAppTheme">
          <component :is="isDark ? Sunny : Moon" class="button-icon" aria-hidden="true" />
          <span>{{ appThemeLabel }}</span>
        </button>
      </div>
    </aside>

    <main
      ref="workspaceRef"
      :class="[
        'workspace',
        {
          'openrouter-workspace': activeTab === 'openrouter-chat',
          'logs-workspace': activeTab === 'logs',
          'workspace-scrollbar-visible': workspaceScrollbarVisible,
        },
      ]"
      @pointermove="handleWorkspacePointerMove"
      @pointerleave="hideWorkspaceScrollbar"
      @scroll="handleWorkspaceScroll"
    >
      <header class="topbar">
        <button
          type="button"
          class="mobile-menu-button"
          :aria-expanded="mobileSidebarOpen"
          aria-label="打开导航"
          @click="mobileSidebarOpen = true"
        >
          <span></span>
          <span></span>
          <span></span>
        </button>
        <div class="topbar-title">
          <p class="eyebrow">本地控制台</p>
          <h1>{{ currentTabLabel }}</h1>
          <p class="topbar-subtitle">OmniProxy 桌面网关</p>
        </div>
      </header>

      <TransitionGroup name="quota-refresh" tag="div" class="toast-stack quota-refresh-stack" aria-live="polite">
        <div v-if="quotaRefreshProgress.visible" key="quota-refresh" class="notice quota-refresh-toast" role="status">
          <div class="quota-refresh-orb" aria-hidden="true">
            <span></span>
          </div>
          <div class="quota-refresh-body">
            <div class="quota-refresh-title-row">
              <strong>刷新中{{ quotaRefreshProgress.percent }}%</strong>
              <span>{{ quotaRefreshProgress.completed }} / {{ quotaRefreshProgress.total }}</span>
            </div>
            <div
              class="quota-refresh-track"
              role="progressbar"
              aria-label="额度刷新进度"
              aria-valuemin="0"
              aria-valuemax="100"
              :aria-valuenow="quotaRefreshProgress.percent"
            >
              <span :style="{ width: `${quotaRefreshProgress.percent}%` }"></span>
            </div>
            <small>{{ quotaRefreshProgress.currentName || `${quotaRefreshProgress.providerLabel} 额度刷新中` }}</small>
          </div>
        </div>
      </TransitionGroup>

      <TransitionGroup name="snackbar" tag="div" class="toast-stack" aria-live="polite">
        <div v-if="errorMessage" key="error" class="alert" role="alert">
          <span class="toast-message">{{ errorMessage }}</span>
          <button type="button" aria-label="关闭错误提示" @click="errorMessage = ''">×</button>
        </div>
        <div v-if="successMessage" key="success" class="notice" role="status">
          <span class="toast-message">{{ successMessage }}</span>
          <button type="button" aria-label="关闭成功提示" @click="successMessage = ''">×</button>
        </div>
      </TransitionGroup>

      <Transition
        name="page-switch"
        mode="out-in"
        appear
        @before-enter="restoreActiveWorkspaceScroll"
        @after-enter="afterPageEnter"
      >
      <DashboardView
        v-if="activeTab === 'dashboard'"
        key="dashboard"
        :proxy-status="proxyStatus"
        :proxy-endpoint="proxyEndpoint"
        :dashboard-signals="dashboardSignals"
        :active-tokens="activeTokens"
        :invalid-tokens="invalidTokens"
        :low-tokens="lowTokens"
        :cooling-tokens="coolingTokens"
        :exhausted-tokens="exhaustedTokens"
        :disabled-tokens="disabledTokens"
        :total-proxy-tokens="totalProxyTokens"
        :total-proxy-input-tokens="totalProxyInputTokens"
        :total-proxy-output-tokens="totalProxyOutputTokens"
        :today-proxy-tokens="todayProxyTokens"
        :today-proxy-requests="todayProxyRequests"
        :total-proxy-requests="totalProxyRequests"
        :active-requests="activeRequests"
        :active-token-ids="activeTokenIds"
        :tool-usage-rows="toolUsageRows"
        :subscription-overview-tokens="subscriptionOverviewTokens"
        :subscription-quota-page="subscriptionQuotaPage"
        :subscription-overview-page-count="subscriptionOverviewPageCount"
        :subscription-quota-page-text="subscriptionQuotaPageText"
        :paged-subscription-overview-tokens="pagedSubscriptionOverviewTokens"
        :api-overview-tokens="apiOverviewTokens"
        :api-quota-page="apiQuotaPage"
        :api-overview-page-count="apiOverviewPageCount"
        :api-quota-page-text="apiQuotaPageText"
        :paged-api-overview-tokens="pagedApiOverviewTokens"
        :logs="logs"
        :daily-usage-rows="dailyUsageRows"
        :dashboard-trend-rows="dashboardTrendRows"
        :dashboard-daily-usage-rows="dashboardDailyUsageRows"
        :format-number="formatNumber"
        :format-time="formatTime"
        :client-tool-label="clientToolLabel"
        :tool-usage-meta="toolUsageMeta"
        :tool-usage-duration="toolUsageDuration"
        :quota-overview-range-text="quotaOverviewRangeText"
        :is-token-active-now="isTokenActiveNow"
        :weekly-limit-reached="weeklyLimitReached"
        :display-status-class="displayStatusClass"
        :display-status-label="displayStatusLabel"
        :provider-label="providerLabel"
        :quota-primary-label="quotaPrimaryLabel"
        :quota-percent-value="quotaPercentValue"
        :quota-percent-text="quotaPercentText"
        :credential-label="credentialLabel"
        :api-quota-display="apiQuotaDisplay"
        :api-quota-meta="apiQuotaMeta"
        :trend-width="trendWidth"
        :request-trend-width="requestTrendWidth"
        @toggle-proxy="toggleProxy"
        @refresh="refreshAll"
        @open-settings="selectTab('settings')"
        @open-billing="openBillingView"
        @open-trends="selectTab('usage-trends')"
        @change-quota-page="changeQuotaOverviewPage"
      />
      <TokenTrendView
        v-else-if="activeTab === 'usage-trends'"
        key="usage-trends"
        :daily-usage-rows="dailyUsageRows"
        :format-number="formatNumber"
        @refresh="refreshAll"
      />
      <BillingView
        v-else-if="activeTab === 'billing'"
        key="billing"
        :entries="requestHistory"
        :daily-usage="billingUsage"
        :available-dates="billingDates"
        :selected-date="selectedBillingDate"
        :format-number="formatNumber"
        @date-change="changeBillingDate"
        @refresh="refreshBilling"
      />

      <QuotasView
        v-else-if="activeTab === 'quotas'"
        key="quotas"
        :providers="providers"
        :active-provider="activeProvider"
        :active-provider-info="activeProviderInfo"
        :active-provider-tokens="activeProviderTokens"
        :active-provider-enabled-count="activeProviderEnabledCount"
        :api-balance-summaries="activeProviderAPIBalanceSummaries"
        :refreshing-provider="refreshingProvider"
        :switching-only-token-ids="switchingOnlyTokenIds"
        :validating-ids="validatingIds"
        :provider-tokens="providerTokens"
        :credential-label="credentialLabel"
        :provider-label="providerLabel"
        @refresh="refreshAll"
        @refresh-provider-quotas="refreshProviderQuotas"
        @select-provider="selectProvider"
        @toggle-token-selected="toggleTokenSelected"
        @refresh-quota="refreshQuota"
      />

      <TokensView
        v-else-if="activeTab === 'tokens'"
        key="tokens"
        :providers="providers"
        :active-provider="activeProvider"
        :active-provider-info="activeProviderInfo"
        :active-provider-tokens="activeProviderTokens"
        :api-balance-summaries="activeProviderAPIBalanceSummaries"
        :exporting-tokens="exportingTokens"
        :exporting-codex-auth="exportingCodexAuth"
        :codex-auth-importing="codexAuthImporting"
        :batch-importing="batchImporting"
        :open-router-models="openRouterModels"
        :open-router-models-loading="openRouterModelsLoading"
        :open-router-models-error="openRouterModelsError"
        :open-router-models-fetched-at="openRouterModelsFetchedAt"
        :open-router-models-cached="openRouterModelsCached"
        :validating-ids="validatingIds"
        :refreshing-token-ids="refreshingTokenIds"
        :toggling-token-ids="togglingTokenIds"
        :provider-tokens="providerTokens"
        :credential-label="credentialLabel"
        :credential-display="credentialDisplay"
        :display-status-type="displayStatusType"
        :display-status-label="displayStatusLabel"
        :health-summary="healthSummary"
        :format-time="formatTime"
        :format-number="formatNumber"
        :format-balance="formatBalance"
        :quota-display="quotaDisplay"
        @select-provider="selectProvider"
        @export-token-backup="exportTokenBackup"
        @open-codex-auth-file-picker="openCodexAuthFilePicker"
        @import-codex-auth-files="importCodexAuthFiles"
        @export-codex-auth-backups="exportCodexAuthBackups"
        @refresh-open-router-models="refreshOpenRouterModels({ force: true })"
        @open-router-model-chat="openOpenRouterChat"
        @open-create-form="openCreateForm"
        @open-batch-import="openBatchImport"
        @verify-token="verifyToken"
        @refresh-token-auth="refreshAuthToken"
        @toggle-token-enabled="toggleTokenEnabled"
        @open-edit-form="openEditForm"
        @remove-token="removeToken"
      />

      <OpenRouterChatView
        v-else-if="activeTab === 'openrouter-chat'"
        key="openrouter-chat"
        :models="openRouterModels"
        :selected-model="selectedOpenRouterChatModel"
        :models-loading="openRouterModelsLoading"
        :models-error="openRouterModelsError"
        :open-router-tokens="openRouterTokens"
        :validating-ids="validatingIds"
        :format-time="formatTime"
        :format-number="formatNumber"
        @update:selected-model="selectOpenRouterChatModel"
        @refresh-models="refreshOpenRouterModels({ force: true })"
        @refresh-token="verifyToken"
        @open-create-key="openCreateForm('openrouter')"
      />

      <HistoryView
        v-else-if="activeTab === 'history'"
        key="history"
        :entries="requestHistory"
        :summary="requestHistorySummary"
        :providers="providers"
        :exporting="exportingHistory"
        :format-time="formatTime"
        :format-duration="formatDuration"
        :format-number="formatNumber"
        :provider-label="providerLabel"
        @refresh="refreshHistory"
        @export="exportRequestHistory"
        @diagnose="selectedHistoryEntry = $event"
      />

      <LogsView
        v-else-if="activeTab === 'logs'"
        key="logs"
        :logs="logs"
        :format-time="formatTime"
        :format-duration="formatDuration"
        @refresh="refreshRealtime"
      />

      <GatewayRoutesView
        v-else-if="activeTab === 'gateway-routes'"
        key="gateway-routes"
        :config="config"
        @persist-config="persistConfig"
      />

      <SettingsView
        v-else-if="activeTab === 'settings'"
        key="settings"
        :config="config"
        :data-directory="dataDirectory"
        :data-dir-changing="dataDirChanging"
        :auto-start-changing="autoStartChanging"
        :auto-start-enabled="autoStartEnabled"
        :task-automation-browser-profiles="taskAutomationBrowserProfiles"
        :task-automation-browser-profiles-loading="taskAutomationBrowserProfilesLoading"
        :task-automation-browser-profiles-error="taskAutomationBrowserProfilesError"
        :clearing-billing-usage="clearingBillingUsage"
        :clearing-request-history="clearingRequestHistory"
        @persist-config="persistConfig"
        @choose-data-directory="chooseDataDirectory"
        @toggle-auto-start="toggleAutoStart"
        @refresh-task-automation-browser-profiles="refreshTaskAutomationBrowserProfiles"
        @clear-billing-usage="clearBillingUsageData"
        @clear-request-history="clearRequestHistoryData"
      />

      <AboutView
        v-else-if="activeTab === 'about'"
        key="about"
        :app-info="appInfo"
        :config="config"
        :data-directory="dataDirectory"
        :proxy-status="proxyStatus"
        :auto-start-enabled="autoStartEnabled"
        :update-checking="updateChecking"
        :update-info="lastUpdateInfo"
        :update-download-status="updateDownloadStatus"
        :update-checked-at="lastUpdateCheckedAt"
        :format-time="formatTime"
        @manual-check-for-updates="manualCheckForUpdates"
        @download-update="startUpdateDownload"
        @install-update="installReadyUpdateFromUpdateSurface"
        @open-url="openExternalURL"
      />

      <QuickstartView
        v-else-if="activeTab === 'quickstart'"
        key="quickstart"
        v-model:selected-claude-models="selectedClaudeModels"
        :config="config"
        :claude-model-options="claudeModelOptions"
        :claude-model-selection-limit="claudeModelSelectionLimit"
        :selected-claude-model-labels="selectedClaudeModelLabels"
        :can-configure-claude-models="canConfigureClaudeModels"
        :is-claude-model-option-disabled="isClaudeModelOptionDisabled"
        :codex-configuring="codexConfiguring"
        :codex-restoring="codexRestoring"
        :claude-models-configuring="claudeModelsConfiguring"
        :claude-desktop-configuring="claudeDesktopConfiguring"
        :claude-desktop-restoring="claudeDesktopRestoring"
        :claude-cli-restoring="claudeCliRestoring"
        :gemini-configuring="geminiConfiguring"
        :gemini-restoring="geminiRestoring"
        :opencode-configuring="opencodeConfiguring"
        :opencode-restoring="opencodeRestoring"
        :pi-configuring="piConfiguring"
        :pi-restoring="piRestoring"
        :deep-seek-tui-configuring="deepSeekTUIConfiguring"
        :deep-seek-tui-restoring="deepSeekTUIRestoring"
        @configure-codex="configureLocalCodex"
        @restore-codex="restoreLocalCodex"
        @configure-claude-models="configureLocalClaudeModels"
        @configure-claude-desktop-models="configureLocalClaudeDesktopModels"
        @restore-claude-desktop="restoreLocalClaudeDesktop"
        @restore-claude="restoreLocalClaude"
        @configure-gemini="configureLocalGemini"
        @restore-gemini="restoreLocalGemini"
        @configure-opencode="configureLocalOpenCode"
        @restore-opencode="restoreLocalOpenCode"
        @configure-pi="configureLocalPi"
        @restore-pi="restoreLocalPi"
        @configure-deepseek-tui="configureLocalDeepSeekTUI"
        @restore-deepseek-tui="restoreLocalDeepSeekTUI"
      />

      <HelpView
        v-else-if="activeTab === 'help'"
        key="help"
        :proxy-status="proxyStatus"
        :config="config"
        :active-tokens-count="activeTokens.length"
        :token-count="tokens.length"
        :low-tokens-count="lowTokens.length"
        :invalid-tokens-count="invalidTokens.length"
        :active-requests-count="activeRequests.length"
        :today-proxy-requests="todayProxyRequests"
        :format-number="formatNumber"
        @select-tab="selectTab"
        @copy-endpoint="copyEndpointValue"
      />
      </Transition>

      <DiagnosticDrawer
        :entry="selectedHistoryEntry"
        :format-time="formatTime"
        :format-duration="formatDuration"
        :provider-label="providerLabel"
        @close="closeHistoryDiagnosis"
      />

      <Transition name="modal-pop" appear>
        <FirstUseGuideModal
          v-if="firstUseGuideVisible"
          v-model:step-index="firstUseGuideStepIndex"
          :steps="firstUseGuideSteps"
          :current-step="currentFirstUseGuideStep"
          :proxy-endpoint="proxyEndpoint"
          @close="closeFirstUseGuide"
          @previous="previousFirstUseGuideStep"
          @run-action="runFirstUseGuideAction"
          @next="nextFirstUseGuideStep"
        />
      </Transition>

      <Transition name="modal-pop" appear>
        <TokenEditorModal
          v-if="form.visible"
          :form="form"
          :providers="providers"
          :is-codex-form="isAutoNameForm"
          :placeholder="credentialPlaceholder()"
          @close="closeForm"
          @submit="submitForm"
          @provider-change="onProviderChange"
        />
      </Transition>

      <Transition name="modal-pop" appear>
        <TokenBatchImportModal
          v-if="batchImportForm.visible"
          :form="batchImportForm"
          :providers="providers"
          :placeholder="batchImportPlaceholder()"
          :importing="batchImporting"
          @close="closeBatchImport"
          @submit="submitBatchImport"
          @provider-change="onBatchImportProviderChange"
        />
      </Transition>

      <Transition name="modal-pop" appear>
        <div v-if="deleteCandidate" class="danger-confirm-backdrop" @click.self="closeDeleteConfirm">
          <section class="danger-confirm" role="dialog" aria-modal="true" aria-labelledby="delete-token-title">
            <div class="danger-confirm-mark" aria-hidden="true">
              <span></span>
            </div>
            <div class="danger-confirm-body">
              <p class="danger-confirm-kicker">危险操作</p>
              <h2 id="delete-token-title">删除这个账号？</h2>
              <p>
                删除后该账号会立即从调度池移除，已保存的凭据也会从本机账号池删除。历史请求记录不会被清空。
              </p>
              <div class="danger-confirm-card">
                <span>账号</span>
                <strong>{{ deleteCandidate.name }}</strong>
                <small>{{ providerLabel(deleteCandidate.provider) }} · {{ credentialLabel(deleteCandidate) }}</small>
              </div>
            </div>
            <div class="danger-confirm-actions">
              <button type="button" class="ghost-button" :disabled="deleteBusy" @click="closeDeleteConfirm">
                取消
              </button>
              <button type="button" class="danger-button solid" :disabled="deleteBusy" @click="confirmRemoveToken">
                {{ deleteBusy ? '删除中' : '删除账号' }}
              </button>
            </div>
          </section>
        </div>
      </Transition>

      <div v-if="loading" class="loading">加载中...</div>
    </main>
  </div>
</template>
