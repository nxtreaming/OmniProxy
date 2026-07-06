<script setup>
import { routeStrategyPresets } from './gatewayRoutePresets'

defineEmits(['apply'])
</script>

<template>
  <div class="gateway-strategy-list" aria-label="路由策略预设">
    <button
      v-for="strategy in routeStrategyPresets"
      :key="strategy.key"
      type="button"
      class="gateway-model-preset"
      @click="$emit('apply', strategy.key)"
    >
      <strong>{{ strategy.label }}</strong>
      <small>{{ strategy.description }}</small>
    </button>
  </div>
</template>
