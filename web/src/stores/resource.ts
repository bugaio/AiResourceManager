import { defineStore } from 'pinia'
import { ref, watch } from 'vue'
import { fetchResources as apiFetchResources } from '@/api/resource'
import { useUiStore } from './ui'
import { useSelectionStore } from './selection'
import type { Resource } from '@/types/resource'

/** 资源列表状态管理 */
export const useResourceStore = defineStore('resource', () => {
  const ui = useUiStore()

  // 资源列表
  const resources = ref<Resource[]>([])
  // 加载状态
  const loading = ref(false)
  // 分页
  const page = ref(1)
  const pageSize = ref(20)
  const total = ref(0)
  // 搜索关键词
  const search = ref('')
  // 当前分组ID（'0'或空表示全部）
  const currentGroupId = ref<string>('0')

  // 防抖定时器
  let searchTimer: ReturnType<typeof setTimeout> | null = null

  /** 获取资源列表 */
  async function fetchResources() {
    loading.value = true
    try {
      const params = {
        type: ui.currentType,
        page: page.value,
        page_size: pageSize.value,
        search: search.value || undefined,
        group_id: currentGroupId.value === '0' ? undefined : currentGroupId.value,
      }
      const res = await apiFetchResources(params)
      resources.value = res.list
      total.value = res.total
    } catch (_e) {
      resources.value = []
      total.value = 0
    } finally {
      loading.value = false
    }
  }

  /** 设置页码并刷新 */
  function setPage(p: number) {
    page.value = p
    fetchResources()
  }

  /** 设置搜索关键词（防抖300ms） */
  function setSearch(keyword: string) {
    search.value = keyword
    if (searchTimer) clearTimeout(searchTimer)
    searchTimer = setTimeout(() => {
      page.value = 1
      fetchResources()
    }, 300)
  }

  /** 设置当前分组并刷新 */
  function setGroupId(id: string) {
    currentGroupId.value = id
    page.value = 1
    fetchResources()
    // 切换分组时清空选中状态
    useSelectionStore().clearAll()
  }

  /** 重置分页到首页 */
  function resetPagination() {
    page.value = 1
    search.value = ''
    currentGroupId.value = '0'
    total.value = 0
  }

  // 监听资源类型切换 → 重置并重新获取
  watch(() => ui.currentType, () => {
    resetPagination()
    fetchResources()
  })

  return {
    resources,
    loading,
    page,
    pageSize,
    total,
    search,
    currentGroupId,
    fetchResources,
    setPage,
    setSearch,
    setGroupId,
    resetPagination,
  }
})
