<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { ElMessageBox, ElMessage } from 'element-plus'
import AppLayout from '@/components/layout/AppLayout.vue'
import ResourceGrid from '@/components/resource/ResourceGrid.vue'
import ResourceList from '@/components/resource/ResourceList.vue'
import EmptyState from '@/components/resource/EmptyState.vue'
import BatchBar from '@/components/common/BatchBar.vue'
import ResourceForm from '@/components/resource/ResourceForm.vue'
import EditorDrawer from '@/components/editor/EditorDrawer.vue'
import DeployDialog from '@/components/deploy/DeployDialog.vue'
import { useUiStore } from '@/stores/ui'
import { useResourceStore } from '@/stores/resource'
import { useDeployStore } from '@/stores/deploy'
import { useSelectionStore } from '@/stores/selection'
import { deleteResource } from '@/api/resource'
import { useGroupStore } from '@/stores/group'
import wsManager from '@/api/ws'
import type { Resource } from '@/types/resource'

/** 资源管理主视图 */
const uiStore = useUiStore()
const resourceStore = useResourceStore()
const deployStore = useDeployStore()
const selectionStore = useSelectionStore()
const groupStore = useGroupStore()

// 搜索关键词（本地输入值）
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
// 部署时关联的分组 ID（从具体分组部署时非空，"全部"时为空）
const deployGroupId = ref<string | undefined>(undefined)

// WebSocket监听事件
function handleWsMessage(data: unknown) {
  if (!data || typeof data !== 'object') return
  const msg = data as Record<string, unknown>

  // 部署同步事件
  if (msg.type === 'deploy:synced') {
    deployStore.fetchTargets()

  // 切换分组时清空选中状态
  selectionStore.clearAll()

    ElMessage.info('部署已自动同步')
  }

  // 资源更新事件 - 刷新列表以获取最新数据
  if (msg.type === 'resource:updated') {
    resourceStore.fetchResources()
  }

  // 资源删除事件 - 从本地列表移除并提示
  if (msg.type === 'resource:deleted') {
    const payload = msg.data as Record<string, unknown> | undefined
    const id = payload?.id as string | undefined
    const uuid = payload?.uuid as string | undefined
    const name = payload?.name as string | undefined
    if (id || uuid) {
      const idx = resourceStore.resources.findIndex(
        (r) => r.id === id || (r as any).uuid === uuid
      )
      if (idx !== -1) {
        const removedName = name || resourceStore.resources[idx].name
        resourceStore.resources.splice(idx, 1)
        ElMessage.warning(`资源 ${removedName} 已被外部删除`)
      }
    }
  }
}

onMounted(() => {
  resourceStore.fetchResources()
  wsManager.onMessage(handleWsMessage)
})

onUnmounted(() => {
  wsManager.offMessage(handleWsMessage)
})

/** 全选状态 */
const isAllSelected = computed(() => {
  const resources = resourceStore.resources
  return resources.length > 0 && resources.every(r => selectionStore.isSelected(r.id))
})

/** 半选状态 */
const isIndeterminate = computed(() => {
  const resources = resourceStore.resources
  const selectedCount = resources.filter(r => selectionStore.isSelected(r.id)).length
  return selectedCount > 0 && selectedCount < resources.length
})

/** 全选/取消全选 */
function handleSelectAll(val: boolean | string | number) {
  if (val) {
    selectionStore.selectAll(resourceStore.resources.map(r => r.id))
  } else {
    selectionStore.clearAll()
  }
}

// 搜索防抖
let debounceTimer: ReturnType<typeof setTimeout> | null = null
watch(searchInput, (val) => {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => {
    resourceStore.setSearch(val)
  }, 300)
})

