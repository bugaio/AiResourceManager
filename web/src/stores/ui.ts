import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { ResourceType } from '@/types/resource'

export type { ResourceType }

/** 视图模式 */
export type ViewMode = 'grid' | 'list'

/** 主题类型 */
export type Theme = 'light' | 'dark'

/** 获取初始主题：优先localStorage，其次系统偏好 */
function getInitialTheme(): Theme {
  const stored = localStorage.getItem('theme')
  if (stored === 'light' || stored === 'dark') return stored
  if (window.matchMedia('(prefers-color-scheme: dark)').matches) return 'dark'
  return 'light'
}

/** 获取持久化的侧边栏宽度 */
function getStoredSidebarWidth(): number {
  const stored = localStorage.getItem('sidebarWidth')
  if (stored) {
    const num = Number(stored)
    if (num >= 180 && num <= 400) return num
  }
  return 240
}

/** UI状态管理 */
export const useUiStore = defineStore('ui', () => {
  // 当前资源类型
  const currentType = ref<ResourceType>('skill')
  // 视图模式
  const viewMode = ref<ViewMode>('grid')
  // 主题
  const theme = ref<Theme>(getInitialTheme())
  // 侧边栏宽度
  const sidebarWidth = ref<number>(getStoredSidebarWidth())

  /** 设置资源类型 */
  function setType(type: ResourceType) {
    currentType.value = type
  }

  /** 设置视图模式 */
  function setViewMode(mode: ViewMode) {
    viewMode.value = mode
  }

  /** 切换主题 */
  function toggleTheme() {
    theme.value = theme.value === 'light' ? 'dark' : 'light'
    applyTheme()
  }

  /** 设置侧边栏宽度 */
  function setSidebarWidth(width: number) {
    sidebarWidth.value = Math.max(180, Math.min(400, width))
    localStorage.setItem('sidebarWidth', String(sidebarWidth.value))
  }

  /** 应用主题到DOM */
  function applyTheme() {
    if (theme.value === 'dark') {
      document.documentElement.classList.add('dark')
    } else {
      document.documentElement.classList.remove('dark')
    }
    localStorage.setItem('theme', theme.value)
  }

  // 初始化时立即应用主题
  applyTheme()

  return {
    currentType,
    viewMode,
    theme,
    sidebarWidth,
    setType,
    setViewMode,
    toggleTheme,
    setSidebarWidth,
  }
})
