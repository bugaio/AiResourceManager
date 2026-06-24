<script setup lang="ts">
import { computed } from 'vue'
import { useResourceStore } from '@/stores/resource'
import { useSelectionStore } from '@/stores/selection'
import type { Resource } from '@/types/resource'

/** 资源列表视图（表格模式） */
const resourceStore = useResourceStore()
const selectionStore = useSelectionStore()

const emit = defineEmits<{
  (e: 'edit', resource: Resource): void
  (e: 'editContent', resource: Resource): void
  (e: 'deploy', resource: Resource): void
  (e: 'delete', resource: Resource): void
  (e: 'removeFromGroup', resource: Resource): void
  (e: 'viewLinks', resource: Resource): void
}>()

/** 当前是否在"全部"分组 */
const isAllGroup = computed(() => resourceStore.currentGroupId === '0')

/** 类型中文映射 */
const typeLabel: Record<string, string> = {
  skill: 'Skill',
  config: 'Config',
  agent: 'Agent',
  prompt: 'Prompt',
}

/** 格式化时间 */
function formatTime(t: string): string {
  if (!t) return ''
  return new Date(t).toLocaleDateString('zh-CN')
}

/** 表格选中变化 */
function handleSelectionChange(rows: Resource[]) {
  if (rows.length === 0) {
    selectionStore.clearAll()
  } else {
    selectionStore.selectAll(rows.map((r) => r.id))
  }
}

/** 下拉菜单操作 */
function handleCommand(cmd: string, row: Resource) {
  if (cmd === 'edit') emit('edit', row)
  else if (cmd === 'editContent') emit('editContent', row)
  else if (cmd === 'deploy') emit('deploy', row)
  else if (cmd === 'viewLinks') emit('viewLinks', row)
  else if (cmd === 'delete') {
    if (isAllGroup.value) {
      emit('delete', row)
    } else {
      emit('removeFromGroup', row)
    }
  }
}

/** 某行是否被 preset 关联 */
function rowHasLinks(row: Resource): boolean {
  return !!row.preset_links && row.preset_links.length > 0
}

/** 双击行打开内容编辑 */
function handleRowDblClick(row: Resource) {
  emit('editContent', row)
}
</script>

<template>
  <el-table
    :data="resourceStore.resources"
    style="width: 100%"
    @selection-change="handleSelectionChange"
    @row-dblclick="handleRowDblClick"
    class="dark:bg-gray-800"
  >
    <!-- 多选列 -->
    <el-table-column type="selection" width="50" />
    <!-- 名称 -->
    <el-table-column prop="name" label="名称" min-width="160" show-overflow-tooltip>
      <template #default="{ row }">
        <span class="inline-flex items-center gap-1">
          <span class="truncate">{{ row.name }}</span>
          <el-tooltip
            v-if="row.preset_links && row.preset_links.length > 0"
            effect="dark"
            placement="top"
            :content="'被以下 Preset 管理：' + row.preset_links.map((p: any) => p.name).join(', ')"
          >
            <span class="text-sm leading-none" aria-label="被 Preset 关联">🔗</span>
          </el-tooltip>
        </span>
      </template>
    </el-table-column>
    <!-- 描述 -->
    <el-table-column prop="description" label="描述" min-width="240" show-overflow-tooltip />
    <!-- 类型 -->
    <el-table-column label="类型" width="100">
      <template #default="{ row }">
        <span class="text-xs px-2 py-0.5 rounded-full bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300">
          {{ typeLabel[row.type] || row.type }}
        </span>
      </template>
    </el-table-column>
    <!-- 更新时间 -->
    <el-table-column label="更新时间" width="120">
      <template #default="{ row }">
        <span class="text-sm text-gray-500">{{ formatTime(row.updated_at) }}</span>
      </template>
    </el-table-column>
    <!-- 操作 -->
    <el-table-column label="操作" width="80" fixed="right">
      <template #default="{ row }">
        <el-dropdown trigger="click" @command="(cmd: string) => handleCommand(cmd, row)">
          <el-button text size="small">
            <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
              <circle cx="4" cy="10" r="1.5" />
              <circle cx="10" cy="10" r="1.5" />
              <circle cx="16" cy="10" r="1.5" />
            </svg>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="edit">编辑</el-dropdown-item>
              <el-dropdown-item command="editContent">编辑内容</el-dropdown-item>
              <el-dropdown-item command="deploy">部署</el-dropdown-item>
              <el-dropdown-item v-if="rowHasLinks(row)" command="viewLinks" divided>查看关联</el-dropdown-item>
              <el-dropdown-item command="delete" divided>{{ isAllGroup ? '删除' : '从分组移除' }}</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </template>
    </el-table-column>
  </el-table>
</template>
