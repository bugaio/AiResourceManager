<script setup lang="ts">
import { ref, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useSelectionStore } from '@/stores/selection'
import { useGroupStore } from '@/stores/group'
import { useResourceStore } from '@/stores/resource'
import { useDeployStore } from '@/stores/deploy'
import { useUiStore } from '@/stores/ui'
import { batchDelete } from '@/api/resource'
import type { ResourceType } from '@/types/resource'

/** 批量操作栏 — 选中资源后底部显示
 *
 * 每个 ResourceType 各拥有一个独立实例(skill / mcp / agent),
 * 通过 `type` prop 锁定,只读写自己 type 的选择集合,与其他模块完全隔离。
 */
const props = defineProps<{
  /** 该操作栏所属的资源类型 */
  type: ResourceType
}>()

const selectionStore = useSelectionStore()
const resourceStore = useResourceStore()
const groupStore = useGroupStore()
const deployStore = useDeployStore()
const uiStore = useUiStore()

const emit = defineEmits<{
  (e: 'batch-deploy'): void
}>()

const movePopoverVisible = ref(false)

/** 当前 type 的选中集合(只读) */
const selectedIds = computed(() => selectionStore.selectedIdsForType(props.type))
const selectedCount = computed(() => selectionStore.selectedCountForType(props.type))

/** 仅对"自己这个 type"且"当前正在显示这个 type"时,才考虑分组上下文 */
const isCurrentType = computed(() => uiStore.currentType === props.type)

/** 当前是否在"全部"分组(只在自己 type 是当前 type 时才参与判断) */
const isAllGroup = computed(() =>
  isCurrentType.value ? resourceStore.currentGroupId === '0' : true
)

const groupButtonLabel = computed(() => (isAllGroup.value ? '关联到分组' : '关联到其他分组'))
const deleteButtonLabel = computed(() => (isAllGroup.value ? '批量删除' : '从分组移除'))

/** 可选目标分组(自己 type 是当前 type 时排除当前分组) */
const availableGroups = computed(() => {
  // groupStore 也按 currentType 过滤,只有自己 type 是当前时分组数据才匹配
  if (!isCurrentType.value) return []
  if (isAllGroup.value) return groupStore.groups
  return groupStore.groups.filter((g) => g.id !== resourceStore.currentGroupId)
})

/** 批量删除(带确认) */
async function handleBatchDelete() {
  const ids = Array.from(selectedIds.value)
  if (ids.length === 0) return

  if (isAllGroup.value) {
    try {
      await ElMessageBox.confirm(
        `确定要删除选中的 ${ids.length} 个资源吗？`,
        '确认删除',
        { confirmButtonText: '确定删除', cancelButtonText: '取消', type: 'warning' }
      )
      await batchDelete(ids, true)
      ElMessage.success('批量删除成功')
      selectionStore.clearForType(props.type)
      if (isCurrentType.value) {
        resourceStore.fetchResources()
        deployStore.fetchTargets()
        groupStore.fetchGroups()
      }
    } catch (_e) {}
  } else {
    for (const id of ids) {
      await groupStore.removeResource(resourceStore.currentGroupId, id)
    }
    ElMessage.success('已从分组移除')
    selectionStore.clearForType(props.type)
    resourceStore.fetchResources()
  }
}

function handleBatchDeploy() {
  emit('batch-deploy')
}

async function handleLinkToGroup(groupId: string) {
  const ids = Array.from(selectedIds.value)
  if (ids.length === 0) return
  try {
    await groupStore.addResources(groupId, ids)
    ElMessage.success('已关联到分组')
    selectionStore.clearForType(props.type)
    if (isCurrentType.value) resourceStore.fetchResources()
    movePopoverVisible.value = false
  } catch (e: any) {
    ElMessage.error(e?.message || '关联失败')
  }
}

function handleCancel() {
  selectionStore.clearForType(props.type)
}
</script>

<template>
  <Transition name="batch-bar">
    <div
      v-if="selectedCount > 0"
      class="w-full max-w-xl mx-auto
             bg-blue-50 dark:bg-blue-900/20 rounded-lg p-3 px-5
             flex items-center gap-4 shadow-lg border border-blue-200 dark:border-blue-800"
    >
      <span class="text-sm text-blue-700 dark:text-blue-300">
        已选择 {{ selectedCount }} 项
      </span>
      <el-button type="primary" size="small" @click="handleBatchDeploy">批量部署</el-button>
      <el-popover
        v-model:visible="movePopoverVisible"
        placement="top"
        :width="200"
        trigger="click"
      >
        <template #reference>
          <el-button size="small">{{ groupButtonLabel }}</el-button>
        </template>
        <div class="flex flex-col gap-1 max-h-48 overflow-auto">
          <p class="text-xs text-gray-500 dark:text-gray-400 mb-1">选择目标分组</p>
          <button
            v-for="group in availableGroups"
            :key="group.id"
            class="text-left px-2 py-1.5 text-sm rounded hover:bg-blue-50 dark:hover:bg-blue-900/30 text-gray-700 dark:text-gray-300 transition-colors"
            @click="handleLinkToGroup(group.id)"
          >
            {{ group.name }}
          </button>
          <p v-if="availableGroups.length === 0" class="text-xs text-gray-400 py-2 text-center">
            暂无可选分组
          </p>
        </div>
      </el-popover>
      <el-button type="danger" size="small" @click="handleBatchDelete">{{ deleteButtonLabel }}</el-button>
      <el-button size="small" @click="handleCancel">取消选择</el-button>
    </div>
  </Transition>
</template>

<style scoped>
.batch-bar-enter-active,
.batch-bar-leave-active {
  transition: opacity 0.2s ease;
}
.batch-bar-enter-from,
.batch-bar-leave-to {
  opacity: 0;
}
</style>
