<script setup lang="ts">
/** Preset 主区 — 4 列布局 */
import { ref, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { usePresetStore } from '@/stores/preset'
import { deletePrivateResource } from '@/api/preset'
import AddResourceDialog from '@/components/preset/AddResourceDialog.vue'
import DeployPresetDialog from '@/components/preset/DeployPresetDialog.vue'
import EditPrivateResourceDialog from '@/components/preset/EditPrivateResourceDialog.vue'
import PresetResourceCard from '@/components/preset/PresetResourceCard.vue'
import PresetForm from '@/components/preset/PresetForm.vue'
import EditorDrawer from '@/components/editor/EditorDrawer.vue'
import type { ResourceType } from '@/types/resource'
import type { PresetResource } from '@/types/preset'

const presetStore = usePresetStore()

const addResourceVisible = ref(false)
const deployDialogVisible = ref(false)
const presetFormVisible = ref(false)

// 编辑器抽屉
const editorVisible = ref(false)
const editorResourceId = ref('')
const editorReadonly = ref(false)

// 编辑私有资源信息（名称 + 描述）
const editInfoVisible = ref(false)
const editingResource = ref<PresetResource | null>(null)

const TYPE_COLUMNS: { type: ResourceType; label: string }[] = [
  { type: 'skill', label: 'Skill' },
  { type: 'agent', label: 'SubAgent' },
  { type: 'config', label: 'Config' },
  { type: 'prompt', label: 'Prompt' },
]

/** 按类型分组的资源 */
const groupedResources = computed(() => {
  const map: Record<ResourceType, PresetResource[]> = {
    skill: [],
    agent: [],
    config: [],
    prompt: [],
  }
  for (const r of presetStore.currentPresetResources) {
    map[r.type].push(r)
  }
  return map
})

/** 列头统计：私有 N · 关联 M */
function columnStats(type: ResourceType) {
  const items = groupedResources.value[type]
  const priv = items.filter((r) => !!r.owner_preset_id).length
  return { total: items.length, private: priv, linked: items.length - priv }
}

function handleAddResource() {
  if (!presetStore.currentPresetID) return
  addResourceVisible.value = true
}

function handleDeploy() {
  if (!presetStore.currentPresetID) return
  if (presetStore.currentPresetResources.length === 0) {
    ElMessage.warning('该 Preset 没有资源，无法部署')
    return
  }
  deployDialogVisible.value = true
}

function handleEditPreset() {
  presetFormVisible.value = true
}

function handleAddSuccess() {
  presetStore.loadCurrentResources()
  presetStore.fetchPresets()
}

/** 部署成功后刷新 preset(含 deployments)，使下次部署弹窗过滤掉已部署路径组 */
function handleDeploySuccess() {
  presetStore.fetchPresets()
}

function handleEditContent(r: PresetResource) {
  editorResourceId.value = r.id
  editorReadonly.value = false
  editorVisible.value = true
}

function handleViewContent(r: PresetResource) {
  editorResourceId.value = r.id
  editorReadonly.value = true
  editorVisible.value = true
}

async function handleDeletePrivate(r: PresetResource) {
  try {
    await ElMessageBox.confirm(`确定要删除私有资源「${r.name}」吗？`, '确认删除', {
      confirmButtonText: '确定删除',
      cancelButtonText: '取消',
      type: 'warning',
    })
  } catch {
    return
  }
  try {
    await deletePrivateResource(presetStore.currentPresetID!, r.id)
    ElMessage.success('已删除')
    presetStore.loadCurrentResources()
    presetStore.fetchPresets()
  } catch (e: any) {
    ElMessage.error(e?.message || '删除失败')
  }
}

async function handleUnlink(r: PresetResource) {
  if (!presetStore.currentPresetID) return
  try {
    await ElMessageBox.confirm(
      `确定要从 Preset 中取消关联「${r.name}」吗？`,
      '取消关联',
      { confirmButtonText: '取消关联', cancelButtonText: '取消', type: 'warning' },
    )
  } catch {
    return
  }
  try {
    await presetStore.unlinkResources(presetStore.currentPresetID, [r.id])
    ElMessage.success('已取消关联')
  } catch (e: any) {
    ElMessage.error(e?.message || '取消关联失败')
  }
}

function handleEditResourceInfo(r: PresetResource) {
  editingResource.value = r
  editInfoVisible.value = true
}
</script>

<template>
  <div class="flex flex-col h-full bg-neutral-50 dark:bg-gray-900">
    <!-- 未选中 preset -->
    <div
      v-if="!presetStore.currentPreset"
      class="flex-1 flex items-center justify-center text-gray-400 dark:text-gray-500 text-sm"
    >
      请在左侧选择一个 Preset
    </div>

    <template v-else>
      <!-- 顶部工具栏 -->
      <div
        class="flex items-center justify-between gap-4 px-6 py-4 border-b border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800"
      >
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-2">
            <h2 class="text-lg font-semibold text-gray-800 dark:text-gray-100 truncate">
              {{ presetStore.currentPreset.name }}
            </h2>
            <el-button text size="small" @click="handleEditPreset">编辑</el-button>
          </div>
          <div
            v-if="presetStore.currentPreset.description"
            class="text-xs text-gray-500 dark:text-gray-400 mt-0.5 truncate"
          >
            {{ presetStore.currentPreset.description }}
          </div>
        </div>
        <div class="flex items-center gap-2 flex-shrink-0">
          <el-button type="primary" @click="handleAddResource">+ 添加资源</el-button>
          <el-button type="success" @click="handleDeploy">部署 Preset</el-button>
        </div>
      </div>

      <!-- 内容区：4 列 -->
      <div class="flex-1 overflow-hidden p-4">
        <div v-if="presetStore.resourcesLoading" class="text-center text-sm text-gray-400 py-10">
          加载中...
        </div>
        <div
          v-else-if="presetStore.currentPresetResources.length === 0"
          class="text-center text-sm text-gray-400 py-10"
        >
          该 Preset 还没有资源，点击右上角添加
        </div>
        <div v-else class="grid grid-cols-4 gap-3 h-full">
          <div
            v-for="col in TYPE_COLUMNS"
            :key="col.type"
            class="flex flex-col min-h-0 bg-white dark:bg-gray-800/50 rounded-lg border border-gray-200 dark:border-gray-700"
          >
            <!-- 列头 -->
            <div
              class="px-3 py-2 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between"
            >
              <span class="text-sm font-medium text-gray-700 dark:text-gray-200">{{ col.label }}</span>
              <span class="text-xs text-gray-400">
                私有 {{ columnStats(col.type).private }} · 关联 {{ columnStats(col.type).linked }}
              </span>
            </div>
            <!-- 列内容 -->
            <div class="flex-1 overflow-y-auto p-2 space-y-2 min-h-0">
              <div
                v-if="groupedResources[col.type].length === 0"
                class="text-xs text-gray-400 text-center py-4"
              >
                空
              </div>
              <PresetResourceCard
                v-for="r in groupedResources[col.type]"
                :key="r.id"
                :resource="r"
                @edit="handleEditResourceInfo"
                @edit-content="handleEditContent"
                @view-content="handleViewContent"
                @delete="handleDeletePrivate"
                @unlink="handleUnlink"
              />
            </div>
          </div>
        </div>
      </div>

      <!-- 添加资源对话框 -->
      <AddResourceDialog
        v-if="presetStore.currentPresetID"
        v-model:visible="addResourceVisible"
        :preset-i-d="presetStore.currentPresetID"
        @success="handleAddSuccess"
      />

      <!-- 部署对话框 -->
      <DeployPresetDialog
        v-if="presetStore.currentPresetID"
        v-model:visible="deployDialogVisible"
        :preset-i-d="presetStore.currentPresetID"
        :preset-name="presetStore.currentPreset.name"
        :resources="presetStore.currentPresetResources"
        :deployments="presetStore.currentPreset.deployments || []"
        @success="handleDeploySuccess"
      />

      <!-- Preset 编辑表单 -->
      <PresetForm
        v-model:visible="presetFormVisible"
        mode="edit"
        :preset="presetStore.currentPreset"
      />

      <!-- 编辑器抽屉 -->
      <EditorDrawer
        v-model:visible="editorVisible"
        :resource-id="editorResourceId"
        :readonly="editorReadonly"
        @saved="handleAddSuccess"
      />

      <!-- 编辑私有资源信息（名称 + 描述） -->
      <EditPrivateResourceDialog
        v-model:visible="editInfoVisible"
        :resource="editingResource"
        @success="handleAddSuccess"
      />
    </template>
  </div>
</template>
