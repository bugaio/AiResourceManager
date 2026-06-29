<script setup lang="ts">
/** 资源被 Preset 锁定时的删除确认对话框
 *
 * 展示该资源被哪些 preset 关联，以及每个 preset 部署到的路径组（track=1 显示 🔗）。
 * 确认后由父级执行「取消关联 + 删除」。
 */
import { computed } from 'vue'
import { usePresetStore } from '@/stores/preset'
import { usePathGroupStore } from '@/stores/pathGroup'
import type { PresetLinkInfo } from '@/types/preset'
import type { Deployment } from '@/types/deploy'
import type { PathGroup } from '@/types/pathGroup'

const props = defineProps<{
  visible: boolean
  resourceName: string
  presets: PresetLinkInfo[]
}>()

const emit = defineEmits<{
  (e: 'update:visible', val: boolean): void
  (e: 'confirm'): void
}>()

const presetStore = usePresetStore()
const pathGroupStore = usePathGroupStore()

const dialogVisible = computed({
  get: () => props.visible,
  set: (val) => emit('update:visible', val),
})

/** 一条部署在某路径组下的归属信息 */
interface DeployedGroupInfo {
  group: PathGroup
  tracking: boolean
}

/** 关联的 preset（含部署路径组） */
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
      [g.skill_path, g.agent_path, ...(g.config_paths || []), g.prompt_path].filter(Boolean),
    )
    const matched = deployments.filter((d) => groupPaths.has(d.target_path))
    if (matched.length === 0) continue
    result.push({ group: g, tracking: matched.some((d) => d.track === 1) })
  }
  return result
}

/** 关联此资源的 preset 列表（用传入的 presets 的 id 去 store 取 deployments） */
const linkedPresets = computed<LinkedPreset[]>(() =>
  props.presets.map((link) => {
    const full = presetStore.presets.find((p) => p.id === link.id)
    return {
      id: link.id,
      name: link.name,
      deployedGroups: groupDeployments(full?.deployments || []),
    }
  }),
)

function handleConfirm() {
  emit('confirm')
}
</script>

<template>
  <el-dialog
    v-model="dialogVisible"
    width="520px"
    align-center
    :close-on-click-modal="false"
  >
    <template #header>
      <div class="flex items-center gap-2">
        <span class="text-lg leading-none">🔗</span>
        <div class="min-w-0">
          <div class="text-base font-semibold text-gray-800 dark:text-gray-100 truncate">
            资源被以下 Preset 管理
          </div>
          <div class="text-xs text-gray-400 truncate">{{ resourceName }}</div>
        </div>
      </div>
    </template>

    <div
      v-if="linkedPresets.length === 0"
      class="text-sm text-gray-400 py-10 text-center"
    >
      未提供 Preset 列表
    </div>
    <div v-else class="flex flex-col gap-3 max-h-[55vh] overflow-y-auto px-0.5 py-1">
      <div
        v-for="lp in linkedPresets"
        :key="lp.id"
        class="rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800/40 overflow-hidden"
      >
        <!-- preset 标题行 -->
        <div class="flex items-center gap-2 px-4 py-2.5 bg-gray-50/80 dark:bg-gray-900/30 border-b border-gray-100 dark:border-gray-700/60">
          <span class="inline-flex items-center justify-center w-6 h-6 rounded-md bg-blue-100 dark:bg-blue-900/40 text-blue-600 dark:text-blue-300 text-xs font-semibold flex-shrink-0">P</span>
          <span class="font-medium text-gray-800 dark:text-gray-100 truncate">{{ lp.name }}</span>
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

    <div class="text-xs text-amber-600 dark:text-amber-400 mt-3">
      取消关联后，开启「跟踪变更」🔗 的路径组会同步移除已部署内容
    </div>

    <template #footer>
      <el-button @click="dialogVisible = false">取消</el-button>
      <el-button type="danger" @click="handleConfirm">取消关联并删除</el-button>
    </template>
  </el-dialog>
</template>
