<script setup lang="ts">
/** Preset 侧栏：Preset 列表 + Path Group 列表（含已部署 preset） */
import { ref, computed, watch, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { usePresetStore } from '@/stores/preset'
import { usePathGroupStore } from '@/stores/pathGroup'
import { useDeployStore } from '@/stores/deploy'
import { checkPathExists } from '@/api/deploy'
import { undeployPresetDeployment } from '@/api/preset'
import PresetForm from '@/components/preset/PresetForm.vue'
import PathGroupForm from '@/components/preset/PathGroupForm.vue'
import type { Preset } from '@/types/preset'
import type { PathGroup } from '@/types/pathGroup'
import type { Deployment } from '@/types/deploy'
import { isRandomGroupName } from '@/utils/pathFormat'

const emit = defineEmits<{
  (e: 'open-deploy-manage', payload: { presetID: string; presetName: string; groupID: string; groupName: string }): void
}>()

const presetStore = usePresetStore()
const pathGroupStore = usePathGroupStore()
const deployStore = useDeployStore()

/** 路径组检查状态 — 防止重复点击 */
const checkingGroups = ref<Set<string>>(new Set())

/** 各路径组下 broken item 映射 — preset.id → Set<itemId> */
const brokenItemsByPreset = ref<Record<string, Set<string>>>({})

const presetCollapsed = ref(false)
const pathGroupCollapsed = ref(false)

// Preset 表单
const presetFormVisible = ref(false)
const presetFormMode = ref<'create' | 'edit'>('create')
const editingPreset = ref<Preset | null>(null)

// PathGroup 表单
const pathGroupFormVisible = ref(false)
const pathGroupFormMode = ref<'create' | 'edit'>('create')
const editingPathGroup = ref<PathGroup | null>(null)

onMounted(() => {
  presetStore.fetchPresets()
  pathGroupStore.fetchPathGroups()
})

/** 每个路径组下已部署的 preset 映射（按 preset 去重，聚合部署信息） */
interface GroupDeployedPreset {
  preset: Preset
  /** 匹配此路径组的 deployment 列表 */
  matchedDeployments: Deployment[]
  /** 部署类型标签（去重后） */
  typeTags: string[]
  /** 是否有跟踪 */
  isTracking: boolean
  /** 最近部署时间 */
  latestTime: string
}

const deployedPresetsByGroup = computed(() => {
  const map: Record<string, GroupDeployedPreset[]> = {}
  for (const g of pathGroupStore.pathGroups) {
    const groupPaths = new Set(
      [g.skill_path, g.agent_path, g.config_path, g.prompt_path].filter(Boolean)
    )
    const items: GroupDeployedPreset[] = []
    for (const p of presetStore.presets) {
      if (!p.deployments || p.deployments.length === 0) continue
      const matched = p.deployments.filter((d) => groupPaths.has(d.target_path))
      if (matched.length === 0) continue
      const typeTags = [...new Set(matched.map((d) => d.deploy_type))]
      const isTracking = matched.some((d) => d.track === 1)
      const latestTime = matched.reduce((latest, d) => (d.created_at > latest ? d.created_at : latest), '')
      items.push({ preset: p, matchedDeployments: matched, typeTags, isTracking, latestTime })
    }
    map[g.id] = items
  }
  return map
})

/** 路径组展开状态 — 默认全部展开 */
const expandedGroups = ref<Set<string>>(new Set())

/** pathGroups 加载/变更后，自动展开新出现的组 */
watch(
  () => pathGroupStore.pathGroups,
  (groups) => {
    const s = new Set(expandedGroups.value)
    for (const g of groups) {
      s.add(g.id)
    }
    expandedGroups.value = s
  },
  { immediate: true },
)

function toggleGroupExpand(groupID: string) {
  const s = new Set(expandedGroups.value)
  if (s.has(groupID)) s.delete(groupID)
  else s.add(groupID)
  expandedGroups.value = s
}

/** 点击已部署 preset 项，打开管理对话框（按路径组维度，含缺失类型漂移） */
function handleOpenDeployManage(g: PathGroup, item: GroupDeployedPreset) {
  emit('open-deploy-manage', {
    presetID: item.preset.id,
    presetName: item.preset.name,
    groupID: g.id,
    groupName: pathGroupDisplayName(g),
  })
}

/** 点击路径组的 ✓ 按钮 — 检查该路径组下所有 preset 部署的健康状态 */
async function handleCheckGroup(g: PathGroup) {
  if (checkingGroups.value.has(g.id)) return
  const items = deployedPresetsByGroup.value[g.id] || []
  if (items.length === 0) {
    ElMessage.info('该路径组下没有已部署的 Preset')
    return
  }
  const s = new Set(checkingGroups.value)
  s.add(g.id)
  checkingGroups.value = s
  try {
    // 1. 收集该路径组涉及的目标路径
    const targetPaths = new Set<string>(
      [g.skill_path, g.agent_path, g.config_path, g.prompt_path].filter(Boolean),
    )

    // 2. 检查每个目标路径是否还存在；不存在则自动撤销关联部署
    const removedPathsMsg: string[] = []
    for (const tp of targetPaths) {
      try {
        const { exists } = await checkPathExists(tp)
        if (exists) continue
        // 路径不存在 — 撤销所有 preset 在该路径上的部署
        for (const item of items) {
          for (const dep of item.matchedDeployments) {
            if (dep.target_path === tp) {
              try {
                await undeployPresetDeployment(item.preset.id, dep.id)
              } catch {
                // 忽略单条失败,继续
              }
            }
          }
        }
        removedPathsMsg.push(tp)
      } catch {
        // checkPathExists 失败也继续
      }
    }
    if (removedPathsMsg.length > 0) {
      ElMessage.warning(`目标路径已不存在，已自动清理: ${removedPathsMsg.join(', ')}`)
      await presetStore.fetchPresets()
    }

    // 3. 全局 health check,过滤出属于该路径组涉及目标的 broken 项
    const broken = await deployStore.checkHealth()
    // 该路径组的 deployment id 集合（刷新前的快照已经够用）
    const groupDeploymentIDs = new Set<string>()
    for (const item of items) {
      for (const dep of item.matchedDeployments) {
        groupDeploymentIDs.add(dep.id)
      }
    }
    const myBroken = broken.filter((b) => groupDeploymentIDs.has(b.deployment_id))

    // 4. 把 broken 按 preset 分组存入,模板会显示标记和修复按钮
    //    先清除该路径组下旧的 broken 记录(防止已修复的项残留)
    const newBrokenMap: Record<string, Set<string>> = { ...brokenItemsByPreset.value }
    for (const item of items) {
      newBrokenMap[item.preset.id] = new Set()
    }
    for (const b of myBroken) {
      for (const item of items) {
        if (item.matchedDeployments.some((d) => d.id === b.deployment_id)) {
          newBrokenMap[item.preset.id].add(b.id)
        }
      }
    }
    brokenItemsByPreset.value = newBrokenMap

    if (removedPathsMsg.length === 0 && myBroken.length === 0) {
      ElMessage.success(`路径组「${g.name}」下所有部署状态正常`)
    } else if (myBroken.length > 0) {
      ElMessage.warning(`路径组「${g.name}」发现 ${myBroken.length} 个异常项，点"修复"还原`)
    }
  } catch (e: any) {
    ElMessage.error(e?.message || '检查失败')
  } finally {
    const ns = new Set(checkingGroups.value)
    ns.delete(g.id)
    checkingGroups.value = ns
  }
}

/** 修复某 preset 在此路径组下的所有 broken item */
async function handleRepairPreset(item: GroupDeployedPreset) {
  const brokenIDs = brokenItemsByPreset.value[item.preset.id]
  if (!brokenIDs || brokenIDs.size === 0) return
  // broken itemId 不直接给我们 deployment_id,需要重新拿一次 health 找到 (deployment_id, item_id) 对
  try {
    const broken = await deployStore.checkHealth()
    const pairs: Array<{ depID: string; itemID: string }> = []
    const matchedDepIDs = new Set(item.matchedDeployments.map((d) => d.id))
    for (const b of broken) {
      if (matchedDepIDs.has(b.deployment_id) && brokenIDs.has(b.id)) {
        pairs.push({ depID: b.deployment_id, itemID: b.id })
      }
    }
    if (pairs.length === 0) {
      ElMessage.info('没有可修复的异常项')
      brokenItemsByPreset.value = { ...brokenItemsByPreset.value, [item.preset.id]: new Set() }
      return
    }
    let failCount = 0
    for (const p of pairs) {
      try {
        await deployStore.repair(p.depID, p.itemID)
      } catch {
        failCount++
      }
    }
    if (failCount > 0) {
      ElMessage.warning(`${pairs.length - failCount} 项修复成功，${failCount} 项失败`)
    } else {
      ElMessage.success(`已修复 ${pairs.length} 项`)
    }
    // 清除该 preset 的 broken 标记
    brokenItemsByPreset.value = { ...brokenItemsByPreset.value, [item.preset.id]: new Set() }
  } catch (e: any) {
    ElMessage.error(e?.message || '修复失败')
  }
}

/** 移除某 preset 在此路径组下的所有部署 */
async function handleRemoveDeployment(preset: Preset, deployments: Deployment[]) {
  if (deployments.length === 0) return
  try {
    await ElMessageBox.confirm(
      `确定要移除 Preset「${preset.name}」在此路径组下的 ${deployments.length} 个部署吗？`,
      '移除部署',
      { confirmButtonText: '移除', cancelButtonText: '取消', type: 'warning' },
    )
  } catch {
    return
  }
  let failCount = 0
  for (const d of deployments) {
    try {
      await undeployPresetDeployment(preset.id, d.id)
    } catch {
      failCount++
    }
  }
  if (failCount > 0) {
    ElMessage.warning(`${failCount} 个移除失败，其余已移除`)
  } else {
    ElMessage.success('已移除')
  }
  await presetStore.fetchPresets()
}

/** 部署类型简写 */
function deployTypeLabel(t: string) {
  return t === 'symlink' ? '软链' : t === 'merge' ? '合并' : t
}

/** 该 preset 在此路径组下的漂移总数（新增未部署 + 残留），按路径组维度 */
function deployDriftCount(groupID: string, item: GroupDeployedPreset): number {
  const d = item.preset.group_drifts?.[groupID]
  if (!d) return 0
  return (d.pending || 0) + (d.stale || 0)
}

/**
 * 路径组整体部署状态分类（用于卡片颜色）：
 *   - none      未部署任何 preset            → 灰
 *   - unsynced  存在「普通部署且有漂移」      → 红（最需关注，优先级最高）
 *   - tracking  存在跟踪部署（且无未同步）    → 蓝（柔和）
 *   - synced    仅普通部署且全部同步          → 绿
 */
type GroupDeployState = 'none' | 'unsynced' | 'tracking' | 'synced'
function groupDeployState(g: PathGroup): GroupDeployState {
  const items = deployedPresetsByGroup.value[g.id] || []
  if (items.length === 0) return 'none'
  let hasUnsynced = false
  let hasTracking = false
  for (const item of items) {
    const drift = deployDriftCount(g.id, item)
    if (item.isTracking) hasTracking = true
    // 普通部署（非跟踪）且有漂移 → 未同步
    if (!item.isTracking && drift > 0) hasUnsynced = true
  }
  if (hasUnsynced) return 'unsynced'
  if (hasTracking) return 'tracking'
  return 'synced'
}

/** 卡片 ring（边框）颜色 */
function groupRingClass(g: PathGroup): string {
  switch (groupDeployState(g)) {
    case 'unsynced':
      return 'ring-rose-300/80 dark:ring-rose-800/60'
    case 'tracking':
      return 'ring-sky-200/80 dark:ring-sky-900/50'
    case 'synced':
      return 'ring-green-200/70 dark:ring-green-900/40'
    default:
      return 'ring-gray-200/60 dark:ring-gray-700/50'
  }
}

/** 卡片背景色 */
function groupBgClass(g: PathGroup): string {
  switch (groupDeployState(g)) {
    case 'unsynced':
      return 'bg-rose-50/70 dark:bg-rose-950/20'
    case 'tracking':
      return 'bg-sky-50/60 dark:bg-sky-950/20'
    case 'synced':
      return 'bg-green-50/50 dark:bg-green-950/15'
    default:
      return 'bg-white dark:bg-gray-800/60'
  }
}

/** 左侧状态色条颜色 */
function groupBarClass(g: PathGroup): string {
  switch (groupDeployState(g)) {
    case 'unsynced':
      return 'bg-rose-400 dark:bg-rose-500'
    case 'tracking':
      return 'bg-sky-400 dark:bg-sky-500'
    case 'synced':
      return 'bg-green-400 dark:bg-green-500'
    default:
      return 'bg-gray-200 dark:bg-gray-600'
  }
}

/** 新建 preset */
function handleCreatePreset() {
  presetFormMode.value = 'create'
  editingPreset.value = null
  presetFormVisible.value = true
}

/** 选中 preset */
function handleSelectPreset(id: string) {
  presetStore.selectPreset(id)
}

/** preset 菜单 */
function handlePresetCommand(cmd: string, p: Preset) {
  if (cmd === 'edit') {
    presetFormMode.value = 'edit'
    editingPreset.value = p
    presetFormVisible.value = true
  } else if (cmd === 'delete') {
    confirmDeletePreset(p)
  }
}

async function confirmDeletePreset(p: Preset) {
  try {
    await ElMessageBox.confirm(
      `确定要删除 Preset「${p.name}」吗？\n\n会自动撤销其部署、删除私有资源、解除关联资源。`,
      '确认删除',
      { confirmButtonText: '确定删除', cancelButtonText: '取消', type: 'warning' },
    )
  } catch {
    return
  }
  try {
    await presetStore.deletePreset(p.id)
    ElMessage.success('Preset 已删除')
  } catch (e: any) {
    ElMessage.error(e?.message || '删除失败')
  }
}

/** 新建 path group */
function handleCreatePathGroup() {
  pathGroupFormMode.value = 'create'
  editingPathGroup.value = null
  pathGroupFormVisible.value = true
}

/** path group 菜单 */
function handlePathGroupCommand(cmd: string, g: PathGroup) {
  if (cmd === 'edit') {
    pathGroupFormMode.value = 'edit'
    editingPathGroup.value = g
    pathGroupFormVisible.value = true
  } else if (cmd === 'delete') {
    confirmDeletePathGroup(g)
  }
}

async function confirmDeletePathGroup(g: PathGroup) {
  try {
    await ElMessageBox.confirm(`确定要删除路径组「${g.name}」吗？`, '确认删除', {
      confirmButtonText: '确定删除',
      cancelButtonText: '取消',
      type: 'warning',
    })
  } catch {
    return
  }
  try {
    await pathGroupStore.deletePathGroup(g.id)
    ElMessage.success('路径组已删除')
  } catch (e: any) {
    ElMessage.error(e?.message || '删除失败')
  }
}

/** 简化的子路径预览（前 2 个非空） */
function pathGroupBrief(g: PathGroup): string {
  const items: string[] = []
  if (g.skill_path) items.push('skill')
  if (g.agent_path) items.push('agent')
  if (g.config_path) items.push('config')
  if (g.prompt_path) items.push('prompt')
  return items.join(' · ')
}

/** 路径组显示名：随机名展示为友好占位，正式别名直接显示 */
function pathGroupDisplayName(g: PathGroup): string {
  return isRandomGroupName(g.name) ? '未命名路径组' : g.name
}

/** 是否随机名（无正式别名） */
function isRandomGroup(g: PathGroup): boolean {
  return isRandomGroupName(g.name)
}
</script>

<template>
  <aside
    class="flex flex-col h-full overflow-hidden bg-slate-50 dark:bg-slate-900 border-r border-gray-200 dark:border-gray-700 flex-shrink-0"
  >
    <!-- 上部：Preset 列表 -->
    <div class="flex flex-col px-3 pt-3 pb-1 min-h-0">
      <div class="flex items-center justify-between mb-1">
        <div
          class="flex items-center gap-1 cursor-pointer select-none"
          @click="presetCollapsed = !presetCollapsed"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="w-3.5 h-3.5 text-gray-400 transition-transform duration-200"
            :class="{ '-rotate-90': presetCollapsed }"
            viewBox="0 0 20 20"
            fill="currentColor"
          >
            <path
              fill-rule="evenodd"
              d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z"
              clip-rule="evenodd"
            />
          </svg>
          <span class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Presets</span>
        </div>
        <button
          class="p-0.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
          @click="handleCreatePreset"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" viewBox="0 0 20 20" fill="currentColor">
            <path
              fill-rule="evenodd"
              d="M10 3a1 1 0 011 1v5h5a1 1 0 110 2h-5v5a1 1 0 11-2 0v-5H4a1 1 0 110-2h5V4a1 1 0 011-1z"
              clip-rule="evenodd"
            />
          </svg>
        </button>
      </div>
      <div v-show="!presetCollapsed" class="overflow-y-auto flex-1 min-h-0">
        <div v-if="presetStore.loading" class="text-xs text-gray-400 px-3 py-1">加载中...</div>
        <div
          v-else-if="presetStore.presets.length === 0"
          class="text-xs text-gray-400 px-3 py-2"
        >
          暂无 Preset
        </div>
        <div
          v-for="p in presetStore.presets"
          :key="p.id"
          class="flex items-center justify-between px-3 py-2 rounded-md cursor-pointer text-sm transition-colors"
          :class="
            presetStore.currentPresetID === p.id
              ? 'bg-blue-100 dark:bg-blue-900/40 text-blue-700 dark:text-blue-300'
              : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800'
          "
          @click="handleSelectPreset(p.id)"
        >
          <span class="truncate flex-1">{{ p.name }}</span>
          <span
            v-if="p.resource_count > 0"
            class="text-xs text-gray-400 ml-1"
          >({{ p.resource_count }})</span>
          <el-dropdown
            trigger="click"
            @command="(cmd: string) => handlePresetCommand(cmd, p)"
          >
            <span
              class="ml-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 flex-shrink-0"
              @click.stop
            >···</span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="edit">编辑</el-dropdown-item>
                <el-dropdown-item command="delete" divided>删除</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </div>
    </div>

    <!-- 下部：Path Group 列表（含已部署 preset） -->
    <div class="flex flex-col px-3 pt-1 pb-3 border-t border-gray-200 dark:border-gray-700 min-h-0 flex-1">
      <div class="flex items-center justify-between mb-1">
        <div
          class="flex items-center gap-1 cursor-pointer select-none"
          @click="pathGroupCollapsed = !pathGroupCollapsed"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="w-3.5 h-3.5 text-gray-400 transition-transform duration-200"
            :class="{ '-rotate-90': pathGroupCollapsed }"
            viewBox="0 0 20 20"
            fill="currentColor"
          >
            <path
              fill-rule="evenodd"
              d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z"
              clip-rule="evenodd"
            />
          </svg>
          <span class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Path Groups</span>
        </div>
        <button
          class="p-0.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
          @click="handleCreatePathGroup"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" viewBox="0 0 20 20" fill="currentColor">
            <path
              fill-rule="evenodd"
              d="M10 3a1 1 0 011 1v5h5a1 1 0 110 2h-5v5a1 1 0 11-2 0v-5H4a1 1 0 110-2h5V4a1 1 0 011-1z"
              clip-rule="evenodd"
            />
          </svg>
        </button>
      </div>
      <div v-show="!pathGroupCollapsed" class="overflow-y-auto flex-1 min-h-0 space-y-2.5 px-1 py-1">
        <div
          v-if="pathGroupStore.pathGroups.length === 0"
          class="text-xs text-gray-400 px-3 py-2"
        >
          暂无路径组
        </div>
        <div
          v-for="g in pathGroupStore.pathGroups"
          :key="g.id"
          class="relative rounded-lg shadow-sm ring-1 overflow-hidden transition-all hover:shadow-md"
          :class="[groupRingClass(g), groupBgClass(g)]"
        >
          <!-- 左侧状态色条：未同步=红 / 跟踪=蓝 / 已同步=绿 / 未部署=灰 -->
          <span
            class="absolute left-0 top-0 bottom-0 w-1"
            :class="groupBarClass(g)"
          ></span>
          <!-- 路径组主行 -->
          <div
            class="flex items-center justify-between gap-2 pl-3 pr-2 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-50/80 dark:hover:bg-gray-800/60 cursor-pointer"
            @click="toggleGroupExpand(g.id)"
          >
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-1.5">
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="w-3 h-3 text-gray-400 transition-transform duration-150 flex-shrink-0"
                  :class="{ 'rotate-90': expandedGroups.has(g.id) }"
                  viewBox="0 0 20 20"
                  fill="currentColor"
                >
                  <path
                    fill-rule="evenodd"
                    d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z"
                    clip-rule="evenodd"
                  />
                </svg>
                <span
                  class="truncate font-medium"
                  :class="isRandomGroup(g) ? 'text-gray-400 dark:text-gray-500 italic' : 'text-gray-700 dark:text-gray-200'"
                >{{ pathGroupDisplayName(g) }}</span>
                <span
                  v-if="isRandomGroup(g)"
                  class="text-[10px] px-1 rounded bg-gray-100 dark:bg-gray-700 text-gray-400 dark:text-gray-500 flex-shrink-0"
                >随机</span>
                <span
                  v-if="(deployedPresetsByGroup[g.id] || []).length > 0"
                  class="text-[10px] bg-green-100 dark:bg-green-900/40 text-green-600 dark:text-green-400 rounded-full px-1.5 flex-shrink-0"
                >{{ deployedPresetsByGroup[g.id].length }}</span>
              </div>
              <div class="text-[10px] text-gray-400 truncate mt-0.5 pl-[18px]">{{ pathGroupBrief(g) }}</div>
            </div>
            <div class="flex items-center gap-0.5 flex-shrink-0">
              <!-- 检查该路径组下所有 preset 部署的健康状态 -->
              <button
                class="p-1 text-gray-400 hover:text-blue-500 dark:hover:text-blue-400 hover:bg-gray-100 dark:hover:bg-gray-700 rounded disabled:opacity-50"
                title="检查健康状态"
                :disabled="checkingGroups.has(g.id)"
                @click.stop="handleCheckGroup(g)"
              >
                <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" viewBox="0 0 20 20" fill="currentColor">
                  <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                </svg>
              </button>
              <el-dropdown
                trigger="click"
                @command="(cmd: string) => handlePathGroupCommand(cmd, g)"
              >
                <span
                  class="inline-flex items-center justify-center w-6 h-6 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700 rounded"
                  @click.stop
                >···</span>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="edit">编辑</el-dropdown-item>
                    <el-dropdown-item command="delete" divided>删除</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
          </div>

          <!-- 已部署 preset 子列表（展开时显示） -->
          <div
            v-if="expandedGroups.has(g.id) && deployedPresetsByGroup[g.id]?.length > 0"
            class="px-2 pb-2 pt-0.5 space-y-0.5 bg-gray-50/60 dark:bg-gray-900/30 border-t border-gray-100 dark:border-gray-700/60"
          >
            <div
              v-for="item in deployedPresetsByGroup[g.id]"
              :key="item.preset.id"
              class="flex items-center gap-1.5 px-2 py-1 rounded text-xs cursor-pointer hover:bg-white dark:hover:bg-gray-800 transition-colors group"
              @click.stop="handleOpenDeployManage(g, item)"
            >
              <span class="truncate flex-1 text-gray-600 dark:text-gray-400">{{ item.preset.name }}</span>
              <!-- 漂移标识：preset 资源集与已部署快照不一致（新增未部署 / 残留） -->
              <span
                v-if="deployDriftCount(g.id, item) > 0"
                class="text-[10px] flex-shrink-0 px-1 rounded bg-amber-100 dark:bg-amber-900/40 text-amber-600 dark:text-amber-400"
                title="部署内容与 Preset 当前资源不一致，点击查看并重新部署"
              >未同步</span>
              <el-tag
                v-for="t in item.typeTags"
                :key="t"
                size="small"
                :type="t === 'symlink' ? 'primary' : 'warning'"
                class="flex-shrink-0 !py-0 !px-1 text-[10px]"
              >
                {{ deployTypeLabel(t) }}
              </el-tag>
              <span
                v-if="item.isTracking"
                class="text-[10px] flex-shrink-0"
                title="跟踪变更"
              >🔗</span>
              <!-- broken 标记 + 修复按钮 -->
              <template v-if="brokenItemsByPreset[item.preset.id]?.size">
                <span class="text-[10px] text-orange-500 flex-shrink-0">⚠️{{ brokenItemsByPreset[item.preset.id]?.size }}</span>
                <button
                  class="text-[10px] text-blue-500 hover:text-blue-600 flex-shrink-0"
                  @click.stop="handleRepairPreset(item)"
                >修复</button>
              </template>
              <button
                class="text-[10px] text-gray-400 hover:text-red-500 px-1.5 py-0.5 rounded transition-colors flex-shrink-0"
                title="移除此部署"
                @click.stop="handleRemoveDeployment(item.preset, item.matchedDeployments)"
              >移除</button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Preset 表单 -->
    <PresetForm
      v-model:visible="presetFormVisible"
      :mode="presetFormMode"
      :preset="editingPreset"
    />
    <!-- Path Group 表单 -->
    <PathGroupForm
      v-model:visible="pathGroupFormVisible"
      :mode="pathGroupFormMode"
      :path-group="editingPathGroup"
    />
  </aside>
</template>
