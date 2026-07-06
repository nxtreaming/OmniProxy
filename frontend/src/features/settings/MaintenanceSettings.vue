<script setup>
import { ref } from 'vue'

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
  clearingBillingUsage: {
    type: Boolean,
    required: true,
  },
  clearingRequestHistory: {
    type: Boolean,
    required: true,
  },
})

const emit = defineEmits([
  'choose-data-directory',
  'toggle-auto-start',
  'create-config-snapshot',
  'restore-config-snapshot',
  'delete-config-snapshot',
  'export-config',
  'import-config',
  'clear-billing-usage',
  'clear-request-history',
])

const importInput = ref(null)

function requestImportConfig() {
  if (typeof window !== 'undefined' && window.go?.main?.DesktopApp) {
    emit('import-config', null)
    return
  }
  importInput.value?.click()
}

function onImportConfigFile(event) {
  const file = event.target.files?.[0]
  event.target.value = ''
  if (file) {
    emit('import-config', file)
  }
}
</script>

<template>
  <section class="settings-section settings-maintenance-section">
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
          <strong>系统托盘 / 菜单栏与开机自启</strong>
          <small>关闭主窗口时保留后台入口，可从托盘或 macOS 菜单栏启动/停止代理、查看端口、打开主界面或退出。</small>
        </div>
        <button type="button" class="ghost-button" :disabled="autoStartChanging" @click="$emit('toggle-auto-start')">
          {{ autoStartChanging ? '更新中' : autoStartEnabled ? '关闭自启' : '开启自启' }}
        </button>
      </div>
      <label class="data-directory-row update-channel-row">
        <div>
          <span>软件更新</span>
          <strong>检测 Beta 版本更新</strong>
          <small>开启后会把 GitHub Pre-release/Beta 纳入更新提醒；关闭时只提醒正式版。</small>
        </div>
        <span class="toggle-field compact-toggle-field">
          <input v-model="config.checkBetaUpdates" class="toggle-input" type="checkbox" aria-label="检测 Beta 版本更新" />
          <span class="toggle-switch" aria-hidden="true">
            <span class="toggle-thumb"></span>
          </span>
        </span>
      </label>
      <div class="data-directory-row maintenance-row">
        <div>
          <span>配置备份</span>
          <strong>快照、导出与导入</strong>
          <small>只处理应用设置，不包含账号池、auth.json、API Key 或请求日志。</small>
          <div class="snapshot-list">
            <div
              v-for="snapshot in configSnapshots"
              :key="snapshot.id"
              class="snapshot-pill"
            >
              <button
                type="button"
                :disabled="configSnapshotBusy === snapshot.id"
                @click="$emit('restore-config-snapshot', snapshot.id)"
              >
                <span>{{ snapshot.name || snapshot.id }}</span>
                <small>{{ snapshot.createdAt || '未知时间' }}</small>
              </button>
              <button
                type="button"
                class="snapshot-delete-button"
                :disabled="configSnapshotBusy === snapshot.id"
                @click="$emit('delete-config-snapshot', snapshot.id)"
              >
                删除
              </button>
            </div>
            <small v-if="!configSnapshots.length">暂无配置快照</small>
          </div>
        </div>
        <div class="maintenance-actions">
          <button
            type="button"
            class="ghost-button"
            :disabled="configSnapshotBusy === 'create'"
            @click="$emit('create-config-snapshot')"
          >
            {{ configSnapshotBusy === 'create' ? '创建中' : '创建快照' }}
          </button>
          <button type="button" class="ghost-button" :disabled="exportingConfig" @click="$emit('export-config')">
            {{ exportingConfig ? '导出中' : '导出配置' }}
          </button>
          <button type="button" class="ghost-button" :disabled="importingConfig" @click="requestImportConfig">
            {{ importingConfig ? '导入中' : '导入配置' }}
          </button>
          <input ref="importInput" class="hidden-file-input" type="file" accept="application/json,.json" @change="onImportConfigFile" />
        </div>
      </div>
      <div class="data-directory-row maintenance-row">
        <div>
          <span>告警阈值</span>
          <strong>账号健康与长请求提醒</strong>
          <small>总览会按这些阈值标出关注账号、高风险账号和长时间占用的请求。</small>
          <div class="threshold-grid">
            <label class="inline-number-field">
              <span>关注分数低于</span>
              <input v-model="config.healthWatchThreshold" type="number" min="2" max="100" />
            </label>
            <label class="inline-number-field">
              <span>高风险低于</span>
              <input v-model="config.healthRiskThreshold" type="number" min="1" max="99" />
            </label>
            <label class="inline-number-field">
              <span>长请求秒数</span>
              <input v-model="config.longRequestAlertSeconds" type="number" min="1" max="3600" />
            </label>
          </div>
        </div>
      </div>
      <div class="data-directory-row maintenance-row">
        <div>
          <span>账单与请求历史</span>
          <strong>每日汇总保留 {{ config.historyRetentionDays || 14 }} 天</strong>
          <small>默认保留 14 天；每日汇总记录请求数和最终 Token 用量，完整请求明细可单独清空。</small>
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
</template>
