<script setup lang="ts">
/** Config 目标路径分配弹窗
 *
 * 当路径组/手动填写存在多条 config 路径时，preset 内每个 config 资源需各自选定目标。
 * 已部署的 config 可回显并锁定其当前路径（locked=true），仅未分配的需要选择。
 */
import { ref, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'

interface ConfigItem {
  id: string
  name: string
  /** 已部署时的当前路径（回显） */
  currentPath?: string
  /** 是否锁定不可改（仅展示用，确认时仍带上其值） */
  locked?: boolean
}

const props = defineProps<{
  visible: boolean
  /** 待分配的 config 资源 */
  configs: ConfigItem[]
  /** 可选的 config 目标路径列表 */
  configPaths: string[]
}>()

const emit = defineEmits<{
  (e: 'update:visible', val: boolean): void
  /** 确认分配：resource_id → target_path */
  (e: 'confirm', assignments: Record<string, string>): void
}>()

const dialogVisible = computed({
  get: () => props.visible,
  set: (val) => emit('update:visible', val),
})

/** 当前选择：resource_id → target_path */
const selection = ref<Record<string, string>>({})

watch(
  () => props.visible,
  (val) => {
    if (!val) return
    const init: Record<string, string> = {}
    for (const c of props.configs) {
      // 已部署的回显当前路径；否则若仅一条路径默认选中，多条则留空待选
      if (c.currentPath) init[c.id] = c.currentPath
      else if (props.configPaths.length === 1) init[c.id] = props.configPaths[0]
      else init[c.id] = ''
    }
    selection.value = init
  },
)

/** 路径下拉展示：截断中间，保留首尾 */
function pathLabel(p: string): string {
  if (p.length <= 48) return p
  return p.slice(0, 24) + '…' + p.slice(-22)
}

const allAssigned = computed(() => props.configs.every((c) => !!selection.value[c.id]))

function handleConfirm() {
  if (!allAssigned.value) {
    ElMessage.warning('请为每个 config 选择目标路径')
    return
  }
  emit('confirm', { ...selection.value })
  emit('update:visible', false)
}
</script>

<template>
  <el-dialog
    v-model="dialogVisible"
    title="分配 Config 目标路径"
    width="600px"
    :close-on-click-modal="false"
    append-to-body
  >
    <p class="text-sm text-gray-500 dark:text-gray-400 mb-3">
      该路径组有多条 config 路径，请为每个 config 资源选择要部署到的目标文件。
    </p>
    <div class="flex flex-col gap-3 max-h-[50vh] overflow-y-auto pr-1">
      <div
        v-for="c in configs"
        :key="c.id"
        class="flex items-center gap-3"
      >
        <span class="w-40 shrink-0 truncate text-sm text-gray-700 dark:text-gray-200" :title="c.name">
          {{ c.name }}
          <span v-if="c.locked" class="text-[10px] text-gray-400 ml-1">已部署</span>
        </span>
        <el-select
          v-model="selection[c.id]"
          :disabled="c.locked"
          placeholder="选择目标路径"
          class="flex-1"
        >
          <el-option
            v-for="p in configPaths"
            :key="p"
            :label="pathLabel(p)"
            :value="p"
            :title="p"
          />
        </el-select>
      </div>
    </div>
    <template #footer>
      <el-button @click="dialogVisible = false">取消</el-button>
      <el-button type="primary" :disabled="!allAssigned" @click="handleConfirm">确认</el-button>
    </template>
  </el-dialog>
</template>
