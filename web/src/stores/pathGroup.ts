import { defineStore } from 'pinia'
import { ref } from 'vue'
import * as pathGroupApi from '@/api/pathGroup'
import type { PathGroup, CreatePathGroupReq, UpdatePathGroupReq } from '@/types/pathGroup'

/** 路径组状态管理 */
export const usePathGroupStore = defineStore('pathGroup', () => {
  const pathGroups = ref<PathGroup[]>([])
  const loading = ref(false)

  async function fetchPathGroups() {
    loading.value = true
    try {
      pathGroups.value = (await pathGroupApi.listPathGroups()) || []
    } catch (_e) {
      pathGroups.value = []
    } finally {
      loading.value = false
    }
  }

  async function createPathGroup(data: CreatePathGroupReq) {
    const g = await pathGroupApi.createPathGroup(data)
    pathGroups.value.push(g)
    return g
  }

  async function updatePathGroup(id: string, data: UpdatePathGroupReq) {
    await pathGroupApi.updatePathGroup(id, data)
    const idx = pathGroups.value.findIndex((g) => g.id === id)
    if (idx !== -1) {
      pathGroups.value[idx] = { ...pathGroups.value[idx], ...data } as PathGroup
    }
  }

  async function deletePathGroup(id: string) {
    await pathGroupApi.deletePathGroup(id)
    pathGroups.value = pathGroups.value.filter((g) => g.id !== id)
  }

  return {
    pathGroups,
    loading,
    fetchPathGroups,
    createPathGroup,
    updatePathGroup,
    deletePathGroup,
  }
})
