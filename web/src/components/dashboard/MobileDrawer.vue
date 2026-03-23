<template>
  <!-- 遮罩 -->
  <Transition name="drawer-mask">
    <div v-if="show" class="fixed inset-0 z-40 bg-[var(--modal-mask)]" @click="close" />
  </Transition>
  <!-- 抽屉面板 -->
  <Transition name="drawer-panel">
    <div v-if="show" class="fixed inset-x-0 bottom-0 z-50 max-h-[85vh] overflow-y-auto
      rounded-t-2xl glass border-t border-b-panel shadow-2xl mobile-only">
      <div class="sticky top-0 flex justify-center py-2 z-10 cursor-pointer" @click="close">
        <div class="w-10 h-1 rounded-full bg-t-faint/50"></div>
      </div>
      <slot />
    </div>
  </Transition>
</template>

<script setup lang="ts">
const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ 'update:show': [value: boolean] }>()

function close() {
  emit('update:show', false)
}
</script>
