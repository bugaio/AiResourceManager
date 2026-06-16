import { defineStore } from 'pinia'
import { ref } from 'vue'
import * as aliasApi from '@/api/alias'
import { useUiStore } from '@/stores/ui'
import type { PathAlias } from '@/types/alias'

/** 路径别名状态管理（按当前资源类型隔离） */
export const useAliasStore = defineStore('alias', () => {
  const aliases = ref<PathAlias[]>([])
  const loading = ref(false)

  /** 获取当前资源类型的别名列表 */
  async function fetchAliases() {
    loading.value = true
    try {
      const uiStore = useUiStore()
      aliases.value = await aliasApi.fetchAliases(uiStore.currentType)
    } catch (_e) {
      aliases.value = []
    } finally {
      loading.value = false
    }
  }

  /** 创建别名（归属当前资源类型） */
  async function createAlias(data: { name: string; path: string }) {
    const uiStore = useUiStore()
    const alias = await aliasApi.createAlias({ ...data, type: uiStore.currentType })
    aliases.value.push(alias)
    return alias
  }

  /** 更新别名 */
  async function updateAlias(id: string, data: { name?: string; path?: string }) {
    await aliasApi.updateAlias(id, data)
    const idx = aliases.value.findIndex(a => a.id === id)
    if (idx !== -1) {
      if (data.name !== undefined) aliases.value[idx].name = data.name
      if (data.path !== undefined) aliases.value[idx].path = data.path
    }
  }

  /** 删除别名 */
  async function deleteAlias(id: string) {
    await aliasApi.deleteAlias(id)
    aliases.value = aliases.value.filter(a => a.id !== id)
  }

  return {
    aliases,
    loading,
    fetchAliases,
    createAlias,
    updateAlias,
    deleteAlias,
  }
})
