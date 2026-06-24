<script setup lang="ts">
import { computed } from 'vue'
import { useUiStore, type ResourceType } from '@/stores/ui'
import { useRouter, useRoute } from 'vue-router'
import { Sunny, Moon, Setting } from '@element-plus/icons-vue'

/** 顶部导航栏：类型切换 + 主题切换 + 设置菜单 */
const uiStore = useUiStore()
const router = useRouter()
const route = useRoute()

/** Preset 路由激活态 */
const presetActive = computed(() => route.path.startsWith('/presets'))

/** 资源类型选项 */
const typeOptions: { label: string; value: ResourceType }[] = [
  { label: 'Skill', value: 'skill' },
  { label: 'SubAgent', value: 'agent' },
  { label: 'Config', value: 'config' },
  { label: 'Prompt', value: 'prompt' },
]

/** 处理设置菜单命令 */
function handleCommand(command: string) {
  if (command === 'aliases') {
    router.push('/aliases')
  } else if (command === 'data') {
    router.push('/data')
  }
}

/** 类型切换：若当前在 /presets 先跳回 /resources，再 setType */
async function handleTypeClick(type: ResourceType) {
  if (presetActive.value) {
    await router.push('/resources')
  }
  uiStore.setType(type)
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

    <!-- 中间：类型切换按钮组 + Preset 入口 -->
    <div class="flex-1 flex justify-center items-center">
      <div class="inline-flex rounded-md border border-gray-300 dark:border-gray-600 overflow-hidden">
        <button
          v-for="opt in typeOptions"
          :key="opt.value"
          class="px-4 py-1.5 text-sm font-medium transition-colors"
          :class="
            !presetActive && uiStore.currentType === opt.value
              ? 'bg-blue-500 text-white'
              : 'bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-600'
          "
          @click="handleTypeClick(opt.value)"
        >
          {{ opt.label }}
        </button>
      </div>

      <!-- Preset 入口（紧挨类型切换组右侧，间隔一点） -->
      <button
        class="ml-3 px-3 py-1.5 rounded-md text-sm font-medium transition-colors"
        :class="
          presetActive
            ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300'
            : 'text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700'
        "
        @click="router.push('/presets')"
      >
        Preset
      </button>
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
