<script setup lang="ts">
/** Preset 部署对话框 — 选已有路径组 / 手动填写 */
import { ref, computed, watch, reactive } from 'vue'
import { ElMessage } from 'element-plus'
import { usePathGroupStore } from '@/stores/pathGroup'
import { deployPreset } from '@/api/preset'
import { checkConflicts } from '@/api/deploy'
import { validatePathGroupPaths, genRandomGroupName } from '@/utils/pathFormat'
import type { PresetResource } from '@/types/preset'
import type { ResourceType } from '@/types/resource'
import type { Deployment } from '@/types/deploy'
import PresetDeployConflictDialog from './PresetDeployConflictDialog.vue'

const props = defineProps<{
  visible: boolean
  presetID: string
  presetName: string
  /** preset 当前的资源，用于推断需要哪些子路径 */
  resources: PresetResource[]
  /** preset 当前已有的部署，用于过滤掉已部署过的路径组 */
  deployments?: Deployment[]
  /** 预选路径组 ID */
  preselectGroupID?: string
}>()

const emit = defineEmits<{
  (e: 'update:visible', val: boolean): void
  (e: 'success'): void
  (e: 'deployed', deployments: Deployment[]): void
}>()

const dialogVisible = computed({
  get: () => props.visible,
  set: (val) => emit('update:visible', val),
})

const pathGroupStore = usePathGroupStore()

const mode = ref<'group' | 'manual'>('group')
const selectedGroupID = ref<string>('')
const manualPaths = reactive({
  skill_path: '',
  agent_path: '',
  config_path: '',
  prompt_path: '',
})
const track = ref(false)
const submitting = ref(false)
// 手动模式：是否绑定别名（自定义路径组名）。关=用随机组名
const bindAlias = ref(false)
// 手动模式：绑定别名时的路径组名称
const aliasName = ref('')
// 部署冲突预检弹窗
const conflictVisible = ref(false)
const conflictList = ref<{ type: string; targetPath: string; conflictWith: string; resourceName: string }[]>([])
// 预检通过后待执行的部署参数(避免重复构造)
const pendingDeploySpec = ref<{ path_group_id?: string; manual_paths?: any } | null>(null)

/** preset 实际包含哪些类型的资源 */
const presentTypes = computed<Set<ResourceType>>(() => {
  const s = new Set<ResourceType>()
  for (const r of props.resources) s.add(r.type)
  return s
})

/** 选中的路径组对象 */
const selectedGroup = computed(() =>
  pathGroupStore.pathGroups.find((g) => g.id === selectedGroupID.value),
)

/** 该 preset 已部署到的路径组 ID 集合
 *
 * 判定方式与侧边栏一致：deployment.target_path 命中某路径组的任一子路径，
 * 即视为该 preset 已部署在这个路径组下。
 */
const deployedGroupIDs = computed<Set<string>>(() => {
  const ids = new Set<string>()
  const deps = props.deployments || []
  if (deps.length === 0) return ids
  const targetPaths = new Set(deps.map((d) => d.target_path))
  for (const g of pathGroupStore.pathGroups) {
    const groupPaths = [g.skill_path, g.agent_path, g.config_path, g.prompt_path].filter(Boolean)
    if (groupPaths.some((p) => targetPaths.has(p))) ids.add(g.id)
  }
  return ids
})

/** 下拉可选路径组：排除该 preset 已部署过的组 */
const availableGroups = computed(() =>
  pathGroupStore.pathGroups.filter((g) => !deployedGroupIDs.value.has(g.id)),
)

/** 类型中文名映射 */
const TYPE_LABEL: Record<string, string> = {
  skill: 'Skill',
  agent: 'SubAgent',
  config: 'Config',
  prompt: 'Prompt',
}

/** 路径组模式下缺失的子路径类型 */
const missingTypes = computed<string[]>(() => {
  if (mode.value !== 'group' || !selectedGroup.value) return []
  const g = selectedGroup.value
  const missing: string[] = []
  if (presentTypes.value.has('skill') && !g.skill_path?.trim()) missing.push('skill')
  if (presentTypes.value.has('agent') && !g.agent_path?.trim()) missing.push('agent')
  if (presentTypes.value.has('config') && !g.config_path?.trim()) missing.push('config')
  if (presentTypes.value.has('prompt') && !g.prompt_path?.trim()) missing.push('prompt')
  return missing
})

