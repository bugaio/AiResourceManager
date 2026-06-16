<script setup lang="ts">
import { ref } from 'vue'
import { useUiStore } from '@/stores/ui'

/** 可拖拽分隔条：调整侧边栏宽度 */
const uiStore = useUiStore()
const isDragging = ref(false)

/** 开始拖拽 */
function onMouseDown(e: MouseEvent) {
  e.preventDefault()
  isDragging.value = true

  const startX = e.clientX
  const startWidth = uiStore.sidebarWidth

  /** 拖拽中更新宽度 */
  function onMouseMove(ev: MouseEvent) {
    const delta = ev.clientX - startX
    uiStore.setSidebarWidth(startWidth + delta)
  }

  /** 结束拖拽 */
  function onMouseUp() {
    isDragging.value = false
    document.removeEventListener('mousemove', onMouseMove)
    document.removeEventListener('mouseup', onMouseUp)
  }

  document.addEventListener('mousemove', onMouseMove)
  document.addEventListener('mouseup', onMouseUp)
}
</script>

<template>
  <div
    class="w-1 cursor-col-resize flex-shrink-0 transition-colors"
    :class="isDragging ? 'bg-blue-400' : 'bg-gray-200 dark:bg-gray-700 hover:bg-blue-400'"
    @mousedown="onMouseDown"
  />
</template>
