<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { getResourceDeployTargets, deploy } from '@/api/deploy'
import type { ResourceDeployTarget } from '@/api/deploy'

/** MCP 保存后同步部署弹窗 */
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

/** 重新部署选中路径 */
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
        resource_ids: [props.resourceId],
        target_path: target.target_path,
        force: true,
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
</script>

<template>
  <el-dialog
    :model-value="visible"
    title="同步已部署路径"
    width="520px"
    @close="handleClose"
    :close-on-click-modal="false"
  >
    <div v-loading="loading" class="flex flex-col gap-3">
      <p class="text-sm text-gray-600 dark:text-gray-400">
        以下路径已部署该 MCP 资源，勾选后将以最新内容重新部署：
      </p>

      <!-- 路径列表（可滚动） -->
      <div class="max-h-[300px] overflow-y-auto border rounded border-gray-200 dark:border-gray-700">
        <div
          v-for="target in targets"
          :key="target.deployment_id"
          class="flex items-center justify-between px-3 py-2 border-b last:border-b-0 border-gray-100 dark:border-gray-700"
        >
          <div class="flex flex-col min-w-0 flex-1 mr-3">
            <span class="text-sm truncate text-gray-800 dark:text-gray-200" :title="target.target_path">
              {{ target.alias_name || target.target_path }}
            </span>
            <span v-if="target.alias_name" class="text-xs text-gray-400 dark:text-gray-500 truncate" :title="target.target_path">
              {{ target.target_path }}
            </span>
            <span v-if="target.has_conflict" class="text-xs text-amber-500">
              存在冲突（将以最新内容覆盖）
            </span>
          </div>
          <el-checkbox
            :model-value="selected.has(target.deployment_id)"
            @change="toggleItem(target.deployment_id)"
          />
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