/** 匹配提示文字和样式 */
const matchHint = computed(() => {
  if (mode.value !== 'group' || !selectedGroup.value) return null
  if (missingTypes.value.length === 0) {
    return { text: '路径组与 Preset 资源类型匹配', cls: 'text-green-600 dark:text-green-400' }
  }
  const labels = missingTypes.value.map((t) => TYPE_LABEL[t] || t).join('、')
  return { text: `子路径缺失：${labels}`, cls: 'text-orange-500 dark:text-orange-400' }
})

watch(
  () => props.visible,
  async (val) => {
    if (!val) return
    mode.value = 'group'
    selectedGroupID.value = props.preselectGroupID || ''
    manualPaths.skill_path = ''
    manualPaths.agent_path = ''
    manualPaths.config_path = ''
    manualPaths.prompt_path = ''
    track.value = false
    bindAlias.value = false
    aliasName.value = ''
    await pathGroupStore.fetchPathGroups()
    if (props.preselectGroupID) {
      selectedGroupID.value = props.preselectGroupID
    }
  },
)

async function handleConfirm() {
  // 校验
  if (mode.value === 'group') {
    if (!selectedGroupID.value) {
      ElMessage.warning('请选择路径组')
      return
    }
    const g = selectedGroup.value
    if (!g) return
    // 校验 preset 涉及的类型在路径组中都有非空路径
    const missing: string[] = []
    if (presentTypes.value.has('skill') && !g.skill_path.trim()) missing.push('skill_path')
    if (presentTypes.value.has('agent') && !g.agent_path.trim()) missing.push('agent_path')
    if (presentTypes.value.has('config') && !g.config_path.trim()) missing.push('config_path')
    if (presentTypes.value.has('prompt') && !g.prompt_path.trim()) missing.push('prompt_path')
    if (missing.length > 0) {
      ElMessage.warning(`所选路径组缺少: ${missing.join(', ')}`)
      return
    }
  } else {
    // 仅校验 preset 包含类型对应的字段
    const candidate = {
      skill_path: presentTypes.value.has('skill') ? manualPaths.skill_path : '',
      agent_path: presentTypes.value.has('agent') ? manualPaths.agent_path : '',
      config_path: presentTypes.value.has('config') ? manualPaths.config_path : '',
      prompt_path: presentTypes.value.has('prompt') ? manualPaths.prompt_path : '',
    }
    const err = validatePathGroupPaths(candidate)
    if (err) {
      ElMessage.warning(err)
      return
    }
    // 每个所需类型必须有值
    const missing: string[] = []
    for (const t of presentTypes.value) {
      const key = (t + '_path') as keyof typeof candidate
      if (!candidate[key].trim()) missing.push(key)
    }
    if (missing.length > 0) {
      ElMessage.warning(`手动模式下缺少: ${missing.join(', ')}`)
      return
    }
    // 绑定别名开启时，路径组名称必填
    if (bindAlias.value && !aliasName.value.trim()) {
      ElMessage.warning('请输入路径组名称，或关闭「绑定别名」使用随机组名')
      return
    }
  }

  // 构造部署参数
  const deployReq = {
    path_group_id: mode.value === 'group' ? selectedGroupID.value : undefined,
    manual_paths:
      mode.value === 'manual'
        ? {
            skill_path: presentTypes.value.has('skill') ? manualPaths.skill_path.trim() : undefined,
            agent_path: presentTypes.value.has('agent') ? manualPaths.agent_path.trim() : undefined,
            config_path: presentTypes.value.has('config') ? manualPaths.config_path.trim() : undefined,
            prompt_path: presentTypes.value.has('prompt') ? manualPaths.prompt_path.trim() : undefined,
          }
        : undefined,
    track: track.value,
  }

  // 预检 config / prompt 与目标已有内容的冲突(skill/agent 为 symlink,force 覆盖无预检意义)
  const conflicts = await runConflictCheck(deployReq)
  if (conflicts.length > 0) {
    conflictList.value = conflicts
    pendingDeploySpec.value = deployReq
    conflictVisible.value = true
    return
  }

  await doDeploy(deployReq)
}

