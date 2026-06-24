<script setup lang="ts">
/** Config 冲突提示弹窗：列出每个有冲突的新增 config，及其下方与之冲突的已有 config */
import { computed } from 'vue'
import type { PresetConfigConflict } from '@/api/preset'

const props = defineProps<{
  visible: boolean
  conflicts: PresetConfigConflict[]
  /** 提示语，按场景定制 */
  hint?: string
}>()

const emit = defineEmits<{
  (e: 'update:visible', val: boolean): void
}>()

const dialogVisible = computed({
  get: () => props.visible,
  set: (val) => emit('update:visible', val),
})

const hintText = computed(
  () => props.hint || '新增的 config 与已有 config 存在冲突，请移除后重试',
)

function handleClose() {
  dialogVisible.value = false
}
</script>

<template>
  <el-dialog
    v-model="dialogVisible"
    title="Config 冲突"
    width="520px"
    :close-on-click-modal="false"
  >
    <div class="flex flex-col gap-2.5 max-h-[420px] overflow-y-auto">
      <div
        v-for="c in conflicts"
        :key="c.resource_id || c.resource_name"
        class="rounded-lg ring-1 ring-rose-200 dark:ring-rose-800/60 overflow-hidden"
      >
        <!-- 新增的 config（冲突方） -->
        <div class="flex items-center gap-2 px-3 py-2 bg-rose-50 dark:bg-rose-950/30">
          <span class="text-xs px-1.5 py-0.5 rounded bg-rose-200 dark:bg-rose-800 text-rose-700 dark:text-rose-200 shrink-0">
            新增
          </span>
          <span class="text-sm font-medium text-gray-800 dark:text-gray-100 truncate">
            {{ c.resource_name }}
          </span>
        </div>
        <!-- 与之冲突的已有 config 列表 -->
        <div class="px-3 py-2 pl-6">
          <div class="text-[11px] text-gray-400 dark:text-gray-500 mb-1">与以下已有 config 冲突：</div>
          <div class="flex flex-col gap-1">
            <div
              v-for="e in c.conflicts_with"
              :key="e.resource_id"
              class="flex items-center gap-2 text-xs"
            >
              <span class="w-1.5 h-1.5 rounded-full bg-rose-400 shrink-0"></span>
              <span class="text-gray-700 dark:text-gray-300 truncate">{{ e.resource_name }}</span>
              <span class="text-[10px] px-1 rounded bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400 shrink-0">已存在</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 底部提示 -->
    <div class="mt-3 flex items-center gap-2 px-3 py-2 rounded-md bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 text-xs text-amber-700 dark:text-amber-300">
      <span>⚠️</span>
      <span>{{ hintText }}</span>
    </div>

    <template #footer>
      <el-button type="primary" @click="handleClose">知道了</el-button>
    </template>
  </el-dialog>
</template>
