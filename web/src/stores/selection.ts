import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { useUiStore } from './ui'
import type { ResourceType } from '@/types/resource'

/** 选择状态管理 — 按 ResourceType 隔离三份独立选择集
 *
 * 设计原则:
 * - 三份 Set 完全独立(skill / mcp / agent),互不影响
 * - 默认所有读写以 ui.currentType 为目标,保持调用方零改动
 * - 提供 *ForType 系列方法,允许显式指定 type(给 3 个 BatchBar 用)
 */
export const useSelectionStore = defineStore('selection', () => {
  const ui = useUiStore()

  const buckets = ref<Record<ResourceType, Set<string>>>({
    skill: new Set<string>(),
    mcp: new Set<string>(),
    agent: new Set<string>(),
  })

  // ---- 当前 type 的便捷接口(给 ResourceCard / ResourcesView 全选 checkbox 用) ----

  const selectedIds = computed<Set<string>>(() => buckets.value[ui.currentType])
  const selectedCount = computed(() => selectedIds.value.size)
  const isSelectMode = computed(() => selectedCount.value > 0)

  function toggle(id: string) {
    toggleForType(ui.currentType, id)
  }
  function selectAll(ids: string[]) {
    selectAllForType(ui.currentType, ids)
  }
  function clearAll() {
    clearForType(ui.currentType)
  }
  function isSelected(id: string): boolean {
    return buckets.value[ui.currentType].has(id)
  }

  // ---- 按显式 type 寻址(给 BatchBar 实例用,每个实例只读写自己 type 的 bucket) ----

  function selectedIdsForType(t: ResourceType): Set<string> {
    return buckets.value[t]
  }
  function selectedCountForType(t: ResourceType): number {
    return buckets.value[t].size
  }
  function toggleForType(t: ResourceType, id: string) {
    const next = new Set(buckets.value[t])
    if (next.has(id)) next.delete(id)
    else next.add(id)
    buckets.value = { ...buckets.value, [t]: next }
  }
  function selectAllForType(t: ResourceType, ids: string[]) {
    buckets.value = { ...buckets.value, [t]: new Set(ids) }
  }
  function clearForType(t: ResourceType) {
    buckets.value = { ...buckets.value, [t]: new Set() }
  }

  return {
    selectedIds,
    selectedCount,
    isSelectMode,
    toggle,
    selectAll,
    clearAll,
    isSelected,
    selectedIdsForType,
    selectedCountForType,
    toggleForType,
    selectAllForType,
    clearForType,
  }
})
