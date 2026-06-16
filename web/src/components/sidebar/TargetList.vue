<script setup lang="ts">
import { onMounted, computed, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useDeployStore } from '@/stores/deploy'
import { useAliasStore } from '@/stores/alias'
import { useUiStore } from '@/stores/ui'
import TargetItem from './TargetItem.vue'
import type { TargetInfo } from '@/types/deploy'
import type { PathAlias } from '@/types/alias'

/** 目标路径列表组件 */
const deployStore = useDeployStore()
const aliasStore = useAliasStore()
const uiStore = useUiStore()

onMounted(() => {
  deployStore.fetchTargets()
  aliasStore.fetchAliases()
})

// 切换资源类型时，重新拉取该类型的目标路径与别名（别名按类型隔离）
watch(() => uiStore.currentType, () => {
  deployStore.fetchTargets()
  aliasStore.fetchAliases()
})

/** 统一的列表项：已部署目标 或 纯别名 */
interface OrderedItem {
  key: string
  sortTime: string
  target?: TargetInfo
  alias?: PathAlias
}

/**
 * 合并已部署目标 + 纯别名，统一按创建时间倒序排列
 * - 有别名的路径：用别名 created_at
 * - 无别名的手动部署路径：用该路径下最早 deployment 的 created_at
 */
const orderedItems = computed<OrderedItem[]>(() => {
  const items: OrderedItem[] = []
  const deployedPaths = new Set(deployStore.targets.map(t => t.target_path))

  // 已部署的目标
  for (const target of deployStore.targets) {
    const alias = aliasStore.aliases.find(a => a.path === target.target_path)
    let sortTime = ''
    if (alias) {
      sortTime = alias.created_at
    } else {
      // 用最早 deployment 的创建时间作为路径创建时间
      const times = target.deployments.map(d => d.created_at).filter(Boolean)
      sortTime = times.length > 0 ? times.sort()[0] : ''
    }
    items.push({ key: target.target_path, sortTime, target })
  }

  // 未被部署的纯别名
  for (const alias of aliasStore.aliases) {
    if (!deployedPaths.has(alias.path)) {
      items.push({ key: `alias-${alias.id}`, sortTime: alias.created_at, alias })
    }
  }

  // 按创建时间倒序（最新在前）
  items.sort((a, b) => (b.sortTime || '').localeCompare(a.sortTime || ''))
  return items
})

/** 删除别名 */
async function handleDeleteAlias(aliasId: string, aliasName: string) {
  try {
    await ElMessageBox.confirm(
      `确定删除别名「${aliasName}」？`,
      '删除别名',
      { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' }
    )
    await aliasStore.deleteAlias(aliasId)
    ElMessage.success('别名已删除')
  } catch {
    // cancelled
  }
}
</script>

<template>
  <div class="flex flex-col gap-1.5">
    <!-- 加载状态 -->
    <div v-if="deployStore.loading && deployStore.targets.length === 0" class="text-xs text-gray-400 dark:text-gray-500 italic">
      加载中...
    </div>

    <template v-else>
      <!-- 统一有序列表 -->
      <template v-for="item in orderedItems" :key="item.key">
        <!-- 已部署目标 -->
        <TargetItem
          v-if="item.target"
          :target-info="item.target"
        />

        <!-- 未被部署的纯别名 -->
        <div
          v-else-if="item.alias"
          class="flex items-center gap-1.5 px-2 py-1.5 rounded border border-dashed border-purple-200 dark:border-purple-700 bg-purple-50/50 dark:bg-purple-900/20"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5 text-purple-400 flex-shrink-0" viewBox="0 0 20 20" fill="currentColor">
            <path d="M10.707 2.293a1 1 0 00-1.414 0l-7 7a1 1 0 001.414 1.414L4 10.414V17a1 1 0 001 1h2a1 1 0 001-1v-2a1 1 0 011-1h2a1 1 0 011 1v2a1 1 0 001 1h2a1 1 0 001-1v-6.586l.293.293a1 1 0 001.414-1.414l-7-7z" />
          </svg>
          <span class="text-xs text-purple-600 dark:text-purple-300 truncate flex-1" :title="item.alias.path">
            {{ item.alias.name }}
          </span>
          <!-- 删除按钮 -->
          <button
            class="p-0.5 text-gray-400 hover:text-red-500 dark:hover:text-red-400 flex-shrink-0"
            title="删除别名"
            @click="handleDeleteAlias(item.alias.id, item.alias.name)"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" viewBox="0 0 20 20" fill="currentColor">
              <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd" />
            </svg>
          </button>
        </div>
      </template>

      <!-- 空状态 -->
      <div
        v-if="orderedItems.length === 0"
        class="text-xs text-gray-400 dark:text-gray-500 italic"
      >
        暂无部署或别名
      </div>
    </template>
  </div>
</template>
