import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { fetchResources as apiFetchResources } from '@/api/resource'
import { useUiStore } from './ui'
import { useSelectionStore } from './selection'
import type { Resource, ResourceType } from '@/types/resource'

/** 单个资源类型(skill/agent/config/prompt)各自独立的列表状态 */
interface TypeState {
  resources: Resource[]
  loading: boolean
  page: number
  total: number
  search: string
  /** 当前分组('0'=全部) */
  currentGroupId: string
}

const ALL_TYPES: ResourceType[] = ['skill', 'agent', 'config', 'prompt']

function emptyState(): TypeState {
  return {
    resources: [],
    loading: false,
    page: 1,
    total: 0,
    search: '',
    currentGroupId: '0',
  }
}

/** 资源列表状态管理 — 按 ResourceType 完全隔离
 *
 * 每个 type 拥有独立的 resources/page/search/currentGroupId,
 * 切换 type 不再相互重置。配合「每 type 一个 keep-alive 面板组件」,
 * 各模块的搜索框/分组/分页互不干扰,切换零筛选闪动。
 */
export const useResourceStore = defineStore('resource', () => {
  const ui = useUiStore()

  // 全局共享(所有 type 同一分页大小)
  const pageSize = ref(20)

  // 按 type 分桶的状态
  const states = ref<Record<ResourceType, TypeState>>({
    skill: emptyState(),
    agent: emptyState(),
    config: emptyState(),
    prompt: emptyState(),
  })

  /** 取某 type 的状态(默认当前 type) */
  function st(type?: ResourceType): TypeState {
    return states.value[type ?? ui.currentType]
  }

  // ---- 当前 type 的便捷 getter(给 GroupList / ResourceGrid / ResourceList / ResourceCard 用,零改动) ----
  const resources = computed(() => st().resources)
  const loading = computed(() => st().loading)
  const page = computed(() => st().page)
  const total = computed(() => st().total)
  const search = computed(() => st().search)
  const currentGroupId = computed(() => st().currentGroupId)

  // 各 type 独立的搜索防抖定时器
  const searchTimers: Partial<Record<ResourceType, ReturnType<typeof setTimeout>>> = {}

  /** 获取指定 type 的资源列表(默认当前 type) */
  async function fetchResources(type?: ResourceType) {
    const t = type ?? ui.currentType
    const s = states.value[t]
    s.loading = true
    try {
      const res = await apiFetchResources({
        type: t,
        page: s.page,
        page_size: pageSize.value,
        search: s.search || undefined,
        group_id: s.currentGroupId === '0' ? undefined : s.currentGroupId,
      })
      s.resources = res.list
      s.total = res.total
    } catch (_e) {
      s.resources = []
      s.total = 0
    } finally {
      s.loading = false
    }
  }

  /** 设置页码并刷新 */
  function setPage(p: number, type?: ResourceType) {
    const t = type ?? ui.currentType
    states.value[t].page = p
    fetchResources(t)
  }

  /** 设置搜索关键词(防抖300ms),按 type 独立 */
  function setSearch(keyword: string, type?: ResourceType) {
    const t = type ?? ui.currentType
    states.value[t].search = keyword
    if (searchTimers[t]) clearTimeout(searchTimers[t])
    searchTimers[t] = setTimeout(() => {
      states.value[t].page = 1
      fetchResources(t)
    }, 300)
  }

  /** 设置当前分组并刷新 */
  function setGroupId(id: string, type?: ResourceType) {
    const t = type ?? ui.currentType
    const s = states.value[t]
    s.currentGroupId = id
    s.page = 1
    fetchResources(t)
    // 切换分组时清空该 type 的选中(同 type 内,符合预期)
    useSelectionStore().clearForType(t)
  }

  /** 本地移除某资源(WS 删除事件用,不知 type 时遍历所有桶) */
  function removeResourceLocally(id?: string, uuid?: string): string | null {
    for (const t of ALL_TYPES) {
      const arr = states.value[t].resources
      const idx = arr.findIndex(
        (r) => (id && r.id === id) || (uuid && (r as any).uuid === uuid),
      )
      if (idx !== -1) {
        const name = arr[idx].name
        arr.splice(idx, 1)
        states.value[t].total = Math.max(0, states.value[t].total - 1)
        return name
      }
    }
    return null
  }

  return {
    states,
    pageSize,
    resources,
    loading,
    page,
    total,
    search,
    currentGroupId,
    fetchResources,
    setPage,
    setSearch,
    setGroupId,
    removeResourceLocally,
  }
})
