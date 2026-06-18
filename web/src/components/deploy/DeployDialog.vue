<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { ElMessage } from 'element-plus'
import PathInput from '@/components/common/PathInput.vue'
import McpConflictDialog from '@/components/deploy/ConfigConflictDialog.vue'
import type { ConflictTarget } from '@/components/deploy/ConfigConflictDialog.vue'
import { useDeployStore } from '@/stores/deploy'
import { useAliasStore } from '@/stores/alias'
import { useUiStore } from '@/stores/ui'
import { ApiError } from '@/api/request'
import { checkConflicts } from '@/api/deploy'
import type { Resource } from '@/types/resource'
import type { DeployRequest } from '@/types/deploy'

/** 部署对话框 */
const props = defineProps<{
  visible: boolean
  resources: Resource[]
  groupId?: string
}>()

const emit = defineEmits<{
  (e: 'update:visible', val: boolean): void
}>()

const deployStore = useDeployStore()
const aliasStore = useAliasStore()
const uiStore = useUiStore()

// Config / Prompt 模块: 需要走冲突预检流程
const isConfig = computed(() => uiStore.currentType === 'config')
const isPrompt = computed(() => uiStore.currentType === 'prompt')
const needsConflictCheck = computed(() => isConfig.value || isPrompt.value)
const CONFIG_SUFFIXES = ['.json', '.jsonc', '.yaml', '.yml', '.toml']
function isConfigFilePath(p: string): boolean {
  const lower = p.toLowerCase()
  return CONFIG_SUFFIXES.some(ext => lower.endsWith(ext))
}
function isPromptFilePath(p: string): boolean {
  return /\.md$/i.test(p)
}

// 已选目标路径列表
interface SelectedTarget {
  aliasId?: string
  aliasName?: string
  path: string
}
const selectedTargets = ref<SelectedTarget[]>([])

// 当前 PathInput 状态（用于添加）
const currentPath = ref('')
const currentAliasId = ref<string | undefined>(undefined)
const isManualPath = ref(false)
const pathInputKey = ref(0)

// 已选中的别名 ID 列表（传给 PathInput 做排除）
const selectedAliasIds = computed(() => {
  return selectedTargets.value.filter(t => t.aliasId).map(t => t.aliasId!)
})

// 其他表单状态
const track = ref(false)
const saveAsAlias = ref(false)
const aliasName = ref('')

// Config 冲突弹窗
const conflictDialogVisible = ref(false)
const conflictTargets = ref<ConflictTarget[]>([])
// 暂存冲突路径对应的 deploy request（force 时重用）
const pendingConflictReqs = ref<Map<string, DeployRequest>>(new Map())

// 重置表单
watch(() => props.visible, (val) => {
  if (val) {
    selectedTargets.value = []
    currentPath.value = ''
    currentAliasId.value = undefined
    track.value = false
    saveAsAlias.value = false
    aliasName.value = ''
    isManualPath.value = false
    pathInputKey.value += 1
    conflictDialogVisible.value = false
    conflictTargets.value = []
    pendingConflictReqs.value = new Map()
  }
})

/** 添加当前选择的路径到已选列表 */
function handleAddTarget() {
  if (!currentPath.value && !currentAliasId.value) {
    ElMessage.warning('请先选择或输入目标路径')
    return
  }

  const path = currentPath.value
  // 检查是否已添加
  if (selectedTargets.value.some(t => t.path === path)) {
    ElMessage.warning('该路径已添加')
    return
  }

  // Config 目标路径后缀必须是 .json/.jsonc/.yaml/.yml/.toml
  if (isConfig.value && !isConfigFilePath(path)) {
    ElMessage.warning('Config 目标路径后缀必须是 .json/.jsonc/.yaml/.yml/.toml')
    return
  }
  // Prompt 目标路径后缀必须是 .md
  if (isPrompt.value && !isPromptFilePath(path)) {
    ElMessage.warning('Prompt 目标路径后缀必须是 .md')
    return
  }

  const alias = currentAliasId.value
    ? aliasStore.aliases.find(a => a.id === currentAliasId.value)
    : undefined

  selectedTargets.value.push({
    aliasId: currentAliasId.value,
    aliasName: alias?.name,
    path,
  })

  // 重置 PathInput
  currentPath.value = ''
  currentAliasId.value = undefined
  pathInputKey.value += 1
}

/** 移除已选路径 */
function handleRemoveTarget(index: number) {
  selectedTargets.value.splice(index, 1)
}

/** 关闭对话框 */
function handleClose() {
  emit('update:visible', false)
}

