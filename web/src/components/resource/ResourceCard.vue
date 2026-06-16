<script setup lang="ts">
import { computed } from 'vue'
import { ElMessage } from 'element-plus'
import type { Resource } from '@/types/resource'
import { useSelectionStore } from '@/stores/selection'
import { useResourceStore } from '@/stores/resource'
import { openFolder } from '@/api/deploy'

/** 资源卡片组件 */
const props = defineProps<{ resource: Resource }>()
const emit = defineEmits<{
  (e: 'edit', resource: Resource): void
  (e: 'editContent', resource: Resource): void
  (e: 'deploy', resource: Resource): void
  (e: 'delete', resource: Resource): void
  (e: 'removeFromGroup', resource: Resource): void
}>()

const selectionStore = useSelectionStore()
const resourceStore = useResourceStore()

/** 当前卡片是否被选中 */
const checked = computed(() => selectionStore.isSelected(props.resource.id))

/** 当前是否在"全部"分组 */
const isAllGroup = computed(() => resourceStore.currentGroupId === '0')

/** 切换选中 */
function handleCheck() {
  selectionStore.toggle(props.resource.id)
}

/** 类型对应的徽标颜色 */
const badgeClass = computed(() => {
  const map: Record<string, string> = {
    skill: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300',
    mcp: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300',
    agent: 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-300',
  }
  return map[props.resource.type] || ''
})

/** 格式化更新时间 */
const formattedTime = computed(() => {
  if (!props.resource.updated_at) return ''
  const d = new Date(props.resource.updated_at)
  return d.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' })
})

/** 下拉菜单指令处理 */
function handleCommand(cmd: string) {
  if (cmd === 'edit') emit('edit', props.resource)
  else if (cmd === 'editContent') emit('editContent', props.resource)
  else if (cmd === 'deploy') emit('deploy', props.resource)
  else if (cmd === 'reveal') handleReveal()
  else if (cmd === 'delete') {
    if (isAllGroup.value) {
      emit('delete', props.resource)
    } else {
      emit('removeFromGroup', props.resource)
    }
  }
}

/** 在文件管理器中打开资源存储位置 */
async function handleReveal() {
  try {
    await openFolder(props.resource.path)
  } catch (e: any) {
    ElMessage.error(e?.message || '打开文件夹失败')
  }
}
</script>

<template>
  <div
    class="rounded-xl border border-gray-200 bg-white shadow-sm p-4 flex flex-col gap-3
           hover:shadow-md hover:-translate-y-[1px] transition-all
           dark:bg-gray-800 dark:border-gray-700 cursor-pointer"
    :class="{ 'ring-2 ring-blue-400 border-blue-400': checked }"
    @click="handleCheck"
  >
    <!-- 顶部：复选框 + 操作菜单 -->
    <div class="flex items-center justify-between">
      <el-checkbox :model-value="checked" @change="handleCheck" @click.stop aria-label="选择此资源" />
      <el-dropdown trigger="click" @command="handleCommand">
        <button
          class="p-1 rounded hover:bg-gray-100 dark:hover:bg-gray-700 text-gray-400"
          aria-label="更多操作"
          @click.stop
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
            <circle cx="4" cy="10" r="1.5" />
            <circle cx="10" cy="10" r="1.5" />
            <circle cx="16" cy="10" r="1.5" />
          </svg>
        </button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item command="edit">编辑</el-dropdown-item>
            <el-dropdown-item command="editContent">编辑内容</el-dropdown-item>
            <el-dropdown-item command="deploy">部署</el-dropdown-item>
            <el-dropdown-item command="reveal">在文件管理器中打开</el-dropdown-item>
            <el-dropdown-item command="delete" divided>{{ isAllGroup ? '删除' : '从分组移除' }}</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </div>
    <!-- 名称 -->
    <p class="font-semibold text-gray-800 dark:text-gray-100 truncate">{{ resource.name }}</p>
    <!-- 描述 -->
    <p class="text-sm text-gray-500 dark:text-gray-400 line-clamp-2">{{ resource.description || '暂无描述' }}</p>
    <!-- 底部：更新时间 + 类型徽标 -->
    <div class="flex items-center justify-between mt-auto pt-2">
      <span class="text-xs text-gray-400 dark:text-gray-500">{{ formattedTime }}</span>
      <span class="text-xs px-2 py-0.5 rounded-full" :class="badgeClass">{{ resource.type }}</span>
    </div>
  </div>
</template>
