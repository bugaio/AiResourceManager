import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

/** 选择状态管理 */
export const useSelectionStore = defineStore('selection', () => {
  // 已选择的资源ID集合
  const selectedIds = ref<Set<string>>(new Set())
  // 是否处于选择模式
  const isSelectMode = ref(false)

  /** 已选数量 */
  const selectedCount = computed(() => selectedIds.value.size)

  /** 切换选中状态 */
  function toggle(id: string) {
    const next = new Set(selectedIds.value)
    if (next.has(id)) {
      next.delete(id)
    } else {
      next.add(id)
    }
    selectedIds.value = next
  }

  /** 全选 */
  function selectAll(ids: string[]) {
    selectedIds.value = new Set(ids)
  }

  /** 清空选择 */
  function clearAll() {
    selectedIds.value = new Set()
    isSelectMode.value = false
  }

  /** 判断是否选中 */
  function isSelected(id: string): boolean {
    return selectedIds.value.has(id)
  }

  return {
    selectedIds,
    isSelectMode,
    selectedCount,
    toggle,
    selectAll,
    clearAll,
    isSelected,
  }
})
