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
      <p>先选后端平台，再一键套用模型；需要备用链时再进入高级微调。</p>
      <button type="button" class="primary-button" @click="$emit('persist-config')">保存路由</button>
    </div>
    <div class="settings-stack">
      <GatewayRoutingSettings :config="config" @persist-config="$emit('persist-config')" />
    </div>
  </section>
</template>

<style src="../settings/SettingsView.css"></style>
