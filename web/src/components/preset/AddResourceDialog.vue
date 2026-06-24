<script setup lang="ts">
/** 为 Preset 添加资源：左右分栏 — 左侧按类型切换已有全局资源 + 本次新增的私有卡片，底部「新增私有」弹出独立对话框，右侧汇总 4 种类型的已选清单 */
import { ref, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { fetchResources } from '@/api/resource'
import {
  linkResources,
  listPresetResources,
  createPrivateResource,
  checkPresetConfigConflicts,
  type PresetConfigConflict,
} from '@/api/preset'
import type { Resource, ResourceType } from '@/types/resource'
import CreatePrivateResourceDialog from './CreatePrivateResourceDialog.vue'
import ConfigConflictDialog from './ConfigConflictDialog.vue'

const props = defineProps<{
  visible: boolean
  presetID: string
}>()

const emit = defineEmits<{
  (e: 'update:visible', val: boolean): void
  (e: 'success'): void
}>()

const dialogVisible = computed({
  get: () => props.visible,
  set: (val) => emit('update:visible', val),
})

const TABS: { label: string; value: ResourceType }[] = [
  { label: 'Skill', value: 'skill' },
  { label: 'SubAgent', value: 'agent' },
  { label: 'Config', value: 'config' },
  { label: 'Prompt', value: 'prompt' },
]

const ALL_TYPES: ResourceType[] = ['skill', 'agent', 'config', 'prompt']

/** 本次面板中新建的私有资源（临时记录，尚未落库；确定时才真正创建） */
interface DraftPrivate {
  tempId: string
  type: ResourceType
  name: string
  description: string
}

const currentType = ref<ResourceType>('skill')
/** 每个模块各自独立的搜索框内容（强制隔离，切换 tab 不共享、不清空） */
const searchInputs = ref<Record<ResourceType, string>>({
  skill: '', agent: '', config: '', prompt: '',
})
/** 按类型缓存的全局资源列表（切换 tab 不清空，右侧可汇总） */
const globalLists = ref<Record<ResourceType, Resource[]>>({
  skill: [], agent: [], config: [], prompt: [],
})
const loading = ref(false)
/** 按类型已勾选的全局资源 ID */
const checkedGlobalIds = ref<Record<ResourceType, Set<string>>>({
  skill: new Set(), agent: new Set(), config: new Set(), prompt: new Set(),
})
/** preset 已有资源 ID（不可重复选择） */
const existingResourceIds = ref<Set<string>>(new Set())
/** preset 已有资源列表（右侧"已在 Preset 中"区展示） */
const existingResources = ref<Resource[]>([])
/** 本次新建的私有资源临时记录（尚未落库，仅本弹窗生命周期内有效） */
const draftPrivates = ref<DraftPrivate[]>([])
/** 已选中的私有临时记录 tempId（保存即自动选中，可移除） */
const checkedDraftIds = ref<Set<string>>(new Set())

const submitting = ref(false)

/** Config 冲突弹窗 */
const conflictVisible = ref(false)
const conflictList = ref<PresetConfigConflict[]>([])

/** 控制「新增私有」弹窗的当前类型；null = 关闭 */
const privateFormType = ref<ResourceType | null>(null)

/** 生成本地临时 ID */
let draftSeq = 0
function nextTempId() {
  draftSeq += 1
  return `__draft_${Date.now()}_${draftSeq}`
}

/** 加载当前 type 的全局资源（按 type 独立缓存，切换 tab 保留） */
async function loadGlobal() {
  loading.value = true
  try {
    const resp = await fetchResources({
      type: currentType.value,
      page: 1,
      page_size: 1000,
      search: searchInputs.value[currentType.value].trim() || undefined,
    })
    globalLists.value[currentType.value] = resp.list || []
  } catch (e: any) {
    ElMessage.error(e?.message || '加载失败')
    globalLists.value[currentType.value] = []
  } finally {
    loading.value = false
  }
}

/** 加载 preset 已有资源（用于回显 + 禁用重复选择） */
async function loadExisting() {
  try {
    const list = await listPresetResources(props.presetID)
    const ids = new Set<string>()
    for (const r of list || []) ids.add(r.id)
    existingResourceIds.value = ids
    existingResources.value = list || []
  } catch (_e) {
    existingResourceIds.value = new Set()
    existingResources.value = []
  }
}

watch(
  () => props.visible,
  (val) => {
    if (val) {
      currentType.value = 'skill'
      searchInputs.value = { skill: '', agent: '', config: '', prompt: '' }
      for (const t of ALL_TYPES) {
        globalLists.value[t] = []
        checkedGlobalIds.value[t] = new Set()
      }
      draftPrivates.value = []
      checkedDraftIds.value = new Set()
      privateFormType.value = null
      loadGlobal()
      loadExisting()
    }
  },
)

watch(currentType, () => {
  loadGlobal()
})

let searchTimer: ReturnType<typeof setTimeout> | null = null
watch(
  () => searchInputs.value[currentType.value],
  () => {
    if (searchTimer) clearTimeout(searchTimer)
    searchTimer = setTimeout(loadGlobal, 250)
  },
)

/** 当前 type 下的私有临时记录 */
const draftPrivatesOfCurrentType = computed(() =>
  draftPrivates.value.filter((d) => d.type === currentType.value),
)

/** 当前 type 下 preset 已有的资源（含私有 + 关联，回显 + 禁选） */
const existingOfCurrentType = computed(() =>
  existingResources.value.filter((r) => r.type === currentType.value),
)

/** 全局列表里需要剔除的 ID（已在 preset 的资源会单独在置顶区显示，避免重复） */
const existingIdsOfCurrentType = computed(() => {
  const s = new Set<string>()
  for (const r of existingOfCurrentType.value) s.add(r.id)
  return s
})

/** 左侧可勾选的全局资源（排除已在 preset 中的，那些在置顶「已有」区展示） */
const globalListVisible = computed(() =>
  globalLists.value[currentType.value].filter((r) => !existingIdsOfCurrentType.value.has(r.id)),
)

/** 切换当前 type 下某个全局资源的选中（已在 preset 中的资源禁止操作） */
function toggleGlobal(r: Resource) {
  if (existingResourceIds.value.has(r.id)) return
  const ids = checkedGlobalIds.value[currentType.value]
  const next = new Set(ids)
  if (next.has(r.id)) next.delete(r.id)
  else next.add(r.id)
  checkedGlobalIds.value[currentType.value] = next
}

/** 从右侧移除已选全局（仅本次新选的才能移除） */
function removeGlobal(id: string) {
  if (existingResourceIds.value.has(id)) return
  for (const t of ALL_TYPES) {
    const ids = checkedGlobalIds.value[t]
    if (ids.has(id)) {
      const next = new Set(ids)
      next.delete(id)
      checkedGlobalIds.value[t] = next
      return
    }
  }
}

/** 切换私有临时记录的选中 */
function toggleDraft(tempId: string) {
  const next = new Set(checkedDraftIds.value)
  if (next.has(tempId)) next.delete(tempId)
  else next.add(tempId)
  checkedDraftIds.value = next
}

/** 移除私有临时记录（彻底丢弃该草稿） */
function removeDraft(tempId: string) {
  draftPrivates.value = draftPrivates.value.filter((d) => d.tempId !== tempId)
  if (checkedDraftIds.value.has(tempId)) {
    const next = new Set(checkedDraftIds.value)
    next.delete(tempId)
    checkedDraftIds.value = next
  }
}

/** 汇总 4 个 type 的本次新选全局资源（排除已在 preset 中的，用于最终 link） */
const selectedGlobalResources = computed(() => {
  const out: Resource[] = []
  const existing = existingResourceIds.value
  for (const t of ALL_TYPES) {
    const ids = checkedGlobalIds.value[t]
    if (ids.size === 0) continue
    for (const r of globalLists.value[t]) {
      if (ids.has(r.id) && !existing.has(r.id)) out.push(r)
    }
  }
  return out
})

/** 已选中的私有临时记录 */
const selectedDrafts = computed(() =>
  draftPrivates.value.filter((d) => checkedDraftIds.value.has(d.tempId)),
)

/** 总待添加数（本次新选全局 + 已选私有草稿） */
const totalSelected = computed(
  () => selectedGlobalResources.value.length + selectedDrafts.value.length,
)

/** 底部「新增私有」按钮 → 打开对应类型的新对话框 */
function handleAddPrivate() {
  privateFormType.value = currentType.value
}

/** 私有资源弹窗保存：仅加一条临时记录，并自动选中（不调后端） */
function handlePrivateSaved(payload: { type: ResourceType; name: string; description: string }) {
  const draft: DraftPrivate = {
    tempId: nextTempId(),
    type: payload.type,
    name: payload.name,
    description: payload.description,
  }
  draftPrivates.value.push(draft)
  const next = new Set(checkedDraftIds.value)
  next.add(draft.tempId)
  checkedDraftIds.value = next
  privateFormType.value = null
}

/** 提交：先真正创建已选私有资源（建文件+落库），再 link 已勾选的全局资源 */
async function handleConfirm() {
  if (totalSelected.value === 0) {
    ElMessage.warning('请至少选择或新建一个资源')
    return
  }
  submitting.value = true
  try {
    // 0. 关联前先做 config 冲突预检：待关联的全局 config 与 preset 已有 config 不得冲突
    //    （私有草稿内容为空，不可能冲突，无需检测）
    const globalConfigIds = selectedGlobalResources.value
      .filter((r) => r.type === 'config')
      .map((r) => r.id)
    if (globalConfigIds.length > 0) {
      const res = await checkPresetConfigConflicts(props.presetID, globalConfigIds)
      if (res.has_conflict) {
        conflictList.value = res.conflicts
        conflictVisible.value = true
        submitting.value = false
        return
      }
    }
    // 1. 真正创建已选中的私有资源草稿
    for (const d of selectedDrafts.value) {
      await createPrivateResource(props.presetID, {
        type: d.type,
        name: d.name,
        description: d.description || undefined,
      })
    }
    // 2. link 已勾选的全局资源
    const ids = selectedGlobalResources.value.map((r) => r.id)
    if (ids.length > 0) {
      await linkResources(props.presetID, ids)
    }
    ElMessage.success(`成功添加 ${totalSelected.value} 个资源`)
    emit('success')
    emit('update:visible', false)
  } catch (e: any) {
    ElMessage.error(e?.message || '添加失败')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <el-dialog
    v-model="dialogVisible"
    title="添加资源到 Preset"
    width="860px"
    :close-on-click-modal="false"
  >
    <div class="flex gap-4 h-[480px]">
      <!-- 左侧 -->
      <div class="flex-1 flex flex-col border border-gray-200 dark:border-gray-700 rounded">
        <!-- type tab -->
        <div class="flex border-b border-gray-200 dark:border-gray-700">
          <button
            v-for="t in TABS"
            :key="t.value"
            class="flex-1 px-3 py-2 text-sm transition-colors"
            :class="
              currentType === t.value
                ? 'bg-blue-50 text-blue-600 dark:bg-blue-900/30 dark:text-blue-300 font-medium'
                : 'text-gray-600 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-800'
            "
            @click="currentType = t.value"
          >
            {{ t.label }}
          </button>
        </div>
        <!-- 搜索 -->
        <div class="px-3 py-2 border-b border-gray-200 dark:border-gray-700">
          <el-input v-model="searchInputs[currentType]" size="small" placeholder="搜索..." clearable />
        </div>
        <!-- 资源列表 -->
        <div class="flex-1 overflow-y-auto px-2 py-1">
          <div v-if="loading" class="text-center text-xs text-gray-400 py-4">加载中...</div>
          <template v-else>
            <!-- 本次新建的私有卡片（置顶，已选中态可切换） -->
            <div
              v-for="d in draftPrivatesOfCurrentType"
              :key="d.tempId"
              class="flex items-center gap-2 px-2 py-1.5 rounded transition-colors cursor-pointer hover:bg-blue-50/60 dark:hover:bg-blue-900/20"
              @click="toggleDraft(d.tempId)"
            >
              <el-checkbox
                :model-value="checkedDraftIds.has(d.tempId)"
                @click.stop
                @change="toggleDraft(d.tempId)"
              />
              <div class="flex-1 min-w-0">
                <div class="text-sm text-gray-800 dark:text-gray-100 truncate flex items-center gap-1">
                  {{ d.name }}
                  <span class="text-[10px] px-1 rounded bg-blue-200 dark:bg-blue-800 text-blue-700 dark:text-blue-200 shrink-0">私有·待创建</span>
                </div>
                <div v-if="d.description" class="text-xs text-gray-400 truncate">{{ d.description }}</div>
              </div>
              <button
                class="text-gray-400 hover:text-red-500 text-xs shrink-0"
                title="丢弃"
                @click.stop="removeDraft(d.tempId)"
              >✕</button>
            </div>

            <!-- preset 已有资源（私有 + 关联，回显且禁选） -->
            <div
              v-for="r in existingOfCurrentType"
              :key="'exist-' + r.id"
              class="flex items-center gap-2 px-2 py-1.5 rounded bg-gray-100/60 dark:bg-gray-800/40 opacity-70 cursor-not-allowed"
            >
              <el-checkbox :model-value="true" disabled @click.stop />
              <div class="flex-1 min-w-0">
                <div class="text-sm text-gray-700 dark:text-gray-200 truncate flex items-center gap-1">
                  {{ r.name }}
                  <span
                    class="text-[10px] px-1 rounded shrink-0"
                    :class="r.owner_preset_id
                      ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300'
                      : 'bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-300'"
                  >{{ r.owner_preset_id ? '私有' : '关联' }}</span>
                </div>
                <div v-if="r.description" class="text-xs text-gray-400 truncate">{{ r.description }}</div>
              </div>
              <span
                class="text-[10px] px-1.5 py-0.5 rounded bg-gray-200 dark:bg-gray-700 text-gray-500 dark:text-gray-400 shrink-0"
              >已在 Preset 中</span>
            </div>

            <div
              v-if="globalListVisible.length === 0 && draftPrivatesOfCurrentType.length === 0 && existingOfCurrentType.length === 0"
              class="text-center text-xs text-gray-400 py-4"
            >
              没有可选资源
            </div>
            <!-- 可选的全局资源（排除已在 preset 的，避免与上方重复） -->
            <div
              v-for="r in globalListVisible"
              :key="r.id"
              class="flex items-center gap-2 px-2 py-1.5 rounded transition-colors hover:bg-gray-50 dark:hover:bg-gray-800/50 cursor-pointer"
              @click="toggleGlobal(r)"
            >
              <el-checkbox
                :model-value="checkedGlobalIds[currentType].has(r.id)"
                @click.stop
                @change="toggleGlobal(r)"
              />
              <div class="flex-1 min-w-0">
                <div class="text-sm text-gray-800 dark:text-gray-100 truncate flex items-center gap-1">
                  {{ r.name }}
                  <span class="text-[10px] px-1 rounded bg-gray-200 dark:bg-gray-700 text-gray-500 dark:text-gray-400 shrink-0">全局</span>
                </div>
                <div
                  v-if="r.description"
                  class="text-xs text-gray-400 truncate"
                >{{ r.description }}</div>
              </div>
            </div>
          </template>
        </div>
        <!-- 底部：新增私有 → 弹出独立对话框 -->
        <div class="border-t border-gray-200 dark:border-gray-700 px-3 py-2">
          <el-button size="small" type="primary" plain @click="handleAddPrivate">
            + 新增私有 {{ currentType }}
          </el-button>
        </div>
      </div>

      <!-- 右侧 已选（汇总 4 种类型） -->
      <div class="w-72 flex flex-col border border-gray-200 dark:border-gray-700 rounded">
        <div
          class="px-3 py-2 text-sm font-medium text-gray-700 dark:text-gray-200 border-b border-gray-200 dark:border-gray-700"
        >
          已选 ({{ totalSelected }})
        </div>
        <div class="flex-1 overflow-y-auto px-2 py-1 space-y-1">
          <div v-if="totalSelected === 0 && existingResources.length === 0" class="text-center text-xs text-gray-400 py-6">
            尚未选择
          </div>

          <!-- 已在 Preset 中的资源（回显，不可操作） -->
          <div v-if="existingResources.length > 0" class="mb-2">
            <div class="text-[10px] uppercase tracking-wider text-gray-400 dark:text-gray-500 px-1 py-1">
              已在 Preset 中 ({{ existingResources.length }})
            </div>
            <div
              v-for="r in existingResources"
              :key="'e-' + r.id"
              class="flex items-center gap-2 px-2 py-1 rounded bg-gray-100/60 dark:bg-gray-800/40 opacity-70"
            >
              <span
                class="text-[10px] px-1 rounded bg-gray-200 dark:bg-gray-700 text-gray-600 dark:text-gray-300"
              >{{ r.type }}</span>
              <span class="text-xs text-gray-600 dark:text-gray-400 truncate flex-1">{{ r.name }}</span>
            </div>
          </div>

          <!-- 本次新选的全局资源 -->
          <div
            v-for="r in selectedGlobalResources"
            :key="'g-' + r.id"
            class="flex items-center justify-between gap-2 px-2 py-1 rounded bg-gray-50 dark:bg-gray-800/50"
          >
            <div class="flex-1 min-w-0">
              <span
                class="text-[10px] px-1 rounded bg-gray-200 dark:bg-gray-700 text-gray-600 dark:text-gray-300 mr-1"
              >{{ r.type }}</span>
              <span class="text-xs text-gray-700 dark:text-gray-200 truncate">{{ r.name }}</span>
            </div>
            <button
              class="text-gray-400 hover:text-red-500 text-xs"
              @click="removeGlobal(r.id)"
            >✕</button>
          </div>
          <!-- 已新建的私有资源（草稿，可移除） -->
          <div
            v-for="d in selectedDrafts"
            :key="'np-' + d.tempId"
            class="flex items-center justify-between gap-2 px-2 py-1 rounded bg-blue-50 dark:bg-blue-900/20"
          >
            <div class="flex-1 min-w-0">
              <span
                class="text-[10px] px-1 rounded bg-blue-200 dark:bg-blue-800 text-blue-700 dark:text-blue-200 mr-1"
              >{{ d.type }}·私有</span>
              <span class="text-xs text-gray-700 dark:text-gray-200 truncate">{{ d.name }}</span>
            </div>
            <button
              class="text-gray-400 hover:text-red-500 text-xs"
              title="从已选移除（草稿保留在左侧）"
              @click="toggleDraft(d.tempId)"
            >✕</button>
          </div>
        </div>
      </div>
    </div>

    <!-- 「新增私有」对话框（4 种类型统一：名称 + 描述，仅暂存不落库） -->
    <CreatePrivateResourceDialog
      v-if="privateFormType !== null"
      :visible="privateFormType !== null"
      :type="privateFormType"
      @update:visible="(val) => { if (!val) privateFormType = null }"
      @save="handlePrivateSaved"
    />

    <!-- Config 冲突提示弹窗 -->
    <ConfigConflictDialog
      v-model:visible="conflictVisible"
      :conflicts="conflictList"
      hint="新增的 config 有冲突，请移除后重试"
    />

    <template #footer>
      <el-button @click="dialogVisible = false">取消</el-button>
      <el-button
        type="primary"
        :loading="submitting"
        :disabled="totalSelected === 0"
        @click="handleConfirm"
      >确定（添加 {{ totalSelected }} 个资源）</el-button>
    </template>
  </el-dialog>
</template>
