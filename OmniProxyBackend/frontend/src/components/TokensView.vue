<script setup>
import { computed, ref, watch } from 'vue'
import { Connection, Download, Refresh, Upload } from '@element-plus/icons-vue'

const props = defineProps({
  providers: {
    type: Array,
    required: true,
  },
  activeProvider: {
    type: String,
    required: true,
  },
  activeProviderInfo: {
    type: Object,
    required: true,
  },
  activeProviderTokens: {
    type: Array,
    required: true,
  },
  exportingTokens: {
    type: Boolean,
    required: true,
  },
  exportingCodexAuth: {
    type: Boolean,
    required: true,
  },
  codexAuthImporting: {
    type: Boolean,
    required: true,
  },
  openRouterModels: {
    type: Array,
    default: () => [],
  },
  openRouterModelsLoading: {
    type: Boolean,
    default: false,
  },
  openRouterModelsError: {
    type: String,
    default: '',
  },
  openRouterModelsFetchedAt: {
    type: String,
    default: '',
  },
  openRouterModelsCached: {
    type: Boolean,
    default: false,
  },
  validatingIds: {
    type: Object,
    required: true,
  },
  providerTokens: {
    type: Function,
    required: true,
  },
  credentialLabel: {
    type: Function,
    required: true,
  },
  credentialDisplay: {
    type: Function,
    required: true,
  },
  displayStatusType: {
    type: Function,
    required: true,
  },
  displayStatusLabel: {
    type: Function,
    required: true,
  },
  healthSummary: {
    type: Function,
    required: true,
  },
  formatTime: {
    type: Function,
    required: true,
  },
  formatNumber: {
    type: Function,
    required: true,
  },
  quotaDisplay: {
    type: Function,
    required: true,
  },
})

const emit = defineEmits([
  'select-provider',
  'export-token-backup',
  'open-codex-auth-file-picker',
  'import-codex-auth-files',
  'export-codex-auth-backups',
  'refresh-open-router-models',
  'open-router-model-chat',
  'open-create-form',
  'verify-token',
  'open-edit-form',
  'remove-token',
])

const codexAuthInput = ref(null)
const openRouterModelPage = ref(1)
const openRouterModelPageSize = 24

const openRouterModelPageCount = computed(() =>
  Math.max(1, Math.ceil(props.openRouterModels.length / openRouterModelPageSize)),
)
const openRouterModelStart = computed(() => (openRouterModelPage.value - 1) * openRouterModelPageSize)
const openRouterModelEnd = computed(() =>
  Math.min(props.openRouterModels.length, openRouterModelStart.value + openRouterModelPageSize),
)
const pagedOpenRouterModels = computed(() =>
  props.openRouterModels.slice(openRouterModelStart.value, openRouterModelEnd.value),
)

watch(
  () => props.openRouterModels.length,
  () => {
    if (openRouterModelPage.value > openRouterModelPageCount.value) {
      openRouterModelPage.value = openRouterModelPageCount.value
    }
  },
)

watch(
  () => props.activeProvider,
  () => {
    openRouterModelPage.value = 1
  },
)

function openCodexAuthFilePicker() {
  emit('open-codex-auth-file-picker')
  codexAuthInput.value?.click()
}

function changeOpenRouterModelPage(delta) {
  const nextPage = openRouterModelPage.value + delta
  openRouterModelPage.value = Math.min(openRouterModelPageCount.value, Math.max(1, nextPage))
}
</script>