/** 确认部署 */
async function handleConfirm() {
  if (selectedTargets.value.length === 0) {
    ElMessage.warning('请至少添加一个目标路径')
    return
  }

  if (saveAsAlias.value && !aliasName.value.trim()) {
    ElMessage.warning('请输入路径别名名称')
    return
  }

  const resourceIds = props.resources.map(r => r.id)

  if (needsConflictCheck.value) {
    // Config / Prompt 路径：先预检冲突（不写入文件），有冲突弹窗让用户确认
    const conflicts: ConflictTarget[] = []
    const reqs = new Map<string, DeployRequest>()

    for (const target of selectedTargets.value) {
      const req: DeployRequest = {
        target_path: target.aliasId ? undefined : target.path,
        alias_id: target.aliasId || undefined,
        force: false,
        resource_ids: resourceIds,
      }
      if (props.groupId) {
        req.group_id = props.groupId
        if (track.value) req.track = true
      }
      reqs.set(target.path, req)

      // 预检冲突
      try {
        const resp = await checkConflicts({
          resource_ids: resourceIds,
          target_path: target.aliasId ? undefined : target.path,
          alias_id: target.aliasId || undefined,
        })
        if (resp.has_conflict) {
          conflicts.push({
            path: target.path,
            aliasName: target.aliasName,
            conflicts: resp.conflicts.map(c => ({ id: c.resource_id, name: c.resource_name, status: c.status, group: c.group })),
          })
        }
      } catch (e: any) {
        ElMessage.error(`检查冲突失败: ${e?.message}`)
      }
    }

    if (conflicts.length > 0) {
      // 有冲突 → 弹自定义冲突弹窗，等用户确认
      conflictTargets.value = conflicts
      pendingConflictReqs.value = reqs
      conflictDialogVisible.value = true
    } else {
      // 无冲突 → 直接部署全部
      await doMergeDeploy(reqs, false)
    }
  } else {
    // skill/agent 路径
    let hasError = false
    for (const target of selectedTargets.value) {
      const req: DeployRequest = {
        target_path: target.aliasId ? undefined : target.path,
        alias_id: target.aliasId || undefined,
        force: false,
      }
      if (props.groupId) {
        req.group_id = props.groupId
        if (track.value) req.track = true
        if (resourceIds.length > 0) req.resource_ids = resourceIds
      } else if (resourceIds.length === 1) {
        req.resource_id = resourceIds[0]
      } else {
        req.resource_ids = resourceIds
      }

      try {
        await deployStore.deploy(req)
      } catch (err: unknown) {
        if (err instanceof ApiError && err.code === 3002) {
          try {
            const { ElMessageBox } = await import('element-plus')
            await ElMessageBox.confirm(
              `「${target.aliasName || target.path}」存在冲突，是否强制覆盖？`,
              '部署冲突',
              { confirmButtonText: '强制覆盖', cancelButtonText: '跳过', type: 'warning' }
            )
            req.force = true
            await deployStore.deploy(req)
          } catch { /* 跳过 */ }
        } else {
          ElMessage.error(`部署失败: ${(err as Error)?.message}`)
          hasError = true
        }
      }
    }
    await saveAlias()
    if (!hasError) ElMessage.success('部署成功')
    handleClose()
  }
}

/** Config / Prompt 实际部署（冲突确认后 or 无冲突时调用） */
async function doMergeDeploy(reqs: Map<string, DeployRequest>, force: boolean) {
  let hasError = false
  for (const [, req] of reqs) {
    if (force) req.force = true
    try {
      await deployStore.deploy(req)
    } catch (e: any) {
      ElMessage.error(`部署失败: ${e?.message}`)
      hasError = true
    }
  }
  await saveAlias()
  if (!hasError) ElMessage.success('部署成功')
  handleClose()
}

