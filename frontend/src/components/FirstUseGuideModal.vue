<script setup>
defineProps({
  steps: { type: Array, required: true },
  stepIndex: { type: Number, required: true },
  currentStep: { type: Object, required: true },
  proxyEndpoint: { type: String, required: true },
})

defineEmits(['update:stepIndex', 'close', 'previous', 'run-action', 'next'])
</script>

<template>
  <div class="first-run-backdrop" @click.self="$emit('close')">
    <section class="first-run-dialog" role="dialog" aria-modal="true" aria-labelledby="first-run-title">
      <div class="first-run-head">
        <div>
          <span class="section-kicker">首次使用</span>
          <h2 id="first-run-title">3 步完成 OmniProxy 接入</h2>
          <p>这个向导只会自动显示一次；后续可以在“使用说明”和“一键配置”里查看同样的入口。</p>
        </div>
        <button type="button" class="ghost-button" @click="$emit('close')">跳过</button>
      </div>

      <div class="first-run-progress" aria-label="引导进度">
        <button
          v-for="(step, index) in steps"
          :key="step.step"
          type="button"
          :class="{ active: index === stepIndex, done: index < stepIndex }"
          @click="$emit('update:stepIndex', index)"
        >
          <span>{{ step.step }}</span>
          <strong>{{ step.title }}</strong>
        </button>
      </div>

      <div class="first-run-body">
        <div class="first-run-icon">
          <component :is="currentStep.icon" aria-hidden="true" />
        </div>
        <div>
          <span class="first-run-step">{{ currentStep.step }} / 03</span>
          <h3>{{ currentStep.title }}</h3>
          <p>{{ currentStep.description }}</p>
          <div class="first-run-endpoint">
            <span>当前本地入口</span>
            <code>{{ proxyEndpoint }}</code>
          </div>
        </div>
      </div>

      <div class="first-run-actions">
        <button type="button" class="ghost-button" @click="$emit('close')">跳过向导</button>
        <div>
          <button
            type="button"
            class="ghost-button"
            :disabled="stepIndex === 0"
            @click="$emit('previous')"
          >
            上一步
          </button>
          <button type="button" class="ghost-button" @click="$emit('run-action')">
            {{ currentStep.actionLabel }}
          </button>
          <button type="button" class="primary-button" @click="$emit('next')">
            {{ stepIndex >= steps.length - 1 ? '完成' : '下一步' }}
          </button>
        </div>
      </div>
    </section>
  </div>
</template>
