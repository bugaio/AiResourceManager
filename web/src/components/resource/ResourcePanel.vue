<script setup lang="ts">
/** 单个资源类型的完整面板：搜索 + 新建 + 导入 + 列表 + 分页 + 批量栏
 *
 * 每个 ResourceType(skill/agent/config/prompt) 各渲染一个独立实例,
 * 由 ResourcesView 用 <keep-alive> 缓存。搜索框是本组件内的本地 ref,
 * 物理隔离 —— 切换模块时各自的搜索内容原地保留,不会触发其他模块筛选。
 */
import { ref, computed, watch, onActivated } from 'vue'
import { ElMessageBox, ElMessage } from 'element-plus'
import ResourceGrid from '@/components/resource/ResourceGrid.vue'
import ResourceList from '@/components/resource/ResourceList.vue'
import EmptyState from '@/components/resource/EmptyState.vue'
import BatchBar from '@/components/common/BatchBar.vue'
import ResourceForm from '@/components/resource/ResourceForm.vue'
import ImportSkillDialog from '@/components/resource/ImportSkillDialog.vue'
import ImportAgentDialog from '@/components/resource/ImportAgentDialog.vue'
import EditorDrawer from '@/components/editor/EditorDrawer.vue'
import DeployDialog from '@/components/deploy/DeployDialog.vue'
import UnlinkAndDeleteDialog from '@/components/preset/UnlinkAndDeleteDialog.vue'
import ResourceLinksDialog from '@/components/resource/ResourceLinksDialog.vue'
import { useUiStore } from '@/stores/ui'
import { useResourceStore } from '@/stores/resource'
import { useDeployStore } from '@/stores/deploy'
import { useSelectionStore } from '@/stores/selection'
import { usePresetStore } from '@/stores/preset'
import { usePathGroupStore } from '@/stores/pathGroup'
import { deleteResource } from '@/api/resource'
import { useGroupStore } from '@/stores/group'
import type { Resource, ResourceType } from '@/types/resource'
import type { PresetLinkInfo } from '@/types/preset'

const props = defineProps<{ type: ResourceType }>()

const uiStore = useUiStore()
const resourceStore = useResourceStore()
const deployStore = useDeployStore()
const selectionStore = useSelectionStore()
const groupStore = useGroupStore()
const presetStore = usePresetStore()
const pathGroupStore = usePathGroupStore()

/** 本面板自己 type 的状态桶(直接寻址,不依赖全局 currentType) */
const state = computed(() => resourceStore.states[props.type])

// 本面板独立的搜索框(物理隔离,keep-alive 缓存后原地保留)
const searchInput = ref('')

// 资源表单状态
const formVisible = ref(false)
const formMode = ref<'create' | 'edit'>('create')
const formResource = ref<Resource | undefined>()

// 编辑器抽屉状态
const editorVisible = ref(false)
const editorResourceId = ref('')

// 部署对话框状态
const deployDialogVisible = ref(false)
const deployResources = ref<Resource[]>([])
const deployGroupId = ref<string | undefined>(undefined)

// 取消关联并删除对话框
const unlinkDialogVisible = ref(false)
const unlinkTarget = ref<Resource | null>(null)
const unlinkPresets = ref<PresetLinkInfo[]>([])

// 查看关联对话框
const linksDialogVisible = ref(false)
const linksResource = ref<Resource | null>(null)

// 导入相关(仅 skill / agent)
const importInputRef = ref<HTMLInputElement | null>(null)
const importSkillVisible = ref(false)
const importAgentVisible = ref(false)
const importFiles = ref<File[]>([])
const importRootName = ref('')

const canImport = computed(() => props.type === 'skill' || props.type === 'agent')

/** 触发系统目录选择 */
function handleImportClick() {
  if (importInputRef.value) {
    importInputRef.value.value = ''
    importInputRef.value.click()
  }
}

