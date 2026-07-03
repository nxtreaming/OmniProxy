<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

const props = defineProps({
  modelValue: {
    type: [String, Number, Boolean],
    default: '',
  },
  options: {
    type: Array,
    default: () => [],
  },
  disabled: {
    type: Boolean,
    default: false,
  },
  placeholder: {
    type: String,
    default: '请选择',
  },
  ariaLabel: {
    type: String,
    default: '',
  },
})

const emit = defineEmits(['update:modelValue', 'change'])

const open = ref(false)
const root = ref(null)

const normalizedOptions = computed(() =>
  props.options.map((option) => {
    if (typeof option === 'string' || typeof option === 'number') {
      return { value: option, label: String(option), disabled: false }
    }
    return {
      value: option.value,
      label: option.label ?? String(option.value ?? ''),
      description: option.description || '',
      disabled: Boolean(option.disabled),
    }
  }),
)

const selectedOption = computed(() =>
  normalizedOptions.value.find((option) => option.value === props.modelValue),
)

const selectedLabel = computed(() => selectedOption.value?.label || props.placeholder)

function toggleOpen() {
  if (props.disabled) return
  open.value = !open.value
}

function close() {
  open.value = false
}

function chooseOption(option) {
  if (props.disabled || option.disabled) return
  emit('update:modelValue', option.value)
  emit('change', option.value)
  close()
}

function onOutsidePointerDown(event) {
  if (!open.value || !root.value || root.value.contains(event.target)) return
  close()
}

onMounted(() => {
  document.addEventListener('pointerdown', onOutsidePointerDown)
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', onOutsidePointerDown)
})
</script>

<template>
  <div ref="root" :class="['gemini-select', { open, disabled }]">
    <button
      type="button"
      class="gemini-select-trigger"
      :disabled="disabled"
      :aria-label="ariaLabel || placeholder"
      :aria-expanded="open"
      aria-haspopup="listbox"
      @click="toggleOpen"
      @keydown.escape.stop.prevent="close"
    >
      <span>{{ selectedLabel }}</span>
      <i aria-hidden="true"></i>
    </button>
    <Transition name="dropdown-fade">
      <div v-if="open" class="gemini-select-menu" role="listbox" :aria-label="ariaLabel || placeholder">
        <button
          v-for="option in normalizedOptions"
          :key="String(option.value)"
          type="button"
          :class="['gemini-select-option', { selected: option.value === modelValue, disabled: option.disabled }]"
          :disabled="option.disabled"
          role="option"
          :aria-selected="option.value === modelValue"
          @click="chooseOption(option)"
        >
          <span>
            <strong>{{ option.label }}</strong>
            <small v-if="option.description">{{ option.description }}</small>
          </span>
        </button>
      </div>
    </Transition>
  </div>
</template>
