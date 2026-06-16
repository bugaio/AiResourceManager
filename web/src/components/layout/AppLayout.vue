<script setup lang="ts">
import { useUiStore } from '@/stores/ui'
import TopBar from './TopBar.vue'
import Sidebar from './Sidebar.vue'
import ResizeDivider from './ResizeDivider.vue'

/** 应用主布局：顶栏 + 侧边栏 + 内容区 */
const uiStore = useUiStore()
</script>

<template>
  <div class="flex flex-col h-screen overflow-hidden">
    <!-- 顶部导航栏 -->
    <TopBar />
    <!-- 主体：侧边栏 + 拖拽条 + 内容区 -->
    <div class="flex flex-1 overflow-hidden">
      <Sidebar :style="{ width: uiStore.sidebarWidth + 'px' }" />
      <ResizeDivider />
      <main class="flex-1 overflow-auto">
        <slot />
      </main>
    </div>
  </div>
</template>