/** 用户选完目录: 收集 File[] 后打开对应导入弹窗 */
function handleImportFilesChange(e: Event) {
  const input = e.target as HTMLInputElement
  const fl = input.files
  if (!fl || fl.length === 0) return
  const arr = Array.from(fl)
  const first = arr[0].webkitRelativePath.split('/')[0] || ''
  importFiles.value = arr
  importRootName.value = first
  if (props.type === 'skill') importSkillVisible.value = true
  else if (props.type === 'agent') importAgentVisible.value = true
}

/** 导入成功后刷新列表 */
function handleImportSuccess() {
  resourceStore.fetchResources(props.type)
  groupStore.fetchGroups()
}

/** 全选状态 */
const isAllSelected = computed(() => {
  const list = state.value.resources
  return list.length > 0 && list.every((r) => selectionStore.selectedIdsForType(props.type).has(r.id))
})

/** 半选状态 */
const isIndeterminate = computed(() => {
  const list = state.value.resources
  const selected = list.filter((r) => selectionStore.selectedIdsForType(props.type).has(r.id)).length
  return selected > 0 && selected < list.length
})

/** 全选/取消全选 */
function handleSelectAll(val: boolean | string | number) {
  if (val) {
    selectionStore.selectAllForType(props.type, state.value.resources.map((r) => r.id))
  } else {
    selectionStore.clearForType(props.type)
  }
}

// 搜索 → 写入本 type 的 store(store 内部按 type 独立防抖)
watch(searchInput, (val) => {
  resourceStore.setSearch(val, props.type)
})

/** 分页变化 */
function handlePageChange(p: number) {
  resourceStore.setPage(p, props.type)
}

/** 打开新建表单 */
function handleCreate() {
  formMode.value = 'create'
  formResource.value = undefined
  formVisible.value = true
}

/** 编辑资源信息 */
function handleEdit(resource: Resource) {
  formMode.value = 'edit'
  formResource.value = resource
  formVisible.value = true
}

/** 打开编辑内容抽屉 */
function handleEditContent(resource: Resource) {
  editorResourceId.value = resource.id
  editorVisible.value = true
}

/** 查看该资源关联的 Preset（需 preset 列表含 deployments + 路径组数据） */
async function handleViewLinks(resource: Resource) {
  linksResource.value = resource
  linksDialogVisible.value = true
  // 刷新数据：资源列表(拿最新 preset_links) + preset(含 deployments) + 路径组
  await Promise.all([
    resourceStore.fetchResources(props.type),
    presetStore.fetchPresets(),
    pathGroupStore.fetchPathGroups(),
  ])
  // 列表刷新后对象引用已变，用最新的资源对象重新指向(preset_links 才是最新的)
  const fresh = resourceStore.states[props.type].resources.find((r) => r.id === resource.id)
  if (fresh) linksResource.value = fresh
}

/** 取消关联后刷新资源列表 + preset，使查看关联弹窗内容同步更新(已取消的那条消失) */
async function handleUnlinked() {
  const id = linksResource.value?.id
  await Promise.all([resourceStore.fetchResources(props.type), presetStore.fetchPresets()])
  if (id) {
    const fresh = resourceStore.states[props.type].resources.find((r) => r.id === id)
    linksResource.value = fresh || null
  }
}

/** 表单/编辑器保存成功后刷新列表 */
function handleRefresh() {
  resourceStore.fetchResources(props.type)
}

/** 删除资源(带确认逻辑) */
async function handleDelete(resource: Resource) {
  if (resource.preset_links && resource.preset_links.length > 0) {
    unlinkTarget.value = resource
    unlinkPresets.value = resource.preset_links
    unlinkDialogVisible.value = true
    // 弹窗需展示每个 preset 的部署路径组，确保 preset(含 deployments)/路径组数据最新
    await Promise.all([presetStore.fetchPresets(), pathGroupStore.fetchPathGroups()])
    return
  }
  try {
    await ElMessageBox.confirm(`确定要删除资源「${resource.name}」吗？`, '确认删除', {
      confirmButtonText: '确定删除',
      cancelButtonText: '取消',
      type: 'warning',
    })
  } catch {
    return
  }
  await doDelete(resource, {})
}

