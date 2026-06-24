<script setup lang="ts">
/** Preset 部署预检冲突弹窗 —— 按资源类型(skill/config/subagent/prompt)分组展示冲突 */
import { computed } from 'vue'

const props = defineProps<{
  visible: boolean
  presetName: string
  conflicts: { type: string; targetPath: string; conflictWith: string; resourceName: string }[]
}>()

const emit = defineEmits<{
  (e: 'update:visible', val: boolean): void
  (e: 'confirm'): void
}>()

const TYPE_LABEL: Record<string, string> = {
  skill: 'Skill',
  agent: 'SubAgent',
  config: 'Config',
  prompt: 'Prompt',
}

/** 类型展示顺序 */
const TYPE_ORDER = ['skill', 'agent', 'config', 'prompt']

/** 按 类型 + 目标路径 + 冲突对象 分组：标题行展示「类型 冲突对象」+「目标路径」，
 *  下方偏移列出本 preset 中与之冲突的资源名 */
const groups = computed(() => {
  const map = new Map<
    string,
    { type: string; targetPath: string; conflictWith: string; names: string[] }
  >()
  for (const c of props.conflicts) {
    const key = `${c.type}__${c.targetPath}__${c.conflictWith}`
    if (!map.has(key)) {
      map.set(key, { type: c.type, targetPath: c.targetPath, conflictWith: c.conflictWith, names: [] })
    }
    map.get(key)!.names.push(c.resourceName)
  }
  return Array.from(map.values()).sort(
    (a, b) => TYPE_ORDER.indexOf(a.type) - TYPE_ORDER.indexOf(b.type),
  )
})

function handleConfirm() {
  emit('confirm')
  emit('update:visible', false)
}

function handleClose() {
  emit('update:visible', false)
}
</script>

<template>
  <el-dialog
    :model-value="visible"
    title="部署冲突"
    width="560px"
    @close="handleClose"
    :close-on-click-modal="false"
  >
    <div class="flex flex-col gap-3">
      <p class="text-sm text-gray-600 dark:text-gray-400">
        以下部署子项与目标已有内容冲突，继续部署将以最新内容覆盖：
      </p>

      <!-- 按 类型+冲突对象 分组 -->
      <div class="flex flex-col gap-2">
        <div
          v-for="(g, idx) in groups"
          :key="idx"
          class="border rounded border-gray-200 dark:border-gray-700 px-3 py-2"
        >
          <!-- 标题行：类型标签 + 冲突对象 -->
          <div class="flex items-center gap-2">
            <el-tag size="small" type="warning">{{ TYPE_LABEL[g.type] || g.type }}</el-tag>
            <span class="text-sm font-medium text-gray-800 dark:text-gray-100">{{ g.conflictWith }}</span>
          </div>
          <!-- 目标路径 -->
          <div class="text-xs text-gray-400 dark:text-gray-500 truncate mt-0.5" :title="g.targetPath">
            {{ g.targetPath }}
          </div>
          <!-- 偏移列出冲突的资源名 -->
          <div
            v-for="(name, i) in g.names"
            :key="i"
            class="pl-4 mt-1 text-sm text-gray-700 dark:text-gray-200 truncate"
          >
            {{ name }}
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <el-button @click="handleClose">取消</el-button>
      <el-button type="primary" @click="handleConfirm">继续部署</el-button>
    </template>
  </el-dialog>
</template>