<template>
  <section class="panel">
    <div class="section-heading">
      <div>
        <h2>账号管理</h2>
        <p>按厂商独立管理账号池，新添加账号默认显示在对应分组顶部</p>
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
        <p>{{ activeProviderInfo.note }} · {{ activeProviderTokens.length }} 个账号</p>
      </div>
      <div class="provider-summary-actions">
        <el-button :icon="Download" :loading="exportingTokens" @click="$emit('export-token-backup')">
          {{ exportingTokens ? '导出中' : '导出账号池' }}
        </el-button>
        <input
          ref="codexAuthInput"
          class="hidden-file-input"
          type="file"
          accept=".json,application/json"
          multiple
          @change="$emit('import-codex-auth-files', $event)"
        />
        <el-button
          v-if="activeProvider === 'openai'"
          :icon="Upload"
          :loading="codexAuthImporting"
          @click="openCodexAuthFilePicker"
        >
          {{ codexAuthImporting ? '导入中' : '导入 auth 文件' }}
        </el-button>
        <el-button
          v-if="activeProvider === 'openai'"
          :icon="Download"
          :loading="exportingCodexAuth"
          @click="$emit('export-codex-auth-backups')"
        >
          {{ exportingCodexAuth ? '导出中' : '导出 auth 文件' }}
        </el-button>
        <el-button
          v-if="activeProvider === 'openrouter'"
          :icon="Refresh"
          :loading="openRouterModelsLoading"
          @click="$emit('refresh-open-router-models')"
        >
          {{ openRouterModelsLoading ? '刷新中' : '刷新模型' }}
        </el-button>
        <el-button type="primary" :icon="Connection" @click="$emit('open-create-form', activeProvider)">
          添加 {{ activeProviderInfo.label }}
        </el-button>
      </div>
    </div>

    <div v-if="activeProvider === 'openrouter'" class="openrouter-model-panel">
      <div class="openrouter-model-head">
        <div>
          <strong>OpenRouter 模型</strong>
          <small>
            {{ openRouterModels.length ? `${openRouterModels.length} 个模型` : '添加 API Key 后可刷新模型列表' }}
            <template v-if="openRouterModelsFetchedAt">
              · {{ openRouterModelsCached ? '缓存' : '刚刚刷新' }} {{ formatTime(openRouterModelsFetchedAt) }}
            </template>
          </small>
        </div>
      </div>
      <div v-if="openRouterModelsError" class="inline-error">{{ openRouterModelsError }}</div>
      <div v-else-if="openRouterModelsLoading && !openRouterModels.length" class="openrouter-model-skeleton-list">
        <div v-for="index in 6" :key="index" class="openrouter-model-skeleton-row">
          <span></span>
          <small></small>
        </div>
      </div>
      <div v-else-if="openRouterModels.length" class="openrouter-model-list">
        <button
          v-for="model in pagedOpenRouterModels"
          :key="model.id"
          type="button"
          class="openrouter-model-row"
          @click="$emit('open-router-model-chat', model)"
        >
          <div>
            <strong>{{ model.id }}</strong>
            <small>{{ model.name || model.id }}</small>
          </div>
          <span v-if="model.contextLength">{{ formatNumber(model.contextLength) }} ctx</span>
        </button>
      </div>
      <div v-if="openRouterModels.length" class="openrouter-model-pagination">
        <span>
          {{ formatNumber(openRouterModelStart + 1) }}-{{ formatNumber(openRouterModelEnd) }}
          / {{ formatNumber(openRouterModels.length) }}
        </span>
        <div>
          <button
            type="button"
            class="ghost-button compact-button"
            :disabled="openRouterModelPage <= 1"
            @click="changeOpenRouterModelPage(-1)"
          >
            上一页
          </button>
          <strong>第 {{ openRouterModelPage }} / {{ openRouterModelPageCount }} 页</strong>
          <button
            type="button"
            class="ghost-button compact-button"
            :disabled="openRouterModelPage >= openRouterModelPageCount"
            @click="changeOpenRouterModelPage(1)"
          >
            下一页
          </button>
        </div>
      </div>
    </div>

    <div class="table-wrap">
      <table class="account-table">
        <colgroup>
          <col class="account-col-name" />
          <col class="account-col-credential-type" />
          <col class="account-col-credential" />
          <col class="account-col-quota" />
          <col class="account-col-usage" />
          <col class="account-col-status" />
          <col class="account-col-last-used" />
          <col class="account-col-actions" />
        </colgroup>
        <thead>
          <tr>
            <th>账号名称</th>
            <th>凭据类型</th>
            <th>凭据</th>
            <th>额度</th>
            <th>代理用量</th>
            <th>状态</th>
            <th>最后使用</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in activeProviderTokens" :key="item.id">
            <td>
              <strong>{{ item.name }}</strong>
              <small v-if="item.lastError">{{ item.lastError }}</small>
            </td>
            <td>{{ credentialLabel(item) }}</td>
            <td class="mono">{{ credentialDisplay(item) }}</td>
            <td>{{ quotaDisplay(item) }}</td>
            <td>
              {{ formatNumber(item.stats?.totalTokens) }}
              <small>{{ formatNumber(item.stats?.requestCount) }} 次请求</small>
            </td>
            <td>
              <el-tag :type="displayStatusType(item)" effect="light" class="status-tag">
                {{ displayStatusLabel(item) }}
              </el-tag>
              <small class="health-line">{{ healthSummary(item) }}</small>
            </td>
            <td>{{ formatTime(item.lastUsedAt) }}</td>
            <td class="actions-cell">
              <div class="row-actions">
                <el-button
                  size="small"
                  :icon="Refresh"
                  :loading="validatingIds[item.id]"
                  @click="$emit('verify-token', item)"
                >
                  {{ validatingIds[item.id] ? '验证中' : '验证' }}
                </el-button>
                <el-button size="small" @click="$emit('open-edit-form', item)">编辑</el-button>
                <el-button size="small" type="danger" plain @click="$emit('remove-token', item)">删除</el-button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
      <div v-if="!activeProviderTokens.length" class="empty">
        暂无 {{ activeProviderInfo.label }} 账号
      </div>
    </div>
  </section>
</template>
