<script setup lang="ts">
/** Preset 视图中的资源卡片 — 区分私有 / 关联 */
import { computed } from 'vue'
import type { PresetResource } from '@/types/preset'

const props = defineProps<{
  resource: PresetResource
}>()

const emit = defineEmits<{
  (e: 'edit', r: PresetResource): void
  (e: 'editContent', r: PresetResource): void
  (e: 'viewContent', r: PresetResource): void
  (e: 'delete', r: PresetResource): void
  (e: 'unlink', r: PresetResource): void
}>()

/** 是否为私有资源 */
const isPrivate = computed(() => !!props.resource.owner_preset_id)

function handleCommand(cmd: string) {
  if (cmd === 'edit') emit('edit', props.resource)
  else if (cmd === 'editContent') emit('editContent', props.resource)
  else if (cmd === 'viewContent') emit('viewContent', props.resource)
  else if (cmd === 'delete') emit('delete', props.resource)
  else if (cmd === 'unlink') emit('unlink', props.resource)
}

const formattedTime = computed(() => {
  if (!props.resource.updated_at) return ''
  return new Date(props.resource.updated_at).toLocaleDateString('zh-CN', {
    month: 'short',
    day: 'numeric',
  })
})
</script>

<template>
  <div
    class="rounded-lg p-3 flex flex-col gap-2 bg-white dark:bg-gray-800 transition-all hover:shadow-md"
    :class="
      isPrivate
        ? 'border-2 border-blue-500/70'
        : 'border-2 border-dashed border-orange-400/60 dark:border-orange-500/50 bg-orange-50/30 dark:bg-orange-950/10'
    "
  >
    <!-- 顶部：徽标 + 菜单 -->
    <div class="flex items-center justify-between">
      <span
        class="text-[10px] px-1.5 py-0.5 rounded-full font-medium"
        :class="
          isPrivate
            ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300'
            : 'bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-300'
        "
      >{{ isPrivate ? '私有' : '关联' }}</span>
      <el-dropdown trigger="click" @command="handleCommand">
        <button
          class="p-0.5 rounded text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700"
          @click.stop
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
            <circle cx="4" cy="10" r="1.5" />
            <circle cx="10" cy="10" r="1.5" />
            <circle cx="16" cy="10" r="1.5" />
          </svg>
        </button>
        <template #dropdown>
          <el-dropdown-menu v-if="isPrivate">
            <el-dropdown-item command="edit">编辑</el-dropdown-item>
            <el-dropdown-item command="editContent">编辑内容</el-dropdown-item>
            <el-dropdown-item command="delete" divided>删除</el-dropdown-item>
          </el-dropdown-menu>
          <el-dropdown-menu v-else>
            <el-dropdown-item command="viewContent">查看内容</el-dropdown-item>
            <el-dropdown-item command="unlink" divided>取消关联</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </div>
    <!-- 名称 -->
    <div
      class="text-sm font-semibold text-gray-800 dark:text-gray-100 truncate"
    >
      {{ resource.name }}
    </div>
    <!-- 描述 -->
    <div
      class="text-xs text-gray-500 dark:text-gray-400 line-clamp-2"
    >
      {{ resource.description || '暂无描述' }}
    </div>
    <!-- 底部时间 -->
    <div class="text-[10px] text-gray-400 dark:text-gray-500 mt-auto">
      {{ formattedTime }}
    </div>
  </div>
</template>
