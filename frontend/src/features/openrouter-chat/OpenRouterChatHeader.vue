<script setup>
import { Delete, Refresh } from '@element-plus/icons-vue'

defineProps({
  conversationUsageLine: { type: String, required: true },
  formatNumber: { type: Function, required: true },
  messagesCount: { type: Number, default: 0 },
  modelPriceLine: { type: String, required: true },
  modelsLoading: { type: Boolean, default: false },
  openRouterQuotaLimitText: { type: String, required: true },
  openRouterQuotaLoading: { type: Boolean, default: false },
  openRouterQuotaMeta: { type: String, required: true },
  openRouterQuotaRemainingText: { type: String, required: true },
  openRouterQuotaToken: { type: Object, default: null },
  openRouterQuotaUsedText: { type: String, required: true },
  selectedModelId: { type: String, default: '' },
  selectedModelInfo: { type: Object, default: null },
  sending: { type: Boolean, default: false },
  typing: { type: Boolean, default: false },
})

defineEmits(['refresh-models', 'refresh-quota', 'reset-conversation'])
</script>

<template>
  <div class="openrouter-chat-toolbar">
    <div class="openrouter-chat-current">
      <strong>{{ selectedModelInfo?.name || selectedModelId || '选择模型' }}</strong>
      <span>{{ selectedModelId || '未选择模型' }}</span>
    </div>
    <div class="openrouter-chat-metrics">
      <span>{{ selectedModelInfo?.contextLength ? `${formatNumber(selectedModelInfo.contextLength)} ctx` : 'ctx -' }}</span>
      <span>{{ modelPriceLine }}</span>
      <span>{{ conversationUsageLine }}</span>
    </div>
    <div class="openrouter-chat-actions">
      <el-button :icon="Refresh" :loading="modelsLoading" @click="$emit('refresh-models')">
        {{ modelsLoading ? '刷新中' : '刷新' }}
      </el-button>
      <el-button :icon="Refresh" :loading="openRouterQuotaLoading" @click="$emit('refresh-quota')">
        {{ openRouterQuotaToken ? '额度' : '添加 Key' }}
      </el-button>
      <el-button :icon="Delete" :disabled="!messagesCount || sending || typing" @click="$emit('reset-conversation')">
        清空
      </el-button>
    </div>
  </div>

  <div class="openrouter-chat-quota-strip">
    <div class="openrouter-chat-quota-title">
      <span>OpenRouter 额度</span>
      <strong>{{ openRouterQuotaToken?.name || '未配置 API Key' }}</strong>
      <small>{{ openRouterQuotaMeta }}</small>
    </div>
    <div>
      <span>剩余</span>
      <strong>{{ openRouterQuotaRemainingText }}</strong>
    </div>
    <div>
      <span>已用</span>
      <strong>{{ openRouterQuotaUsedText }}</strong>
    </div>
    <div>
      <span>上限</span>
      <strong>{{ openRouterQuotaLimitText }}</strong>
    </div>
  </div>
</template>