/** 分页变化 */
function handlePageChange(p: number) {
  resourceStore.setPage(p)
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

/** 表单保存成功后刷新列表 */
function handleFormSuccess() {
  resourceStore.fetchResources()
}

/** 编辑器保存后刷新列表 */
function handleEditorSaved() {
  resourceStore.fetchResources()
}

/** 删除资源（带确认逻辑） */
async function handleDelete(resource: Resource) {
  try {
    await ElMessageBox.confirm(
      `确定要删除资源「${resource.name}」吗？`,
      '确认删除',
      { confirmButtonText: '确定删除', cancelButtonText: '取消', type: 'warning' }
    )
  } catch {
    return // 用户取消
  }

  try {
    await deleteResource(resource.id)
    ElMessage.success('删除成功')
    resourceStore.fetchResources()
    deployStore.fetchTargets()
    groupStore.fetchGroups()
  } catch (e: any) {
    // 如果后端返回code 1004（有部署信息），二次确认
    if (e?.message?.includes('部署') || e?.code === 1004) {
      try {
        await ElMessageBox.confirm(
          e.message || '该资源已部署，确定删除？',
          '确认删除',
          { confirmButtonText: '确定删除', cancelButtonText: '取消', type: 'warning' }
        )
        await deleteResource(resource.id, true)
        ElMessage.success('删除成功')
        resourceStore.fetchResources()
        deployStore.fetchTargets()
        groupStore.fetchGroups()
      } catch {
        // 用户取消
      }
    } else {
      ElMessage.error(e?.message || '删除失败')
    }
  }
}

/** 部署资源 */

/** 从分组移除资源 */
async function handleRemoveFromGroup(resource: Resource) {
  try {
    await groupStore.removeResource(resourceStore.currentGroupId, resource.id)
    ElMessage.success("已从分组移除")
    resourceStore.fetchResources()
  } catch (e: any) {
    ElMessage.error(e?.message || "移除失败")
  }
}

function handleDeploy(resource: Resource) {
  deployResources.value = [resource]
  // 单个资源部署不关联分组
  deployGroupId.value = undefined
  deployDialogVisible.value = true
}

/** 批量部署（从BatchBar触发） */
function handleBatchDeploy() {
  const ids = Array.from(selectionStore.selectedIds)
  const selected = resourceStore.resources.filter(r => ids.includes(r.id))
  if (selected.length === 0) return
  deployResources.value = selected
  // 在具体分组下批量部署时，关联该分组；"全部"时不关联
  deployGroupId.value = resourceStore.currentGroupId === '0'
    ? undefined
    : resourceStore.currentGroupId
  deployDialogVisible.value = true
}
</script>

<template>
  <AppLayout>
    <div class="p-6 bg-neutral-50 dark:bg-gray-900 min-h-full flex flex-col gap-4">
      <!-- 顶部工具栏：搜索 + 新建 + 视图切换 -->
      <div class="flex items-center justify-between gap-4 flex-wrap">
        <div class="flex items-center gap-3">
          <el-checkbox
            :model-value="isAllSelected"
            :indeterminate="isIndeterminate"
            @change="handleSelectAll"
          >全选</el-checkbox>
          <el-input
            v-model="searchInput"
            placeholder="搜索资源..."
            clearable
            prefix-icon="Search"
            class="max-w-xs"
          />
          <el-button type="primary" @click="handleCreate">新建</el-button>
        </div>
        <div class="flex items-center gap-1">
          <!-- 网格视图 -->
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
          <!-- 列表视图 -->
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

      <!-- 内容区：空状态或资源列表 -->
      <div v-if="resourceStore.loading" class="flex-1 flex items-center justify-center">
        <el-icon class="is-loading text-2xl text-gray-400"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2a10 10 0 100 20 10 10 0 000-20zm0 18a8 8 0 110-16 8 8 0 010 16z" opacity=".3"/><path d="M12 2a10 10 0 0110 10h-2a8 8 0 00-8-8V2z"/></svg></el-icon>
      </div>
      <template v-else-if="resourceStore.resources.length === 0">
        <EmptyState @click="handleCreate" />
      </template>
      <template v-else>
        <!-- 网格 / 列表 -->
        <ResourceGrid
          v-if="uiStore.viewMode === 'grid'"
          @edit="handleEdit"
          @edit-content="handleEditContent"
          @deploy="handleDeploy"
          @delete="handleDelete" @remove-from-group="handleRemoveFromGroup"
        />
        <ResourceList
          v-else
          @edit="handleEdit"
          @edit-content="handleEditContent"
          @deploy="handleDeploy"
          @delete="handleDelete" @remove-from-group="handleRemoveFromGroup"
        />
        <!-- 分页 -->
        <div class="flex justify-center mt-4">
          <el-pagination
            :current-page="resourceStore.page"
            :page-size="resourceStore.pageSize"
            :total="resourceStore.total"
            layout="prev, pager, next"
            @current-change="handlePageChange"
          />
        </div>
      </template>

      <!-- 批量操作栏 -->
      <BatchBar @batch-deploy="handleBatchDeploy" />
    </div>

    <!-- 资源表单对话框 -->
    <ResourceForm
      v-model:visible="formVisible"
      :mode="formMode"
      :resource="formResource"
      @success="handleFormSuccess"
    />

    <!-- 编辑器抽屉 -->
    <EditorDrawer
      v-model:visible="editorVisible"
      :resource-id="editorResourceId"
      @saved="handleEditorSaved"
    />

    <!-- 部署对话框 -->
    <DeployDialog
      v-model:visible="deployDialogVisible"
      :resources="deployResources"
      :group-id="deployGroupId"
    />
  </AppLayout>
</template>
