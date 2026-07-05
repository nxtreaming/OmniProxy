<script setup>
import GatewayRoutingSettings from './GatewayRoutingSettings.vue'

defineProps({
  config: {
    type: Object,
    required: true,
  },
})

defineEmits(['persist-config'])
</script>

<template>
  <section class="settings-panel">
    <div class="settings-page-toolbar">
      <p>先选模型，再设置主后端和备用顺序；修改完成后点击保存。</p>
      <button type="button" class="primary-button" @click="$emit('persist-config')">保存路由</button>
    </div>
    <div class="settings-stack">
      <GatewayRoutingSettings :config="config" />
    </div>
  </section>
</template>

<style src="../settings/SettingsView.css"></style>
