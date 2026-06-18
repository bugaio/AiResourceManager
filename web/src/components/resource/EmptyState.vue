<script setup lang="ts">
import { computed } from 'vue'
import { useUiStore } from '@/stores/ui'

/** 空状态提示组件 */
const uiStore = useUiStore()

const emit = defineEmits<{
  (e: 'click'): void
}>()

/** 根据当前类型生成副标题 */
const subtitle = computed(() => {
  const map: Record<string, string> = {
    skill: '还没有创建任何 Skill 资源',
    config: '还没有创建任何 Config 资源',
    agent: '还没有创建任何 Agent 资源',
    prompt: '还没有创建任何 Prompt 资源',
  }
  return map[uiStore.currentType] || '还没有创建任何资源'
})
</script>

<template>
  <div class="flex flex-col items-center justify-center py-24 text-center">
    <!-- 空状态图标 -->
    <div class="w-16 h-16 mb-4 text-gray-300 dark:text-gray-600">
      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1">
        <path stroke-linecap="round" stroke-linejoin="round"
          d="M2 6a2 2 0 012-2h5l2 2h9a2 2 0 012 2v10a2 2 0 01-2 2H4a2 2 0 01-2-2V6z" />
      </svg>
    </div>
    <!-- 主标题 -->
    <p class="text-lg font-medium text-gray-600 dark:text-gray-300">暂无资源</p>
    <!-- 副标题 -->
    <p class="mt-1 text-sm text-gray-400 dark:text-gray-500">{{ subtitle }}</p>
    <!-- 新建按钮 -->
    <el-button type="primary" class="mt-6" @click="emit('click')">新建</el-button>
  </div>
</template>