/** 执行删除：捕获 1004(部署)/1005(被锁) */
async function doDelete(resource: Resource, opts: { confirm?: boolean; unlink?: boolean }) {
  try {
    await deleteResource(resource.id, opts)
    ElMessage.success('删除成功')
    resourceStore.fetchResources(props.type)
    deployStore.fetchTargets()
    groupStore.fetchGroups()
  } catch (e: any) {
    if (e?.code === 1005) {
      unlinkTarget.value = resource
      unlinkPresets.value = e?.data?.presets || []
      unlinkDialogVisible.value = true
      await Promise.all([presetStore.fetchPresets(), pathGroupStore.fetchPathGroups()])
      return
    }
    if (e?.message?.includes('部署') || e?.code === 1004) {
      try {
        await ElMessageBox.confirm(e.message || '该资源已部署，确定删除？', '确认删除', {
          confirmButtonText: '确定删除',
          cancelButtonText: '取消',
          type: 'warning',
        })
        await doDelete(resource, { ...opts, confirm: true })
      } catch {
        // 用户取消
      }
    } else {
      ElMessage.error(e?.message || '删除失败')
    }
  }
}

/** 取消关联并删除回调 */
async function handleUnlinkConfirm() {
  if (!unlinkTarget.value) return
  await doDelete(unlinkTarget.value, { unlink: true, confirm: true })
  unlinkDialogVisible.value = false
  unlinkTarget.value = null
  unlinkPresets.value = []
}

/** 从分组移除资源 */
async function handleRemoveFromGroup(resource: Resource) {
  try {
    await groupStore.removeResource(state.value.currentGroupId, resource.id)
    ElMessage.success('已从分组移除')
    resourceStore.fetchResources(props.type)
  } catch (e: any) {
    ElMessage.error(e?.message || '移除失败')
  }
}

/** 单个资源部署 */
function handleDeploy(resource: Resource) {
  deployResources.value = [resource]
  deployGroupId.value = undefined
  deployDialogVisible.value = true
}

/** 批量部署(从 BatchBar 触发) */
function handleBatchDeploy() {
  const ids = Array.from(selectionStore.selectedIdsForType(props.type))
  const selected = state.value.resources.filter((r) => ids.includes(r.id))
  if (selected.length === 0) return
  deployResources.value = selected
  deployGroupId.value = state.value.currentGroupId === '0' ? undefined : state.value.currentGroupId
  deployDialogVisible.value = true
}

// keep-alive 下每次激活都刷新当前类型：preset 模块可能在别处改动了关联关系
// (preset_links)、分组、删除等，懒加载会导致切回来时数据陈旧。
onActivated(() => resourceStore.fetchResources(props.type))
</script>

