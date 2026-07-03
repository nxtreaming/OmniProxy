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
      <p>客户端固定连接本地入口；这里切换后端厂商、凭据类型和默认模型。</p>
      <button type="button" class="primary-button" @click="$emit('persist-config')">保存路由</button>
    </div>
    <div class="settings-stack">
      <GatewayRoutingSettings :config="config" />
    </div>
  </section>
</template>

<style src="./SettingsView.css"></style>
