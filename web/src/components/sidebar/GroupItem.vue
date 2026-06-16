<script setup lang="ts">
/** 分组项组件 */
import type { Group } from '@/types/group'

const props = defineProps<{
  group: Group
  isActive: boolean
}>()

const emit = defineEmits<{
  rename: [group: Group]
  delete: [group: Group]
}>()

/** 处理下拉命令 */
function handleCommand(cmd: string) {
  if (cmd === 'rename') emit('rename', props.group)
  else if (cmd === 'delete') emit('delete', props.group)
}
</script>

<template>
  <div
    class="flex items-center justify-between px-3 py-2 rounded-md cursor-pointer text-sm transition-colors"
    :class="isActive
      ? 'bg-blue-100 dark:bg-blue-900/40 text-blue-700 dark:text-blue-300'
      : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800'"
  >
    <span class="flex items-center gap-1.5 truncate">
      <span
        v-if="group.color"
        class="w-2 h-2 rounded-full flex-shrink-0"
        :style="{ backgroundColor: group.color }"
      ></span>
      {{ group.name }}
      <span
        v-if="group.resource_count > 0"
        class="text-xs ml-0.5"
        :style="{ color: group.color }"
      >({{ group.resource_count }})</span>
    </span>
    <el-dropdown trigger="click" @command="handleCommand">
      <span
        class="ml-2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 flex-shrink-0"
        @click.stop
      >
        ···
      </span>
      <template #dropdown>
        <el-dropdown-menu>
          <el-dropdown-item command="rename">重命名</el-dropdown-item>
          <el-dropdown-item command="delete">删除</el-dropdown-item>
        </el-dropdown-menu>
      </template>
    </el-dropdown>
  </div>
</template>
