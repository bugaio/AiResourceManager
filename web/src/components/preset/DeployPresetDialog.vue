<script setup lang="ts">
/** Preset 部署对话框 — 选已有路径组 / 手动填写（config 支持多路径 + 分配弹窗） */
import { ref, computed, watch, reactive } from 'vue'
import { ElMessage } from 'element-plus'
import { usePathGroupStore } from '@/stores/pathGroup'
import { deployPreset } from '@/api/preset'
import { checkConflicts } from '@/api/deploy'
import { genRandomGroupName, isConfigFilePath, isDirectoryPath, isPromptFilePath } from '@/utils/pathFormat'
import type { PresetResource } from '@/types/preset'
import type { ResourceType } from '@/types/resource'
import type { Deployment } from '@/types/deploy'
import PresetDeployConflictDialog from './PresetDeployConflictDialog.vue'
import ConfigAssignDialog from './ConfigAssignDialog.vue'

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
  prompt_path: '',
})
/** 手动模式 config 多路径 */
const manualConfigPaths = ref<string[]>([''])
const track = ref(false)
const submitting = ref(false)
// 手动模式：是否绑定别名（自定义路径组名）。关=用随机组名
const bindAlias = ref(false)
// 手动模式：绑定别名时的路径组名称
const aliasName = ref('')
// 部署冲突预检弹窗
const conflictVisible = ref(false)
const conflictList = ref<{ type: string; targetPath: string; conflictWith: string; resourceName: string }[]>([])
// 预检通过后待执行的部署参数
const pendingDeploySpec = ref<{ path_group_id?: string; manual_paths?: any; config_assignments?: Record<string, string> } | null>(null)
// config 分配弹窗
const assignVisible = ref(false)
/** 分配完成后的 config 资源 → 目标路径 */
const configAssignments = ref<Record<string, string>>({})

/** preset 实际包含哪些类型的资源 */
const presentTypes = computed<Set<ResourceType>>(() => {
  const s = new Set<ResourceType>()
  for (const r of props.resources) s.add(r.type)
  return s
})

/** preset 中的 config 资源 */
const configResources = computed(() => props.resources.filter((r) => r.type === 'config'))

/** 选中的路径组对象 */
const selectedGroup = computed(() =>
  pathGroupStore.pathGroups.find((g) => g.id === selectedGroupID.value),
)

/** 当前部署目标的 config 路径列表（路径组模式取 config_paths，手动模式取输入数组） */
const effectiveConfigPaths = computed<string[]>(() => {
  if (mode.value === 'group') {
    const g = selectedGroup.value
    if (!g) return []
    return (g.config_paths && g.config_paths.length > 0)
      ? g.config_paths
      : (g.config_path ? [g.config_path] : [])
  }
  return manualConfigPaths.value.map((p) => p.trim()).filter(Boolean)
})