/** 解析某类型对应的目标路径 */
function resolveTargetPath(deployReq: { path_group_id?: string; manual_paths?: any }, type: ResourceType): string {
  if (deployReq.path_group_id) {
    const g = selectedGroup.value
    if (!g) return ''
    const map: Record<string, string> = { skill: g.skill_path, agent: g.agent_path, config: g.config_path, prompt: g.prompt_path }
    return (map[type] || '').trim()
  }
  const mp = deployReq.manual_paths || {}
  const key = `${type}_path` as keyof typeof mp
  return (mp[key] || '').trim()
}

/** 预检 config/prompt 资源与目标已有内容冲突,返回需展示的冲突子项
 *
 * 每个类型一次批量预检(传该类型全部资源 id);后端用 conflict_for 字段
 * 把每条 existing 冲突精确归属到「本 preset 中的哪个 config/prompt」。
 * 产出结构: { type, targetPath, conflictWith(冲突对象), resourceName(本 preset 触发冲突的资源名) }
 */
async function runConflictCheck(deployReq: { path_group_id?: string; manual_paths?: any }) {
  const byType = new Map<ResourceType, string[]>()
  for (const r of props.resources) {
    if (r.type !== 'config' && r.type !== 'prompt') continue
    if (!byType.has(r.type)) byType.set(r.type, [])
    byType.get(r.type)!.push(r.id)
  }

  const conflicts: { type: string; targetPath: string; conflictWith: string; resourceName: string }[] = []
  for (const [type, ids] of byType) {
    if (ids.length === 0) continue
    const targetPath = resolveTargetPath(deployReq, type)
    if (!targetPath) continue
    try {
      const resp = await checkConflicts({ resource_ids: ids, target_path: targetPath })
      if (!resp.has_conflict) continue
      for (const c of resp.conflicts) {
        // 仅 existing(与目标已有内容/已部署资源冲突)需展示; applied/ignored 不展示
        if (c.status !== 'existing') continue
        // prompt 的 existing 表示该 prompt 自身已部署在目标里(将被覆盖),冲突对象记为「已部署内容」;
        // config 用后端 conflict_for 归属到具体资源,冲突对象是 resource_name(原始内容/他人资源)
        const conflictWith = type === 'prompt' ? '已部署内容' : c.resource_name
        const resourceName = c.conflict_for || c.resource_name
        conflicts.push({ type, targetPath, conflictWith, resourceName })
      }
    } catch {
      // 预检失败不阻塞部署,交由部署本身报错
    }
  }
  return conflicts
}

/** 冲突弹窗确认后继续部署 */
async function handleConflictConfirm() {
  if (!pendingDeploySpec.value) return
  await doDeploy(pendingDeploySpec.value)
  pendingDeploySpec.value = null
}

/** 执行部署
 *
 * 手动模式：先用填写的路径创建一个路径组（绑定别名→自定义名，否则随机名），
 * 再按 path_group_id 部署。这样部署一定挂在某个路径组下，侧边栏可正常显示。
 */
