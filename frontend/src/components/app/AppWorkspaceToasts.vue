<script setup>
defineProps({
  errorMessage: { type: String, default: '' },
  quotaRefreshProgress: { type: Object, required: true },
  successMessage: { type: String, default: '' },
})

defineEmits(['clear-error', 'clear-success'])
</script>

<template>
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
      <button type="button" aria-label="关闭错误提示" @click="$emit('clear-error')">×</button>
    </div>
    <div v-if="successMessage" key="success" class="notice" role="status">
      <span class="toast-message">{{ successMessage }}</span>
      <button type="button" aria-label="关闭成功提示" @click="$emit('clear-success')">×</button>
    </div>
  </TransitionGroup>
</template>
