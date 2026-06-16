<script setup lang="ts">
import { ref, computed, watch } from 'vue'

/** MCP 部署冲突弹窗 - 按目录分组显示冲突列表 */
export interface ConflictTarget {
  path: string
  aliasName?: string
  conflicts: Array<{ id?: string; name: string; status: 'ignored' | 'applied' | 'existing'; group: number }>
}

const props = defineProps<{
  visible: boolean
  targets: ConflictTarget[]
}>()

const emit = defineEmits<{
  (e: 'update:visible', val: boolean): void
  (e: 'confirm', selectedPaths: string[]): void
}>()

// 选中的目标路径
const selected = ref<Set<string>>(new Set())

// 打开时默认全选
watch(() => props.visible, (val) => {
  if (val) {
    selected.value = new Set(props.targets.map(t => t.path))
  }
})

const isAllSelected = computed(() =>
  props.targets.length > 0 && selected.value.size === props.targets.length
)
const isIndeterminate = computed(() =>
  selected.value.size > 0 && selected.value.size < props.targets.length
)

function toggleTarget(path: string) {
  const s = new Set(selected.value)
  if (s.has(path)) s.delete(path)
  else s.add(path)
  selected.value = s
}

function handleSelectAll(val: boolean) {
  selected.value = val ? new Set(props.targets.map(t => t.path)) : new Set()
}

function handleConfirm() {
  emit('confirm', Array.from(selected.value))
  emit('update:visible', false)
}

function handleClose() {
  emit('update:visible', false)
}

type ConflictItemType = ConflictTarget['conflicts'][number]

/** 将冲突列表按 group 分组（group>0 的归组） */
function getConflictGroups(items: ConflictItemType[]): ConflictItemType[][] {
  const grouped = new Map<number, ConflictItemType[]>()
  for (const item of items) {
    if (item.group > 0) {
      if (!grouped.has(item.group)) grouped.set(item.group, [])
      grouped.get(item.group)!.push(item)
    }
  }
  return Array.from(grouped.values())
}

/** 获取不在冲突组中的项（group=0: 无冲突 applied 或 existing） */
function getNonGroupItems(items: ConflictItemType[]): ConflictItemType[] {
  return items.filter(i => i.group === 0)
}
</script>

<template>
  <el-dialog
    :model-value="visible"
    title="MCP 部署冲突"
    width="500px"
    @close="handleClose"
    :close-on-click-modal="false"
  >
    <!-- 图例 -->
    <div class="flex items-center justify-end gap-4 mb-3 text-xs">
      <span class="flex items-center gap-1">
        <span class="w-3 h-3 rounded bg-red-100 dark:bg-red-900/30 border border-red-300"></span>
        <span class="text-gray-500">冲突忽略</span>
      </span>
      <span class="flex items-center gap-1">
        <span class="w-3 h-3 rounded bg-green-100 dark:bg-green-900/30 border border-green-300"></span>
        <span class="text-gray-500">实际应用</span>
      </span>
      <span class="flex items-center gap-1">
        <span class="w-3 h-3 rounded bg-amber-100 dark:bg-amber-900/30 border border-amber-300"></span>
        <span class="text-gray-500">已有内容冲突</span>
      </span>
    </div>

    <!-- 冲突列表：按目录分组 -->
    <div class="max-h-[360px] overflow-y-auto flex flex-col gap-3">
      <div
        v-for="target in targets"
        :key="target.path"
        class="border rounded-lg border-gray-200 dark:border-gray-600 overflow-hidden"
      >
        <!-- 目录标题行 + 复选框 -->
        <div class="flex items-center gap-2 px-3 py-2 bg-gray-50 dark:bg-gray-800 border-b border-gray-200 dark:border-gray-600">
          <el-checkbox
            :model-value="selected.has(target.path)"
            @change="toggleTarget(target.path)"
          />
          <span class="text-sm font-medium text-gray-700 dark:text-gray-300 truncate" :title="target.path">
            {{ target.aliasName || target.path }}
          </span>
        </div>
        <!-- 冲突的 MCP 列表（按冲突组分块） -->
        <div class="px-3 py-2 flex flex-col gap-2">
          <!-- 冲突组（group > 0，同组用虚线框） -->
          <div
            v-for="(groupItems, gIdx) in getConflictGroups(target.conflicts)"
            :key="'g'+gIdx"
            class="flex flex-wrap gap-1.5 px-2 py-1.5 rounded border border-dashed border-gray-300 dark:border-gray-600"
          >
            <span class="text-[10px] text-gray-400 w-full mb-0.5">冲突组</span>
            <span
              v-for="item in groupItems"
              :key="item.name"
              class="inline-flex items-center px-2 py-0.5 rounded text-xs"
              :class="{
                'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300': item.status === 'ignored',
                'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300': item.status === 'applied',
              }"
            >
              {{ item.name }}
              <span v-if="item.status === 'ignored'" class="ml-1 opacity-70">(忽略)</span>
              <span v-else class="ml-1 opacity-70">(应用)</span>
            </span>
          </div>
          <!-- 无冲突的（group=0 且 applied）+ existing -->
          <div class="flex flex-wrap gap-1.5">
            <span
              v-for="item in getNonGroupItems(target.conflicts)"
              :key="item.name"
              class="inline-flex items-center px-2 py-0.5 rounded text-xs"
              :class="{
                'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300': item.status === 'applied',
                'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300': item.status === 'existing',
              }"
            >
              {{ item.name }}
              <span v-if="item.status === 'applied'" class="ml-1 opacity-70">(应用)</span>
              <span v-else-if="item.status === 'existing'" class="ml-1 opacity-70">(已有冲突)</span>
            </span>
          </div>
        </div>
      </div>
    </div>

    <!-- 全选 -->
    <div class="mt-3 flex items-center">
      <el-checkbox
        :model-value="isAllSelected"
        :indeterminate="isIndeterminate"
        @change="handleSelectAll"
      >
        全选
      </el-checkbox>
    </div>

    <!-- 底部提示 -->
    <p class="mt-3 text-xs text-gray-500 dark:text-gray-400">
      确认覆盖将移除选中目录下冲突 MCP 的部署，保留新 MCP 配置。
    </p>

    <template #footer>
      <el-button @click="handleClose">取消</el-button>
      <el-button type="warning" :disabled="selected.size === 0" @click="handleConfirm">
        强制覆盖
      </el-button>
    </template>
  </el-dialog>
</template>
