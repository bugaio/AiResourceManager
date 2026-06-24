<script setup lang="ts">
/** 查看某资源关联的 Preset 列表
 *
 * 资源被哪些 preset 关联（resource.preset_links），每个 preset 下展示其部署到的
 * 路径组名称；该路径组若开启「跟踪变更」(track=1) 显示 🔗。
 * 每个 preset 右侧提供「取消关联」——等同于在 Preset 内取消关联：
 * 若该 preset 部署到的某些路径组开启了 track，取消关联会移除这些路径组里已部署的内容。
 * 因此先弹二次确认，列出受影响的 track=1 路径组及其会变更的子路径。
 */
import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { usePresetStore } from '@/stores/preset'
import { usePathGroupStore } from '@/stores/pathGroup'
import type { Resource } from '@/types/resource'
import type { Deployment } from '@/types/deploy'
import type { PathGroup } from '@/types/pathGroup'

const props = defineProps<{
  visible: boolean
  resource: Resource | null
}>()

const emit = defineEmits<{
  (e: 'update:visible', val: boolean): void
  /** 取消关联成功后通知父级刷新列表 */
  (e: 'unlinked'): void
}>()

const presetStore = usePresetStore()
const pathGroupStore = usePathGroupStore()

const dialogVisible = computed({
  get: () => props.visible,
  set: (v) => emit('update:visible', v),
})

/** 类型 → 路径组子路径字段 */
const TYPE_PATH_KEY: Record<string, keyof PathGroup> = {
  skill: 'skill_path',
  agent: 'agent_path',
  config: 'config_path',
  prompt: 'prompt_path',
}
const TYPE_LABEL: Record<string, string> = {
  skill: 'Skill',
  agent: 'SubAgent',
  config: 'Config',
  prompt: 'Prompt',
}

/** 一条部署在某路径组下的归属信息 */
interface DeployedGroupInfo {
  group: PathGroup
  /** 命中该路径组的部署（每种类型一条） */
  deployments: Deployment[]
  /** 是否有跟踪（任一命中的部署 track=1） */
  tracking: boolean
}

/** 关联此资源的 preset（用 preset_links 的 id 去 store 取完整 preset，拿 deployments） */
interface LinkedPreset {
  id: string
  name: string
  deployedGroups: DeployedGroupInfo[]
}

/** 把一个 preset 的 deployments 按路径组聚合 */
function groupDeployments(deployments: Deployment[]): DeployedGroupInfo[] {
  const result: DeployedGroupInfo[] = []
  for (const g of pathGroupStore.pathGroups) {
    const groupPaths = new Set(
      [g.skill_path, g.agent_path, g.config_path, g.prompt_path].filter(Boolean),
    )
    const matched = deployments.filter((d) => groupPaths.has(d.target_path))
    if (matched.length === 0) continue
    result.push({
      group: g,
      deployments: matched,
      tracking: matched.some((d) => d.track === 1),
    })
  }
  return result
}

/** 当前资源关联的 preset 列表（含部署归属） */
const linkedPresets = computed<LinkedPreset[]>(() => {
  const links = props.resource?.preset_links || []
  return links.map((link) => {
    const full = presetStore.presets.find((p) => p.id === link.id)
    const deps = full?.deployments || []
    return {
      id: link.id,
      name: link.name,
      deployedGroups: groupDeployments(deps),
    }
  })
})

/** 把某条 deployment 的 target_path 反查出它对应的资源类型标签 */
function deploymentTypeLabel(group: PathGroup, dep: Deployment): string {
  for (const [type, key] of Object.entries(TYPE_PATH_KEY)) {
    if (group[key] === dep.target_path) return TYPE_LABEL[type] || type
  }
  return ''
}

// ---- 二次确认弹窗 ----
const confirmVisible = ref(false)
const confirmTarget = ref<LinkedPreset | null>(null)
const submitting = ref(false)

/** 受影响的 track=1 路径组及其会变更的子路径 */
interface AffectedGroup {
  groupName: string
  /** 会发生内容变更的子路径(类型标签 + 路径) */
  subPaths: { label: string; path: string }[]
}
const affectedGroups = computed<AffectedGroup[]>(() => {
  const t = confirmTarget.value
  if (!t) return []
  return t.deployedGroups
    .filter((dg) => dg.tracking)
    .map((dg) => ({
      groupName: dg.group.name,
      subPaths: dg.deployments
        .filter((d) => d.track === 1)
        .map((d) => ({ label: deploymentTypeLabel(dg.group, d), path: d.target_path })),
    }))
    .filter((ag) => ag.subPaths.length > 0)
})

/** 点击某 preset 的「取消关联」 */
function handleUnlinkClick(lp: LinkedPreset) {
  confirmTarget.value = lp
  confirmVisible.value = true
}

/** 二次确认 → 执行取消关联（后端对 track=1 部署自动移除已部署内容）
 *
 * 关闭策略：
 * - 取消关联前若关联仅剩 1 条 → 操作后主弹窗也一并关闭
 * - 否则只关二次弹窗，主弹窗内容自动刷新（已取消的那条随 preset_links 更新而消失）
 */
