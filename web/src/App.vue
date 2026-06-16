<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useUiStore } from '@/stores/ui'
import wsManager from '@/api/ws'

/** 应用根组件 - 初始化主题、WebSocket连接并提供路由容器 */
// 创建store实例会立即触发applyTheme()，确保主题在首次渲染前生效
useUiStore()

// WebSocket连接状态指示
const wsConnected = ref(false)
let statusTimer: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  // 建立WebSocket连接
  wsManager.connect()
  // 定期检查连接状态（轻量轮询）
  statusTimer = setInterval(() => {
    wsConnected.value = wsManager.getStatus() === 'connected'
  }, 2000)
  wsConnected.value = wsManager.getStatus() === 'connected'
})

onUnmounted(() => {
  wsManager.disconnect()
  if (statusTimer) clearInterval(statusTimer)
})
</script>

<template>
  <!-- WebSocket连接状态指示器 -->
  <div
    class="fixed top-2 right-2 z-[9999] flex items-center gap-1 text-xs text-gray-400 dark:text-gray-500 select-none pointer-events-none"
  >
    <span
      class="w-2 h-2 rounded-full"
      :class="wsConnected ? 'bg-green-500' : 'bg-red-400'"
    />
  </div>
  <router-view />
</template>
