import { defineStore } from 'pinia'
import { ref } from 'vue'
import * as presetApi from '@/api/preset'
import type { Preset, PresetResource } from '@/types/preset'

/** Preset 状态管理 */
export const usePresetStore = defineStore('preset', () => {
  const presets = ref<Preset[]>([])
  const loading = ref(false)

  const currentPresetID = ref<string | null>(null)
  const currentPreset = ref<Preset | null>(null)
  const currentPresetResources = ref<PresetResource[]>([])
  const resourcesLoading = ref(false)

  /** 获取 preset 列表 */
  async function fetchPresets() {
    loading.value = true
    try {
      presets.value = (await presetApi.listPresets()) || []
      // 列表刷新后让 currentPreset 重新指向最新对象(deployments 等字段才会更新)
      if (currentPresetID.value) {
        currentPreset.value = presets.value.find((p) => p.id === currentPresetID.value) || null
      }
    } catch (_e) {
      presets.value = []
    } finally {
      loading.value = false
    }
  }

  /** 选中 preset 并加载资源 */
  async function selectPreset(id: string | null) {
    currentPresetID.value = id
    currentPreset.value = id ? presets.value.find((p) => p.id === id) || null : null
    currentPresetResources.value = []
    if (id) {
      await loadCurrentResources()
    }
  }

  /** 加载当前 preset 的资源 */
  async function loadCurrentResources() {
    if (!currentPresetID.value) return
    resourcesLoading.value = true
    try {
      currentPresetResources.value = await presetApi.listPresetResources(currentPresetID.value)
    } catch (_e) {
      currentPresetResources.value = []
    } finally {
      resourcesLoading.value = false
    }
  }

  /** 创建 preset */
  async function createPreset(data: { name: string; description?: string }) {
    const p = await presetApi.createPreset(data)
    presets.value.push(p)
    return p
  }

  /** 更新 preset */
  async function updatePreset(id: string, data: { name?: string; description?: string }) {
    await presetApi.updatePreset(id, data)
    const idx = presets.value.findIndex((p) => p.id === id)
    if (idx !== -1) {
      if (data.name !== undefined) presets.value[idx].name = data.name
      if (data.description !== undefined) presets.value[idx].description = data.description
    }
    if (currentPreset.value && currentPreset.value.id === id) {
      currentPreset.value = { ...currentPreset.value, ...data } as Preset
    }
  }

  /** 删除 preset */
  async function deletePreset(id: string) {
    await presetApi.deletePreset(id)
    presets.value = presets.value.filter((p) => p.id !== id)
    if (currentPresetID.value === id) {
      currentPresetID.value = null
      currentPreset.value = null
      currentPresetResources.value = []
    }
  }

  /** 关联资源 */
  async function linkResources(presetID: string, resourceIDs: string[]) {
    await presetApi.linkResources(presetID, resourceIDs)
    if (currentPresetID.value === presetID) await loadCurrentResources()
    await fetchPresets()
  }

  /** 取消关联 */
  async function unlinkResources(presetID: string, resourceIDs: string[]) {
    await presetApi.unlinkResources(presetID, resourceIDs)
    if (currentPresetID.value === presetID) await loadCurrentResources()
    await fetchPresets()
  }

  return {
    presets,
    loading,
    currentPresetID,
    currentPreset,
    currentPresetResources,
    resourcesLoading,
    fetchPresets,
    selectPreset,
    loadCurrentResources,
    createPreset,
    updatePreset,
    deletePreset,
    linkResources,
    unlinkResources,
  }
})
