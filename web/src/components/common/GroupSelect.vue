<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useGroupStore } from '@/stores/group'
import type { ResourceType } from '@/types/resource'

/** 分组选择组件 - 带快速创建功能 */
const props = defineProps<{
  modelValue: string[]
  type: ResourceType
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', val: string[]): void
}>()

const groupStore = useGroupStore()

// 新建分组输入
const newGroupName = ref('')
// 创建中状态
const creating = ref(false)

onMounted(() => {
  // 确保分组数据已加载
  if (groupStore.groups.length === 0) {
    groupStore.fetchGroups()
  }
})

/** 切换分组选中状态 */
function toggleGroup(id: string) {
  const current = [...props.modelValue]
  const idx = current.indexOf(id)
  if (idx === -1) {
    current.push(id)
  } else {
    current.splice(idx, 1)
  }
  emit('update:modelValue', current)
}

/** 快速创建分组 */
async function handleCreate() {
  const name = newGroupName.value.trim()
  if (!name) return
  creating.value = true
  try {
    const group = await groupStore.createGroup(name)
    newGroupName.value = ''
    // 自动选中新创建的分组
    emit('update:modelValue', [...props.modelValue, group.id])
  } finally {
    creating.value = false
  }
}
</script>

<template>
  <div class="space-y-2">
    <!-- 分组列表 -->
    <div
      v-if="groupStore.groups.length > 0"
      class="max-h-32 overflow-y-auto space-y-1 border border-gray-200 dark:border-gray-600 rounded p-2"
    >
      <label
        v-for="g in groupStore.groups"
        :key="g.id"
        class="flex items-center gap-2 cursor-pointer text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700 px-1 py-0.5 rounded"
      >
        <input
          type="checkbox"
          :checked="modelValue.includes(g.id)"
          class="rounded border-gray-300"
          @change="toggleGroup(g.id)"
        />
        <span class="truncate">{{ g.name }}</span>
      </label>
    </div>
    <p v-else class="text-xs text-gray-400">暂无分组</p>

    <!-- 快速新建分组 -->
    <div class="flex items-center gap-2">
      <el-input
        v-model="newGroupName"
        placeholder="新建分组名"
        size="small"
        class="flex-1"
        @keyup.enter="handleCreate"
      />
      <el-button
        size="small"
        :loading="creating"
        :disabled="!newGroupName.trim()"
        @click="handleCreate"
      >
        + 新建分组
      </el-button>
    </div>
  </div>
</template>
