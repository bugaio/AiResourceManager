<script setup lang="ts">
import { useUiStore, type ResourceType } from '@/stores/ui'
import { useRouter } from 'vue-router'
import { Sunny, Moon, Setting } from '@element-plus/icons-vue'

/** 顶部导航栏：类型切换 + 主题切换 + 设置菜单 */
const uiStore = useUiStore()
const router = useRouter()

/** 资源类型选项 */
const typeOptions: { label: string; value: ResourceType }[] = [
  { label: 'Skill', value: 'skill' },
  { label: 'MCP', value: 'mcp' },
  { label: 'SubAgent', value: 'agent' },
]

/** 处理设置菜单命令 */
function handleCommand(command: string) {
  if (command === 'aliases') {
    router.push('/aliases')
  } else if (command === 'data') {
    router.push('/data')
  }
}
</script>

<template>
  <header
    class="h-14 min-h-[56px] flex items-center px-4 border-b border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800"
  >
    <!-- 左侧：应用名称 -->
    <div class="font-semibold text-lg text-gray-800 dark:text-gray-100 whitespace-nowrap">
      AiResourceManager
    </div>

    <!-- 中间：类型切换按钮组 -->
    <div class="flex-1 flex justify-center">
      <div class="inline-flex rounded-md border border-gray-300 dark:border-gray-600 overflow-hidden">
        <button
          v-for="opt in typeOptions"
          :key="opt.value"
          class="px-4 py-1.5 text-sm font-medium transition-colors"
          :class="
            uiStore.currentType === opt.value
              ? 'bg-blue-500 text-white'
              : 'bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-600'
          "
          @click="uiStore.setType(opt.value)"
        >
          {{ opt.label }}
        </button>
      </div>
    </div>

    <!-- 右侧：主题切换 + 设置 -->
    <div class="flex items-center gap-2">
      <!-- 主题切换 -->
      <button
        class="p-2 rounded-md text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
        @click="uiStore.toggleTheme()"
      >
        <el-icon :size="18">
          <Moon v-if="uiStore.theme === 'light'" />
          <Sunny v-else />
        </el-icon>
      </button>

      <!-- 设置下拉菜单 -->
      <el-dropdown @command="handleCommand">
        <button
          class="p-2 rounded-md text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
        >
          <el-icon :size="18"><Setting /></el-icon>
        </button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item command="data">数据导入导出</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </div>
  </header>
</template>
