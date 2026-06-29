<script setup lang="ts">
/** Preset 部署管理对话框（按路径组维度）— 展示该路径组下各类型资源的部署状态，支持重新部署。
 *  config 多路径：每条 config 路径一行；未分配的 config 单列一行；可为每个 config 重新选择目标路径。 */
import { ref, computed, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getPresetGroupStatus, redeployPresetGroup } from '@/api/preset'
import type { PresetGroupStatus } from '@/types/deploy'

const props = defineProps<{
  visible: boolean
  presetID: string
  presetName: string
  groupID: string
  groupName: string
}>()

const emit = defineEmits<{
  (e: 'update:visible', val: boolean): void
  (e: 'changed'): void
}>()

const dialogVisible = computed({
  get: () => props.visible,
  set: (val) => emit('update:visible', val),
})

const status = ref<PresetGroupStatus | null>(null)
const loading = ref(false)
const redeploying = ref(false)
/** config 资源 → 目标路径的（重新）分配选择 */
const configAssign = ref<Record<string, string>>({})

/** 打开时拉取该路径组的部署状态 */
watch(
  () => [props.visible, props.presetID, props.groupID],
  ([vis]) => {
    if (vis && props.presetID && props.groupID) load()
  },
  { immediate: true },
)

async function load() {
  loading.value = true
  try {
    status.value = await getPresetGroupStatus(props.presetID, props.groupID)
    // 初始化 config 分配：已部署的用 current_path，未分配的留空
    const assign: Record<string, string> = {}
    for (const t of status.value.targets) {
      if (t.type !== 'config') continue
      for (const r of t.resources) {
        if (r.stale) continue
        assign[r.resource_id] = r.current_path || t.target_path || ''
      }
    }
    configAssign.value = assign
  } catch (e: any) {
    ElMessage.error(e?.message || '加载部署状态失败')
    status.value = null
  } finally {
    loading.value = false
  }
}

/** 类型中文 */
function typeLabel(t: string) {
  const map: Record<string, string> = { skill: '技能', agent: '子代理', config: '配置', prompt: '提示词' }
  return map[t] || t
}

/** 部署类型中文 */
function deployTypeLabel(t: string) {
  return t === 'symlink' ? '软链接' : t === 'merge' ? '合并' : t
}

/** 该路径组所有 config 目标路径（用于重新分配下拉） */
const configPaths = computed<string[]>(() => {
  if (!status.value) return []
  const set = new Set<string>()
  for (const t of status.value.targets) {
    if (t.type === 'config' && t.target_path) set.add(t.target_path)
  }
  return [...set]
})

/** 是否存在多条 config 路径（决定是否显示重新分配下拉） */
const multiConfig = computed(() => configPaths.value.length >= 2)

/** 是否有 config 缺目标路径（任一 config 未选目标即为真，与路径条数无关） */
const hasUnassignedConfig = computed(() => {
  if (!status.value) return false
  // 收集所有非残留的 config 资源 ID
  const ids = new Set<string>()
  for (const t of status.value.targets) {
    if (t.type !== 'config') continue
    for (const r of t.resources) {
      if (!r.stale) ids.add(r.resource_id)
    }
  }
  for (const id of ids) {
    if (!configAssign.value[id]) return true
  }
  return false
})

/** 漂移总数 */
const driftTotal = computed(() => (status.value ? status.value.pending + status.value.stale : 0))

/** 路径展示截断 */
function pathLabel(p: string): string {
  if (p.length <= 40) return p
  return p.slice(0, 20) + '…' + p.slice(-18)
}

/** 重新部署该路径组（以最新全量资源，补齐新增类型 + 应用 config 重新分配） */
async function handleRedeploy() {
  // 有 config 未选目标 → 拦截
  if (hasUnassignedConfig.value) {
    ElMessage.warning('请为缺目标的 config 选择路径')
    return
  }
  const msg = driftTotal.value > 0
    ? `检测到 ${driftTotal.value} 项资源未同步（新增未部署 / 残留）。将以 Preset 最新全量资源重新部署到路径组「${props.groupName}」，确定继续？`
    : `将以最新内容重新部署到路径组「${props.groupName}」，确定继续？`
  try {
    await ElMessageBox.confirm(msg, '重新部署', {
      confirmButtonText: '重新部署',
      cancelButtonText: '取消',
      type: 'warning',
    })
  } catch {
    return
  }
  redeploying.value = true
  try {
    await redeployPresetGroup(props.presetID, props.groupID, configAssign.value)
    ElMessage.success('重新部署完成')
    emit('changed')
    dialogVisible.value = false
  } catch (e: any) {
    ElMessage.error(e?.message || '重新部署失败')
  } finally {
    redeploying.value = false
  }
}

function handleClose() {
  dialogVisible.value = false
}
</script>