<template>
  <div class="p-6 bg-neutral-50 dark:bg-gray-900 min-h-full flex flex-col gap-4">
    <!-- 顶部工具栏：搜索 + 新建 + 视图切换 -->
    <div class="flex items-center justify-between gap-4 flex-wrap">
      <div class="flex items-center gap-3">
        <el-checkbox
          :model-value="isAllSelected"
          :indeterminate="isIndeterminate"
          @change="handleSelectAll"
          >全选</el-checkbox
        >
        <el-input
          v-model="searchInput"
          placeholder="搜索资源..."
          clearable
          prefix-icon="Search"
          class="max-w-xs"
        />
        <el-button type="primary" @click="handleCreate">新建</el-button>
        <el-button v-if="canImport" @click="handleImportClick">导入</el-button>
        <input
          v-if="canImport"
          ref="importInputRef"
          type="file"
          class="hidden"
          multiple
          webkitdirectory
          directory
          @change="handleImportFilesChange"
        />
      </div>
      <div class="flex items-center gap-1">
        <el-button
          :type="uiStore.viewMode === 'grid' ? 'primary' : 'default'"
          text
          @click="uiStore.setViewMode('grid')"
          aria-label="网格视图"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <rect x="3" y="3" width="7" height="7" rx="1" />
            <rect x="14" y="3" width="7" height="7" rx="1" />
            <rect x="3" y="14" width="7" height="7" rx="1" />
            <rect x="14" y="14" width="7" height="7" rx="1" />
          </svg>
        </el-button>
        <el-button
          :type="uiStore.viewMode === 'list' ? 'primary' : 'default'"
          text
          @click="uiStore.setViewMode('list')"
          aria-label="列表视图"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <line x1="4" y1="6" x2="20" y2="6" />
            <line x1="4" y1="12" x2="20" y2="12" />
            <line x1="4" y1="18" x2="20" y2="18" />
          </svg>
        </el-button>
      </div>
    </div>

    <!-- 内容区：加载 / 空状态 / 列表 -->
    <div v-if="state.loading" class="flex-1 flex items-center justify-center">
      <el-icon class="is-loading text-2xl text-gray-400"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2a10 10 0 100 20 10 10 0 000-20zm0 18a8 8 0 110-16 8 8 0 010 16z" opacity=".3"/><path d="M12 2a10 10 0 0110 10h-2a8 8 0 00-8-8V2z"/></svg></el-icon>
    </div>
    <template v-else-if="state.resources.length === 0">
      <EmptyState @click="handleCreate" />
    </template>
    <template v-else>
      <ResourceGrid
        v-if="uiStore.viewMode === 'grid'"
        @edit="handleEdit"
        @edit-content="handleEditContent"
        @deploy="handleDeploy"
        @delete="handleDelete"
        @remove-from-group="handleRemoveFromGroup"
        @view-links="handleViewLinks"
      />
      <ResourceList
        v-else
        @edit="handleEdit"
        @edit-content="handleEditContent"
        @deploy="handleDeploy"
        @delete="handleDelete"
        @remove-from-group="handleRemoveFromGroup"
        @view-links="handleViewLinks"
      />
      <div class="flex justify-center mt-4">
        <el-pagination
          :current-page="state.page"
          :page-size="resourceStore.pageSize"
          :total="state.total"
          layout="prev, pager, next"
          @current-change="handlePageChange"
        />
      </div>
    </template>

    <!-- 批量操作栏 -->
    <BatchBar :type="props.type" @batch-deploy="handleBatchDeploy" />

    <!-- 资源表单对话框 -->
    <ResourceForm
      v-model:visible="formVisible"
      :mode="formMode"
      :resource="formResource"
      @success="handleRefresh"
    />

    <!-- 编辑器抽屉 -->
    <EditorDrawer
      v-model:visible="editorVisible"
      :resource-id="editorResourceId"
      @saved="handleRefresh"
    />

    <!-- 部署对话框 -->
    <DeployDialog
      v-model:visible="deployDialogVisible"
      :resources="deployResources"
      :group-id="deployGroupId"
    />

    <!-- Skill 导入对话框 -->
    <ImportSkillDialog
      v-if="props.type === 'skill'"
      v-model:visible="importSkillVisible"
      :files="importFiles"
      :root-dir-name="importRootName"
      @success="handleImportSuccess"
    />

    <!-- SubAgent 导入对话框 -->
    <ImportAgentDialog
      v-if="props.type === 'agent'"
      v-model:visible="importAgentVisible"
      :files="importFiles"
      :root-dir-name="importRootName"
      @success="handleImportSuccess"
    />

    <!-- 取消关联并删除对话框 -->
    <UnlinkAndDeleteDialog
      v-model:visible="unlinkDialogVisible"
      :resource-name="unlinkTarget?.name || ''"
      :presets="unlinkPresets"
      @confirm="handleUnlinkConfirm"
    />

    <!-- 查看关联对话框 -->
    <ResourceLinksDialog
      v-model:visible="linksDialogVisible"
      :resource="linksResource"
      @unlinked="handleUnlinked"
    />
  </div>
</template>