/** 冲突弹窗确认：对选中路径强制覆盖（只部署 applied 的资源） */
async function handleConflictConfirm(selectedPaths: string[]) {
  let hasError = false

  for (const target of selectedTargets.value) {
    const req = pendingConflictReqs.value.get(target.path)
    if (!req) continue

    const conflictTarget = conflictTargets.value.find(c => c.path === target.path)

    if (conflictTarget && selectedPaths.includes(target.path)) {
      // 有冲突且用户选中了覆盖：部署 applied + existing 的资源（均带 force）
      // - applied: 本次部署实际生效的资源
      // - existing: 目标已有同名资源（如 prompt 已部署过），需 force 覆盖更新
      const deployIds = conflictTarget.conflicts
        .filter(c => (c.status === 'applied' || c.status === 'existing') && c.id)
        .map(c => c.id!)
      if (deployIds.length > 0) {
        req.resource_ids = deployIds
        req.force = true
        try {
          await deployStore.deploy(req)
        } catch (e: any) {
          ElMessage.error(`部署失败: ${e?.message}`)
          hasError = true
        }
      }
    } else if (!conflictTarget) {
      // 无冲突路径：正常部署全部
      try {
        await deployStore.deploy(req)
      } catch (e: any) {
        ElMessage.error(`部署失败: ${e?.message}`)
        hasError = true
      }
    }
    // 有冲突但未选中：跳过
  }

  await saveAlias()
  if (!hasError) ElMessage.success('部署成功')
  handleClose()
}

/** 保存手动路径为别名 */
async function saveAlias() {
  if (saveAsAlias.value && aliasName.value.trim()) {
    const manualTarget = selectedTargets.value.find(t => !t.aliasId)
    if (manualTarget) {
      try {
        await aliasStore.createAlias({ name: aliasName.value.trim(), path: manualTarget.path })
      } catch (aliasErr: any) {
        ElMessage.warning(aliasErr?.message || '路径别名保存失败')
      }
    }
  }
}
</script>

<template>
  <el-dialog
    :model-value="visible"
    title="部署资源"
    width="560px"
    @close="handleClose"
    :close-on-click-modal="false"
  >
    <div class="flex flex-col gap-4">
      <!-- 路径选择 + 添加按钮 -->
      <div>
        <div class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">目标路径</div>
        <div class="flex gap-2 items-end">
          <div class="flex-1">
            <PathInput
              :key="pathInputKey"
              v-model="currentPath"
              :alias-id="currentAliasId"
              :exclude-alias-ids="selectedAliasIds"
              @update:alias-id="currentAliasId = $event"
              @update:mode="isManualPath = $event === 'manual'"
            />
          </div>
          <el-button type="primary" size="default" @click="handleAddTarget">添加</el-button>
        </div>
      </div>

      <!-- 已选路径列表 -->
      <div v-if="selectedTargets.length > 0">
        <div class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
          已选路径 ({{ selectedTargets.length }})
        </div>
        <div class="max-h-32 overflow-y-auto space-y-1">
          <div
            v-for="(target, index) in selectedTargets"
            :key="index"
            class="flex items-center justify-between px-3 py-1.5 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded"
          >
            <span class="text-sm text-gray-700 dark:text-gray-300 truncate flex-1" :title="target.path">
              {{ target.aliasName ? `${target.aliasName} (${target.path})` : target.path }}
            </span>
            <button
              class="ml-2 p-0.5 text-gray-400 hover:text-red-500 flex-shrink-0"
              @click="handleRemoveTarget(index)"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" viewBox="0 0 20 20" fill="currentColor">
                <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd" />
              </svg>
            </button>
          </div>
        </div>
      </div>

      <!-- 保存为常用路径（有手动输入路径时显示） -->
      <div v-if="selectedTargets.some(t => !t.aliasId)" class="flex flex-col gap-2">
        <el-checkbox v-model="saveAsAlias">保存手动路径为常用路径</el-checkbox>
        <el-input
          v-if="saveAsAlias"
          v-model="aliasName"
          size="small"
          placeholder="输入路径别名名称"
        />
      </div>

      <!-- 跟踪开关（仅分组部署时显示） -->
      <div v-if="groupId" class="flex items-center gap-2">
        <el-switch v-model="track" />
        <span class="text-sm text-gray-600 dark:text-gray-400">
          跟踪分组变化（自动同步增减）
        </span>
      </div>

      <!-- 资源预览 -->
      <div>
        <div class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
          待部署资源 ({{ resources.length }})
        </div>
        <div class="max-h-40 overflow-y-auto space-y-1">
          <div
            v-for="r in resources"
            :key="r.id"
            class="text-sm text-gray-600 dark:text-gray-400 px-2 py-1 bg-gray-50 dark:bg-gray-800 rounded"
          >
            {{ r.name }}
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <el-button @click="handleClose">取消</el-button>
      <el-button type="primary" :loading="deployStore.deploying" @click="handleConfirm">
        确认部署
      </el-button>
    </template>
  </el-dialog>

  <!-- Config 冲突弹窗 -->
  <McpConflictDialog
    v-model:visible="conflictDialogVisible"
    :targets="conflictTargets"
    @confirm="handleConflictConfirm"
  />
</template>