async function doDeploy(deployReq: { path_group_id?: string; manual_paths?: any; track?: boolean }) {
  submitting.value = true
  try {
    let groupID = deployReq.path_group_id
    // 手动模式：先建路径组
    if (!groupID && deployReq.manual_paths) {
      const mp = deployReq.manual_paths
      const name = bindAlias.value ? aliasName.value.trim() : genRandomGroupName()
      const g = await pathGroupStore.createPathGroup({
        name,
        skill_path: mp.skill_path || undefined,
        agent_path: mp.agent_path || undefined,
        config_path: mp.config_path || undefined,
        prompt_path: mp.prompt_path || undefined,
      })
      groupID = g.id
    }

    const deployments = await deployPreset(props.presetID, {
      path_group_id: groupID,
      track: track.value,
    })
    ElMessage.success('部署成功')
    emit('success')
    emit('deployed', deployments)
    emit('update:visible', false)
  } catch (e: any) {
    ElMessage.error(e?.message || '部署失败')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <el-dialog
    v-model="dialogVisible"
    :title="`部署 Preset：${presetName}`"
    width="560px"
    :close-on-click-modal="false"
  >
    <div v-if="presentTypes.size === 0" class="text-sm text-gray-500 py-4">
      该 Preset 还没有资源，无法部署
    </div>
    <div v-else class="flex flex-col gap-4">
      <!-- 模式切换 -->
      <el-radio-group v-model="mode">
        <el-radio value="group">使用已有路径组</el-radio>
        <el-radio value="manual">手动填写路径</el-radio>
      </el-radio-group>

      <!-- 路径组模式 -->
      <template v-if="mode === 'group'">
        <el-select
          v-model="selectedGroupID"
          placeholder="选择路径组"
          class="w-full"
          :loading="pathGroupStore.loading"
        >
          <el-option
            v-for="g in availableGroups"
            :key="g.id"
            :label="g.name"
            :value="g.id"
          />
        </el-select>
        <p
          v-if="availableGroups.length === 0"
          class="text-xs text-orange-500 dark:text-orange-400"
        >
          该 Preset 已部署到全部路径组，可改用「手动填写路径」新建部署
        </p>
        <div
          v-if="selectedGroup"
          class="text-xs space-y-1 bg-gray-50 dark:bg-gray-800/50 rounded p-3 border border-gray-200 dark:border-gray-700"
        >
          <div v-if="selectedGroup.skill_path">
            <span class="text-gray-500">skill:</span>
            <span class="ml-2 text-gray-800 dark:text-gray-200">{{ selectedGroup.skill_path }}</span>
          </div>
          <div v-if="selectedGroup.agent_path">
            <span class="text-gray-500">agent:</span>
            <span class="ml-2 text-gray-800 dark:text-gray-200">{{ selectedGroup.agent_path }}</span>
          </div>
          <div v-if="selectedGroup.config_path">
            <span class="text-gray-500">config:</span>
            <span class="ml-2 text-gray-800 dark:text-gray-200">{{ selectedGroup.config_path }}</span>
          </div>
          <div v-if="selectedGroup.prompt_path">
            <span class="text-gray-500">prompt:</span>
            <span class="ml-2 text-gray-800 dark:text-gray-200">{{ selectedGroup.prompt_path }}</span>
          </div>
          <div v-if="matchHint" class="text-xs mt-1 font-medium" :class="matchHint.cls">
            {{ matchHint.text }}
          </div>
        </div>
      </template>

      <!-- 手动模式 -->
      <template v-else>
        <el-form label-position="top">
          <el-form-item v-if="presentTypes.has('skill')" label="Skill 路径">
            <el-input v-model="manualPaths.skill_path" placeholder="目录路径（支持 ~）" />
          </el-form-item>
          <el-form-item v-if="presentTypes.has('agent')" label="Agent 路径">
            <el-input v-model="manualPaths.agent_path" placeholder="目录路径（支持 ~）" />
          </el-form-item>
          <el-form-item v-if="presentTypes.has('config')" label="Config 路径">
            <el-input
              v-model="manualPaths.config_path"
              placeholder=".json/.jsonc/.yaml/.yml/.toml"
            />
          </el-form-item>
          <el-form-item v-if="presentTypes.has('prompt')" label="Prompt 路径">
            <el-input v-model="manualPaths.prompt_path" placeholder=".md 文件" />
          </el-form-item>
        </el-form>

        <!-- 绑定别名：开→自定义路径组名；关→随机组名 -->
        <div class="flex items-center gap-2">
          <el-switch v-model="bindAlias" />
          <span class="text-sm text-gray-600 dark:text-gray-400">绑定别名（自定义路径组名称）</span>
        </div>
        <el-input
          v-if="bindAlias"
          v-model="aliasName"
          placeholder="请输入路径组名称"
          maxlength="100"
        />
        <p v-else class="text-xs text-gray-400 dark:text-gray-500">
          将使用随机组名（部署后可在左侧路径组列表中查看）
        </p>
      </template>

      <!-- track -->
      <div class="flex items-center gap-2 pt-1">
        <el-switch v-model="track" />
        <span class="text-sm text-gray-600 dark:text-gray-400">跟踪变更（Preset 资源更新时同步到目标）</span>
      </div>
    </div>

    <template #footer>
      <el-button @click="dialogVisible = false">取消</el-button>
      <el-button
        type="primary"
        :loading="submitting"
        :disabled="presentTypes.size === 0 || (mode === 'group' && missingTypes.length > 0)"
        @click="handleConfirm"
      >部署</el-button>
    </template>
  </el-dialog>

  <!-- 部署冲突预检弹窗 -->
  <PresetDeployConflictDialog
    v-model:visible="conflictVisible"
    :preset-name="presetName"
    :conflicts="conflictList"
    @confirm="handleConflictConfirm"
  />
</template>