async function confirmUnlink() {
  const lp = confirmTarget.value
  const r = props.resource
  if (!lp || !r) return
  const wasLastOne = linkedPresets.value.length <= 1
  submitting.value = true
  try {
    await presetStore.unlinkResources(lp.id, [r.id])
    ElMessage.success('已取消关联')
    confirmVisible.value = false
    confirmTarget.value = null
    emit('unlinked')
    if (wasLastOne) emit('update:visible', false)
  } catch (e: any) {
    ElMessage.error(e?.message || '取消关联失败')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <el-dialog
    v-model="dialogVisible"
    width="540px"
    align-center
    class="resource-links-dialog"
  >
    <template #header>
      <div class="flex items-center gap-2">
        <span class="text-lg leading-none">🔗</span>
        <div class="min-w-0">
          <div class="text-base font-semibold text-gray-800 dark:text-gray-100 truncate">
            关联的 Preset
          </div>
          <div class="text-xs text-gray-400 truncate">{{ resource?.name }}</div>
        </div>
      </div>
    </template>

    <div v-if="linkedPresets.length === 0" class="text-sm text-gray-400 py-10 text-center">
      该资源未被任何 Preset 关联
    </div>
    <div v-else class="flex flex-col gap-3 max-h-[58vh] overflow-y-auto px-0.5 py-1">
      <div
        v-for="lp in linkedPresets"
        :key="lp.id"
        class="rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800/40 overflow-hidden hover:border-gray-300 dark:hover:border-gray-600 transition-colors"
      >
        <!-- preset 标题行 -->
        <div class="flex items-center justify-between gap-2 px-4 py-2.5 bg-gray-50/80 dark:bg-gray-900/30 border-b border-gray-100 dark:border-gray-700/60">
          <div class="flex items-center gap-2 min-w-0">
            <span class="inline-flex items-center justify-center w-6 h-6 rounded-md bg-blue-100 dark:bg-blue-900/40 text-blue-600 dark:text-blue-300 text-xs font-semibold flex-shrink-0">P</span>
            <span class="font-medium text-gray-800 dark:text-gray-100 truncate">{{ lp.name }}</span>
          </div>
          <el-button
            type="danger"
            size="small"
            plain
            class="flex-shrink-0"
            @click="handleUnlinkClick(lp)"
          >取消关联</el-button>
        </div>

        <!-- 部署的路径组列表 -->
        <div class="pl-9 pr-4 py-2.5">
          <div v-if="lp.deployedGroups.length > 0" class="flex flex-col gap-1.5 border-l-2 border-gray-100 dark:border-gray-700/60 pl-3">
            <div
              v-for="dg in lp.deployedGroups"
              :key="dg.group.id"
              class="flex items-center gap-2 text-sm"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-gray-400 flex-shrink-0" viewBox="0 0 20 20" fill="currentColor">
                <path d="M2 6a2 2 0 012-2h4l2 2h6a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z" />
              </svg>
              <span class="text-gray-700 dark:text-gray-300 truncate">{{ dg.group.name }}</span>
              <span
                v-if="dg.tracking"
                class="inline-flex items-center gap-0.5 text-[11px] px-1.5 py-0.5 rounded-full bg-green-50 dark:bg-green-900/30 text-green-600 dark:text-green-400 flex-shrink-0"
                title="跟踪变更"
              >🔗 跟踪</span>
            </div>
          </div>
          <div v-else class="flex items-center gap-1.5 text-xs text-gray-400">
            <span class="w-1.5 h-1.5 rounded-full bg-gray-300 dark:bg-gray-600"></span>
            尚未部署
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <el-button @click="dialogVisible = false">关闭</el-button>
    </template>
  </el-dialog>

  <!-- 二次确认：取消关联会变更 track=1 路径组的已部署内容 -->
  <el-dialog
    v-model="confirmVisible"
    title="确认取消关联"
    width="500px"
    align-center
    append-to-body
  >
    <div class="flex flex-col gap-3 text-sm">
      <p class="text-gray-700 dark:text-gray-200 leading-relaxed">
        确定要从 Preset「<span class="font-medium text-gray-900 dark:text-gray-100">{{ confirmTarget?.name }}</span>」取消关联
        「<span class="font-medium text-gray-900 dark:text-gray-100">{{ resource?.name }}</span>」吗？
      </p>

      <template v-if="affectedGroups.length > 0">
        <div class="rounded-lg border border-orange-200 dark:border-orange-900/50 bg-orange-50 dark:bg-orange-900/20 p-3">
          <p class="text-orange-600 dark:text-orange-400 font-medium mb-2">
            ⚠️ 以下开启「跟踪变更」的路径组，取消关联后会移除已部署的内容：
          </p>
          <div class="space-y-2.5">
            <div v-for="ag in affectedGroups" :key="ag.groupName">
              <div class="flex items-center gap-1.5 text-gray-800 dark:text-gray-100 font-medium">
                <span>📁 {{ ag.groupName }}</span>
                <span class="text-[11px] text-green-600 dark:text-green-400" title="跟踪变更">🔗</span>
              </div>
              <ul class="mt-1 pl-5 space-y-1">
                <li
                  v-for="sp in ag.subPaths"
                  :key="sp.path"
                  class="flex items-baseline gap-2 text-xs"
                >
                  <span class="px-1.5 py-0.5 rounded bg-gray-200/70 dark:bg-gray-700 text-gray-600 dark:text-gray-300 flex-shrink-0">{{ sp.label }}</span>
                  <span class="text-gray-500 dark:text-gray-400 break-all">{{ sp.path }}</span>
                </li>
              </ul>
            </div>
          </div>
        </div>
      </template>
      <p v-else class="text-gray-500 dark:text-gray-400">
        该 Preset 的部署均未开启「跟踪变更」，取消关联不会改变已部署内容。
      </p>
    </div>

    <template #footer>
      <el-button @click="confirmVisible = false">取消</el-button>
      <el-button type="danger" :loading="submitting" @click="confirmUnlink">确认取消关联</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.resource-links-dialog :deep(.el-dialog__body) {
  padding-top: 8px;
}
</style>