<template>
  <el-dialog
    v-model="dialogVisible"
    :title="`部署管理：${presetName} → ${groupName}`"
    width="720px"
    :close-on-click-modal="false"
  >
    <div class="flex flex-col gap-3">
      <div v-if="loading" class="text-center text-sm text-gray-400 py-8">加载中...</div>

      <template v-else-if="status">
        <!-- 漂移提示 -->
        <div
          v-if="driftTotal > 0"
          class="flex items-center gap-2 px-3.5 py-2.5 rounded-md bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 text-sm text-amber-700 dark:text-amber-300"
        >
          <span>⚠️</span>
          <span class="flex-1">
            该 Preset 的资源已变更，有
            <b v-if="status.pending > 0">{{ status.pending }} 项待部署</b><span v-if="status.pending > 0 && status.stale > 0">、</span><b v-if="status.stale > 0">{{ status.stale }} 项待清理</b>。点击「重新部署」即可同步。
          </span>
        </div>
        <div
          v-else
          class="flex items-center gap-2 px-3.5 py-2.5 rounded-md bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 text-sm text-green-700 dark:text-green-300"
        >
          <span>✓</span>
          <span>部署内容与 Preset 当前资源一致。</span>
        </div>

        <!-- 各类型目标（中间区固定高度，超出滚动查看全部） -->
        <div class="h-[52vh] overflow-y-auto flex flex-col gap-3 pr-1">
          <div
            v-for="(t, ti) in status.targets"
            :key="t.type + '|' + t.target_path + '|' + ti"
            class="shrink-0 rounded-lg ring-1 ring-gray-200 dark:ring-gray-700 overflow-hidden"
          >
            <!-- 类型标题行 -->
            <div class="flex items-center gap-2 px-3.5 py-2.5 bg-gray-50 dark:bg-gray-800/60">
              <span class="text-sm font-medium text-gray-700 dark:text-gray-200">{{ typeLabel(t.type) }}</span>
              <el-tag
                v-if="t.has_deployment"
                size="small"
                :type="t.deploy_type === 'symlink' ? 'primary' : 'warning'"
              >{{ deployTypeLabel(t.deploy_type) }}</el-tag>
              <el-tag
                v-if="t.has_deployment"
                size="small"
                :type="t.track === 1 ? 'success' : 'info'"
              >{{ t.track === 1 ? '跟踪' : '静态' }}</el-tag>
              <span
                v-else-if="t.target_path"
                class="text-xs px-2 py-0.5 rounded bg-amber-100 dark:bg-amber-900/40 text-amber-600 dark:text-amber-400"
              >未部署</span>
              <span
                v-else
                class="text-xs px-2 py-0.5 rounded bg-blue-100 dark:bg-blue-900/40 text-blue-600 dark:text-blue-400"
              >待分配</span>
              <span class="flex-1"></span>
              <span class="text-xs text-gray-400 truncate max-w-[220px]" :title="t.target_path">{{ t.target_path || '（未指定路径）' }}</span>
            </div>
            <!-- 资源列表 -->
            <div class="px-3.5 py-2.5">
              <div v-if="t.resources.length === 0" class="text-sm text-gray-400 py-1.5">该类型在 Preset 中暂无资源</div>
              <div v-else class="flex flex-col gap-2">
                <div
                  v-for="r in t.resources"
                  :key="r.resource_id + '|' + t.target_path"
                  class="flex items-center gap-2.5 text-sm py-0.5"
                >
                  <span
                    class="w-2 h-2 rounded-full flex-shrink-0"
                    :class="r.stale ? 'bg-rose-400' : r.deployed ? 'bg-green-400' : 'bg-amber-400'"
                  ></span>
                  <span class="text-gray-700 dark:text-gray-300 truncate">{{ r.resource_name }}</span>
                  <span class="flex-1"></span>
                  <!-- config 目标分配下拉：多路径可重选，或待分配行(缺目标)必须选；残留项不可改 -->
                  <el-select
                    v-if="t.type === 'config' && !r.stale && (multiConfig || !t.target_path)"
                    v-model="configAssign[r.resource_id]"
                    size="small"
                    placeholder="选择目标"
                    class="!w-44 flex-shrink-0"
                  >
                    <el-option
                      v-for="p in configPaths"
                      :key="p"
                      :label="pathLabel(p)"
                      :value="p"
                      :title="p"
                    />
                  </el-select>
                  <span v-if="r.stale" class="text-xs text-rose-500 flex-shrink-0">残留（重新部署将清理）</span>
                  <span v-else-if="!r.deployed" class="text-xs text-amber-500 flex-shrink-0">新增（重新部署将补齐）</span>
                  <span v-else class="text-xs text-green-500 flex-shrink-0">已部署</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </template>

      <div v-else class="text-center text-sm text-gray-400 py-8">无部署信息</div>
    </div>

    <template #footer>
      <el-button @click="handleClose">关闭</el-button>
      <span v-if="hasUnassignedConfig" class="text-xs text-amber-500 mr-3">
        请为缺目标的 config 选择路径后再重新部署
      </span>
      <el-button
        type="primary"
        :loading="redeploying"
        :disabled="!status || status.targets.length === 0 || hasUnassignedConfig"
        @click="handleRedeploy"
      >
        {{ driftTotal > 0 ? '重新部署（同步）' : '重新部署' }}
      </el-button>
    </template>
  </el-dialog>
</template>