/** 该 preset 已部署到的路径组 ID 集合 */
const deployedGroupIDs = computed<Set<string>>(() => {
  const ids = new Set<string>()
  const deps = props.deployments || []
  if (deps.length === 0) return ids
  const targetPaths = new Set(deps.map((d) => d.target_path))
  for (const g of pathGroupStore.pathGroups) {
    const groupPaths = [g.skill_path, g.agent_path, ...(g.config_paths || []), g.prompt_path].filter(Boolean)
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
  if (presentTypes.value.has('config') && effectiveConfigPaths.value.length === 0) missing.push('config')
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

/** config 资源数 ≥1 且目标 config 路径 ≥2 时，需要分配 */
const needAssign = computed(
  () => configResources.value.length > 0 && effectiveConfigPaths.value.length >= 2,
)

/** 手动模式 config 增删 */
function addManualConfig() {
  manualConfigPaths.value.push('')
}
function removeManualConfig(idx: number) {
  manualConfigPaths.value.splice(idx, 1)
  if (manualConfigPaths.value.length === 0) manualConfigPaths.value.push('')
}

watch(
  () => props.visible,
  async (val) => {
    if (!val) return
    mode.value = 'group'
    selectedGroupID.value = props.preselectGroupID || ''
    manualPaths.skill_path = ''
    manualPaths.agent_path = ''
    manualPaths.prompt_path = ''
    manualConfigPaths.value = ['']
    track.value = false
    bindAlias.value = false
    aliasName.value = ''
    configAssignments.value = {}
    await pathGroupStore.fetchPathGroups()
    if (props.preselectGroupID) {
      selectedGroupID.value = props.preselectGroupID
    }
  },
)

// 目标变化时清空已有分配（避免用旧目标的分配部署到新目标）
watch([selectedGroupID, manualConfigPaths, mode], () => {
  configAssignments.value = {}
}, { deep: true })

async function handleConfirm() {
  // 校验
  if (mode.value === 'group') {
    if (!selectedGroupID.value) {
      ElMessage.warning('请选择路径组')
      return
    }
    const g = selectedGroup.value
    if (!g) return
    const missing: string[] = []
    if (presentTypes.value.has('skill') && !g.skill_path.trim()) missing.push('skill_path')
    if (presentTypes.value.has('agent') && !g.agent_path.trim()) missing.push('agent_path')
    if (presentTypes.value.has('config') && effectiveConfigPaths.value.length === 0) missing.push('config_path')
    if (presentTypes.value.has('prompt') && !g.prompt_path.trim()) missing.push('prompt_path')
    if (missing.length > 0) {
      ElMessage.warning(`所选路径组缺少: ${missing.join(', ')}`)
      return
    }
  } else {
    // 手动模式校验
    const skill = manualPaths.skill_path.trim()
    const agent = manualPaths.agent_path.trim()
    const prompt = manualPaths.prompt_path.trim()
    const configs = effectiveConfigPaths.value
    if (presentTypes.value.has('skill') && !skill) { ElMessage.warning('请填写 Skill 路径'); return }
    if (presentTypes.value.has('agent') && !agent) { ElMessage.warning('请填写 Agent 路径'); return }
    if (presentTypes.value.has('prompt') && !prompt) { ElMessage.warning('请填写 Prompt 路径'); return }
    if (presentTypes.value.has('config') && configs.length === 0) { ElMessage.warning('请填写 Config 路径'); return }
    if (skill && !isDirectoryPath(skill)) { ElMessage.warning('Skill 路径必须是目录'); return }
    if (agent && !isDirectoryPath(agent)) { ElMessage.warning('Agent 路径必须是目录'); return }
    if (prompt && !isPromptFilePath(prompt)) { ElMessage.warning('Prompt 路径后缀必须是 .md'); return }
    for (const c of configs) {
      if (!isConfigFilePath(c)) { ElMessage.warning('Config 路径后缀必须是 .json/.jsonc/.yaml/.yml/.toml'); return }
    }
    if (new Set(configs).size !== configs.length) { ElMessage.warning('Config 路径有重复'); return }
    if (bindAlias.value && !aliasName.value.trim()) {
      ElMessage.warning('请输入路径组名称，或关闭「绑定别名」使用随机组名')
      return
    }
  }

  // config 多路径 → 先弹分配窗，让每个 config 选目标
  if (needAssign.value) {
    if (Object.keys(configAssignments.value).length === 0) {
      assignVisible.value = true
      return
    }
  } else if (effectiveConfigPaths.value.length === 1) {
    // 单条 config 路径：自动分配
    const only = effectiveConfigPaths.value[0]
    const map: Record<string, string> = {}
    for (const r of configResources.value) map[r.id] = only
    configAssignments.value = map
  }

  await proceedDeploy()
}

/** 分配弹窗确认回调 */
async function handleAssignConfirm(assignments: Record<string, string>) {
  configAssignments.value = assignments
  await proceedDeploy()
}

/** 构造部署参数 → 冲突预检 → 部署 */
async function proceedDeploy() {
  const deployReq = {
    path_group_id: mode.value === 'group' ? selectedGroupID.value : undefined,
    manual_paths:
      mode.value === 'manual'
        ? {
            skill_path: presentTypes.value.has('skill') ? manualPaths.skill_path.trim() : undefined,
            agent_path: presentTypes.value.has('agent') ? manualPaths.agent_path.trim() : undefined,
            config_paths: presentTypes.value.has('config') ? effectiveConfigPaths.value : undefined,
            prompt_path: presentTypes.value.has('prompt') ? manualPaths.prompt_path.trim() : undefined,
          }
        : undefined,
    track: track.value,
    config_assignments: configAssignments.value,
  }

  const conflicts = await runConflictCheck()
  if (conflicts.length > 0) {
    conflictList.value = conflicts
    pendingDeploySpec.value = deployReq
    conflictVisible.value = true
    return
  }

  await doDeploy(deployReq)
}

/** 预检 config/prompt 资源与目标已有内容冲突
 *
 * config 按各自分配的 target 分别预检；prompt 用单路径批量预检。
 */
async function runConflictCheck() {
  const conflicts: { type: string; targetPath: string; conflictWith: string; resourceName: string }[] = []

  // config：每个资源按分配的 target 单独预检
  for (const r of configResources.value) {
    const targetPath = configAssignments.value[r.id]
    if (!targetPath) continue
    try {
      const resp = await checkConflicts({ resource_ids: [r.id], target_path: targetPath })
      if (!resp.has_conflict) continue
      for (const c of resp.conflicts) {
        if (c.status !== 'existing') continue
        conflicts.push({
          type: 'config',
          targetPath,
          conflictWith: c.resource_name,
          resourceName: c.conflict_for || c.resource_name,
        })
      }
    } catch { /* 预检失败不阻塞 */ }
  }

  // prompt：单路径批量预检
  const promptIDs = props.resources.filter((r) => r.type === 'prompt').map((r) => r.id)
  if (promptIDs.length > 0) {
    const targetPath = mode.value === 'group'
      ? (selectedGroup.value?.prompt_path || '').trim()
      : manualPaths.prompt_path.trim()
    if (targetPath) {
      try {
        const resp = await checkConflicts({ resource_ids: promptIDs, target_path: targetPath })
        if (resp.has_conflict) {
          for (const c of resp.conflicts) {
            if (c.status !== 'existing') continue
            conflicts.push({ type: 'prompt', targetPath, conflictWith: '已部署内容', resourceName: c.conflict_for || c.resource_name })
          }
        }
      } catch { /* 预检失败不阻塞 */ }
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
async function doDeploy(deployReq: { path_group_id?: string; manual_paths?: any; track?: boolean; config_assignments?: Record<string, string> }) {
  submitting.value = true
  try {
    let groupID = deployReq.path_group_id
    if (!groupID && deployReq.manual_paths) {
      const mp = deployReq.manual_paths
      const name = bindAlias.value ? aliasName.value.trim() : genRandomGroupName()
      const g = await pathGroupStore.createPathGroup({
        name,
        skill_path: mp.skill_path || undefined,
        agent_path: mp.agent_path || undefined,
        config_paths: mp.config_paths || undefined,
        prompt_path: mp.prompt_path || undefined,
      })
      groupID = g.id
    }

    const deployments = await deployPreset(props.presetID, {
      path_group_id: groupID,
      track: track.value,
      config_assignments: configAssignments.value,
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
          <div v-if="effectiveConfigPaths.length > 0">
            <span class="text-gray-500 align-top">config:</span>
            <span class="ml-2 inline-flex flex-col gap-0.5">
              <span
                v-for="cp in effectiveConfigPaths"
                :key="cp"
                class="text-gray-800 dark:text-gray-200 break-all"
              >{{ cp }}</span>
            </span>
          </div>
          <div v-if="selectedGroup.prompt_path">
            <span class="text-gray-500">prompt:</span>
            <span class="ml-2 text-gray-800 dark:text-gray-200">{{ selectedGroup.prompt_path }}</span>
          </div>
          <div v-if="matchHint" class="text-xs mt-1 font-medium" :class="matchHint.cls">
            {{ matchHint.text }}
          </div>
          <div v-if="needAssign" class="text-xs mt-1 text-blue-500 dark:text-blue-400">
            该路径组有多条 config 路径，点击部署后将逐个分配
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
            <div class="w-full flex flex-col gap-2">
              <div
                v-for="(_, idx) in manualConfigPaths"
                :key="idx"
                class="flex items-center gap-2"
              >
                <el-input
                  v-model="manualConfigPaths[idx]"
                  placeholder=".json/.jsonc/.yaml/.yml/.toml"
                />
                <el-button
                  v-if="manualConfigPaths.length > 1"
                  text
                  class="!px-2 text-gray-400 hover:!text-red-500"
                  title="删除此路径"
                  @click="removeManualConfig(idx)"
                >✕</el-button>
                <el-button
                  v-if="idx === manualConfigPaths.length - 1"
                  text
                  type="primary"
                  class="!px-2"
                  title="添加一条 config 路径"
                  @click="addManualConfig"
                >＋</el-button>
              </div>
            </div>
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
      >{{ needAssign ? '下一步：分配 Config' : '部署' }}</el-button>
    </template>
  </el-dialog>

  <!-- 部署冲突预检弹窗 -->
  <PresetDeployConflictDialog
    v-model:visible="conflictVisible"
    :preset-name="presetName"
    :conflicts="conflictList"
    @confirm="handleConflictConfirm"
  />

  <!-- config 目标路径分配弹窗 -->
  <ConfigAssignDialog
    v-model:visible="assignVisible"
    :configs="configResources.map((r) => ({ id: r.id, name: r.name }))"
    :config-paths="effectiveConfigPaths"
    @confirm="handleAssignConfirm"
  />
</template>
