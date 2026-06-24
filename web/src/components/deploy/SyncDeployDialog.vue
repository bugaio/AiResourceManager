<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { getResourceDeployTargets, deploy } from '@/api/deploy'
import type { ResourceDeployTarget } from '@/api/deploy'

/** 保存后同步部署弹窗 —— 按 preset 分组展示其部署子项 */
const props = defineProps<{
  visible: boolean
  resourceId: string
}>()

const emit = defineEmits<{
  (e: 'update:visible', val: boolean): void
  (e: 'synced'): void
}>()

const loading = ref(false)
const deploying = ref(false)
const targets = ref<ResourceDeployTarget[]>([])
const selected = ref<Set<string>>(new Set())

/** 按「路径组」分组(无匹配路径组归入「其他部署」) */
interface PathGroupBlock {
  key: string
  groupName: string
  items: ResourceDeployTarget[]
}

const groups = computed<PathGroupBlock[]>(() => {
  const map = new Map<string, PathGroupBlock>()
  for (const t of targets.value) {
    const name = t.path_group_name || t.alias_name || '其他部署'
    const key = name
    if (!map.has(key)) {
      map.set(key, { key, groupName: name, items: [] })
    }
    map.get(key)!.items.push(t)
  }
  return Array.from(map.values())
})

/** 本次更新的资源名(取首个 target 的资源名;同步弹窗针对单个被改资源) */
const resourceName = computed(() => targets.value[0]?.resource_names?.[0] || '')

/** 全选状态 */
const isAllSelected = computed(() =>
  targets.value.length > 0 && selected.value.size === targets.value.length
)
const isIndeterminate = computed(() =>
  selected.value.size > 0 && selected.value.size < targets.value.length
)

/** 打开弹窗时加载已部署路径 */
watch(() => props.visible, async (val) => {
  if (val && props.resourceId) {
    loading.value = true
    try {
      targets.value = await getResourceDeployTargets(props.resourceId)
      // 无冲突的默认勾选
      selected.value = new Set(
        targets.value.filter(t => !t.has_conflict).map(t => t.deployment_id)
      )
    } catch (e: any) {
      ElMessage.error(e?.message || '获取部署列表失败')
    } finally {
      loading.value = false
    }
  }
})

/** 切换单项勾选 */
function toggleItem(deploymentId: string) {
  const s = new Set(selected.value)
  if (s.has(deploymentId)) {
    s.delete(deploymentId)
  } else {
    s.add(deploymentId)
  }
  selected.value = s
}

/** 全选/取消全选 */
function handleSelectAll(val: boolean) {
  if (val) {
    selected.value = new Set(targets.value.map(t => t.deployment_id))
  } else {
    selected.value = new Set()
  }
}

/** 重新部署选中路径(用各子项自身的 resource_ids) */
async function handleRedeploy() {
  if (selected.value.size === 0) {
    ElMessage.warning('请至少选择一个路径')
    return
  }
  deploying.value = true
  try {
    for (const target of targets.value) {
      if (!selected.value.has(target.deployment_id)) continue
      await deploy({
        resource_ids: target.resource_ids,
        target_path: target.target_path,
        force: true,
        // 保持原部署的 preset 关联,避免同步后资源脱离 preset
        preset_id: target.preset_id || undefined,
      })
    }
    ElMessage.success('同步部署完成')
    emit('synced')
    emit('update:visible', false)
  } catch (e: any) {
    ElMessage.error(e?.message || '同步部署失败')
  } finally {
    deploying.value = false
  }
}

function handleClose() {
  emit('update:visible', false)
}

/** 子项资源名展示 */
function itemLabel(t: ResourceDeployTarget): string {
  return t.resource_names.join('、')
}
</script>

<template>
  <el-dialog
    :model-value="visible"
    title="同步已部署路径"
    width="560px"
    @close="handleClose"
    :close-on-click-modal="false"
  >
    <div v-loading="loading" class="flex flex-col gap-3">
      <p class="text-sm text-gray-600 dark:text-gray-400">
        该 Preset 下的
        <span class="font-medium text-gray-800 dark:text-gray-100">{{ resourceName }}</span>
        子项需要同步，勾选后将以最新内容重新部署：
      </p>

      <!-- 按路径组分组列表（可滚动） -->
      <div class="max-h-[340px] overflow-y-auto border rounded border-gray-200 dark:border-gray-700 divide-y divide-gray-100 dark:divide-gray-700">
        <div v-for="group in groups" :key="group.key" class="px-3 py-2">
          <!-- 主标题:路径组名称 -->
          <div class="text-sm font-medium text-gray-800 dark:text-gray-100 flex items-center gap-1">
            <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5 text-gray-400 flex-shrink-0" viewBox="0 0 20 20" fill="currentColor">
              <path d="M2 6a2 2 0 012-2h4l2 2h6a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z" />
            </svg>
            <span class="truncate">{{ group.groupName }}</span>
          </div>
          <!-- 子行:偏移显示资源名 + 部署子路径 -->
          <div
            v-for="t in group.items"
            :key="t.deployment_id"
            class="flex items-center justify-between pl-5 mt-1.5"
          >
            <div class="flex flex-col min-w-0 flex-1 mr-3">
              <span class="text-sm truncate text-gray-700 dark:text-gray-200" :title="itemLabel(t)">
                {{ itemLabel(t) }}
              </span>
              <span class="text-xs text-gray-400 dark:text-gray-500 truncate" :title="t.target_path">
                {{ t.target_path }}
              </span>
              <span v-if="t.has_conflict" class="text-xs text-amber-500">
                存在冲突（将以最新内容覆盖）
              </span>
            </div>
            <el-checkbox
              :model-value="selected.has(t.deployment_id)"
              @change="toggleItem(t.deployment_id)"
            />
          </div>
        </div>
        <div v-if="targets.length === 0 && !loading" class="px-3 py-4 text-center text-sm text-gray-400">
          该资源暂无部署记录
        </div>
      </div>

      <!-- 全选 -->
      <el-checkbox
        v-if="targets.length > 0"
        :model-value="isAllSelected"
        :indeterminate="isIndeterminate"
        @change="handleSelectAll"
      >
        全选
      </el-checkbox>
    </div>

    <template #footer>
      <el-button @click="handleClose">取消</el-button>
      <el-button type="primary" :loading="deploying" :disabled="selected.size === 0" @click="handleRedeploy">
        重新部署
      </el-button>
    </template>
  </el-dialog>
</template>
