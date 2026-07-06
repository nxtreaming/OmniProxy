<script setup>
import MaintenanceSettings from './MaintenanceSettings.vue'
import OutboundProxySettings from './OutboundProxySettings.vue'
import ProviderUrlSettings from './ProviderUrlSettings.vue'
import ServiceSchedulingSettings from './ServiceSchedulingSettings.vue'
import TaskAutomationSettings from './TaskAutomationSettings.vue'

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
  configSnapshots: {
    type: Array,
    default: () => [],
  },
  configSnapshotBusy: {
    type: String,
    default: '',
  },
  exportingConfig: {
    type: Boolean,
    default: false,
  },
  importingConfig: {
    type: Boolean,
    default: false,
  },
  taskAutomationBrowserProfiles: {
    type: Array,
    default: () => [],
  },
  taskAutomationBrowserProfilesLoading: {
    type: Boolean,
    default: false,
  },
  taskAutomationBrowserProfilesError: {
    type: String,
    default: '',
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
  'create-config-snapshot',
  'restore-config-snapshot',
  'delete-config-snapshot',
  'export-config',
  'import-config',
  'refresh-task-automation-browser-profiles',
  'clear-billing-usage',
  'clear-request-history',
])
</script>

<template>
  <section class="settings-panel">
    <div class="settings-page-toolbar">
      <p>保存后新请求会使用最新配置，端口变更需要重启代理。</p>
      <button type="button" class="primary-button" @click="$emit('persist-config')">保存设置</button>
    </div>
    <div class="settings-stack">
      <div class="settings-columns">
        <div class="settings-primary-column">
          <MaintenanceSettings
            :config="config"
            :data-directory="dataDirectory"
            :data-dir-changing="dataDirChanging"
            :auto-start-changing="autoStartChanging"
            :auto-start-enabled="autoStartEnabled"
            :config-snapshots="configSnapshots"
            :config-snapshot-busy="configSnapshotBusy"
            :exporting-config="exportingConfig"
            :importing-config="importingConfig"
            :clearing-billing-usage="clearingBillingUsage"
            :clearing-request-history="clearingRequestHistory"
            @choose-data-directory="$emit('choose-data-directory')"
            @toggle-auto-start="$emit('toggle-auto-start')"
            @create-config-snapshot="$emit('create-config-snapshot')"
            @restore-config-snapshot="$emit('restore-config-snapshot', $event)"
            @delete-config-snapshot="$emit('delete-config-snapshot', $event)"
            @export-config="$emit('export-config')"
            @import-config="$emit('import-config', $event)"
            @clear-billing-usage="$emit('clear-billing-usage')"
            @clear-request-history="$emit('clear-request-history')"
          />
          <ServiceSchedulingSettings :config="config" />
        </div>

        <div class="settings-side-column">
          <TaskAutomationSettings
            :config="config"
            :browser-profiles="taskAutomationBrowserProfiles"
            :browser-profiles-loading="taskAutomationBrowserProfilesLoading"
            :browser-profiles-error="taskAutomationBrowserProfilesError"
            @refresh-task-automation-browser-profiles="$emit('refresh-task-automation-browser-profiles', $event)"
          />
          <OutboundProxySettings :config="config" />
        </div>
      </div>

      <ProviderUrlSettings :config="config" />
    </div>
  </section>
</template>

<style src="./SettingsView.css"></style>
