<script setup lang="ts">
import { ref, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useDeployStore } from '@/stores/deploy'
import { useAliasStore } from '@/stores/alias'
import { useUiStore } from '@/stores/ui'
import { checkPathExists, openFolder } from '@/api/deploy'
import type { TargetInfo } from '@/types/deploy'

/** 目标路径项组件 */
const props = defineProps<{ targetInfo: TargetInfo }>()

const deployStore = useDeployStore()
const aliasStore = useAliasStore()
const uiStore = useUiStore()

const expanded = ref(false)
const checking = ref(false)

// 编辑菜单弹窗
const editMenuVisible = ref(false)

// 收藏弹窗
const favoriteDialogVisible = ref(false)
const favoriteAliasName = ref('')
const favoriteSaving = ref(false)

/** 是否有对应别名 */
const matchedAlias = computed(() => {
  return aliasStore.aliases.find(a => a.path === props.targetInfo.target_path)
})

/** 显示名称：有别名显示别名，否则显示完整路径 */
const displayName = computed(() => {
  return matchedAlias.value ? matchedAlias.value.name : props.targetInfo.target_path
})

/** 扁平化所有部署子项（按分组名称排序，同分组紧挨；附加所属 deployment 的 track 标记） */
const allItems = computed(() => {
  const items = props.targetInfo.deployments.flatMap(d =>
    d.items.map(item => ({ ...item, track: d.track === 1 }))
  )
  return items.sort((a, b) => {
    const ga = a.group_name || ''
    const gb = b.group_name || ''
    if (ga !== gb) return ga.localeCompare(gb)
    return 0
  })
})

/** 当前类型对应的 deploy_type */
const currentDeployType = computed(() => {
  return uiStore.currentType === 'config' ? 'merge' : 'symlink'
})

/** 当前类型下的部署记录 */
const currentTypeDeployments = computed(() => {
  return props.targetInfo.deployments.filter(d => d.deploy_type === currentDeployType.value)
})

/** 检查该目标下所有部署健康状态 */
async function handleCheck() {
  checking.value = true
  try {
    const { exists } = await checkPathExists(props.targetInfo.target_path)
    if (!exists) {
      ElMessage.warning('目标路径已不存在，自动清理关联记录')
      for (const d of props.targetInfo.deployments) {
        await deployStore.undeploy(d.id)
      }
      if (matchedAlias.value) {
        await aliasStore.deleteAlias(matchedAlias.value.id)
      }
      return
    }
    const broken = await deployStore.checkHealth()
    const myBroken = broken.filter(item =>
      props.targetInfo.deployments.some(d => d.items.some(i => i.id === item.id))
    )
    if (myBroken.length === 0) {
      ElMessage.success('该路径下部署状态正常')
    } else {
      ElMessage.warning(`发现 ${myBroken.length} 个异常项`)
    }
  } catch {
    ElMessage.error('检查失败')
  } finally {
    checking.value = false
  }
}

/** 打开编辑菜单 */
function handleEditMenu() {
  editMenuVisible.value = true
}

/** 清空当前类型的部署 */
async function handleClear() {
  editMenuVisible.value = false
  const typeName = uiStore.currentType === 'skill' ? 'Skill' : uiStore.currentType === 'config' ? 'Config' : 'SubAgent'
  const deployments = currentTypeDeployments.value
  if (deployments.length === 0) {
    ElMessage.info(`该路径下没有 ${typeName} 类型的部署`)
    return
  }
  try {
    await ElMessageBox.confirm(
      `确定清空「${displayName.value}」下所有 ${typeName} 类型的部署？`,
      '清空部署',
      { confirmButtonText: '清空', cancelButtonText: '取消', type: 'warning' }
    )
    for (const d of deployments) {
      await deployStore.undeploy(d.id)
    }
    ElMessage.success(`已清空 ${typeName} 部署`)
  } catch {
    // cancelled
  }
}

/** 删除此路径 */
async function handleDelete() {
  editMenuVisible.value = false
  try {
    await ElMessageBox.confirm(
      `确定删除「${displayName.value}」？将撤销该路径下所有部署。`,
      '删除路径',
      { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' }
    )
    for (const d of props.targetInfo.deployments) {
      await deployStore.undeploy(d.id)
    }
    if (matchedAlias.value) {
      await aliasStore.deleteAlias(matchedAlias.value.id)
    }
    ElMessage.success('已删除')
  } catch {
    // cancelled
  }
}

/** 转为别名目录 */
function handleFavorite() {
  editMenuVisible.value = false
  favoriteAliasName.value = ''
  favoriteDialogVisible.value = true
}

/** 确认收藏 */
async function handleFavoriteConfirm() {
  if (!favoriteAliasName.value.trim()) {
    ElMessage.warning('请输入别名名称')
    return
  }
  favoriteSaving.value = true
  try {
    await aliasStore.createAlias({
      name: favoriteAliasName.value.trim(),
      path: props.targetInfo.target_path,
    })
    ElMessage.success('已保存为别名')
    favoriteDialogVisible.value = false
  } catch (e: any) {
    ElMessage.error(e?.message || '保存失败')
  } finally {
    favoriteSaving.value = false
  }
}

/** 在文件管理器中打开 */
async function handleRevealInFinder() {
  editMenuVisible.value = false
  try {
    await openFolder(props.targetInfo.target_path)
  } catch (e: any) {
    ElMessage.error(e?.message || '打开文件夹失败')
  }
}

/** 修复部署 */
async function handleRepair(deploymentId: string, itemId: string) {
  try {
    await deployStore.repair(deploymentId, itemId)
    ElMessage.success('修复完成')
  } catch (e: any) {
    ElMessage.error(e?.message || '修复失败')
  }
}

/** 清理部署项（broken 时只删记录，ok 时撤销文件+删记录） */
async function handleClean(deploymentId: string, itemId: string, status: string) {
  const undeploy = status !== 'broken'
  await deployStore.cleanItem(deploymentId, itemId, undeploy)
  ElMessage.success('已移除')
}
</script>

<template>
  <div>
    <!-- 目标路径行 -->
    <div class="flex items-center gap-1 px-2 py-1.5 rounded hover:bg-gray-100 dark:hover:bg-gray-800 group">
      <!-- 展开箭头 -->
      <svg
        xmlns="http://www.w3.org/2000/svg"
        class="w-3 h-3 text-gray-400 transition-transform cursor-pointer flex-shrink-0"
        :class="{ 'rotate-90': expanded }"
        fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"
        @click="expanded = !expanded"
      >
        <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" />
      </svg>

      <!-- 名称 -->
      <span
        class="text-xs truncate flex-1 cursor-pointer"
        :class="matchedAlias
          ? 'text-purple-600 dark:text-purple-300 font-medium'
          : 'text-gray-700 dark:text-gray-300'"
        :title="targetInfo.target_path"
        @click="expanded = !expanded"
      >
        {{ displayName }}
      </span>

      <!-- 操作按钮：检查 + 编辑菜单 -->
      <div class="flex items-center gap-0.5">
        <button
          class="p-0.5 text-gray-400 hover:text-blue-500 dark:hover:text-blue-400"
          title="检查健康状态"
          :disabled="checking"
          @click.stop="handleCheck"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" viewBox="0 0 20 20" fill="currentColor">
            <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
          </svg>
        </button>
        <button
          class="p-0.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
          title="更多操作"
          @click.stop="handleEditMenu"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" viewBox="0 0 20 20" fill="currentColor">
            <path d="M6 10a2 2 0 11-4 0 2 2 0 014 0zM12 10a2 2 0 11-4 0 2 2 0 014 0zM16 12a2 2 0 100-4 2 2 0 000 4z" />
          </svg>
        </button>
      </div>
    </div>

    <!-- 展开：资源列表 -->
    <div v-if="expanded" class="ml-5 pl-2 border-l border-gray-200 dark:border-gray-700">
      <!-- 有别名时显示完整路径 -->
      <div v-if="matchedAlias" class="text-[10px] text-gray-400 dark:text-gray-500 truncate py-0.5">
        {{ targetInfo.target_path }}
      </div>

      <!-- 资源列表 -->
      <div
        v-for="item in allItems"
        :key="item.id"
        class="flex items-center gap-1.5 py-0.5 group/item"
      >
        <span
          class="w-1.5 h-1.5 rounded-full flex-shrink-0"
          :class="item.status === 'broken' ? 'bg-orange-400' : 'bg-green-400'"
        ></span>
        <span
          class="text-xs truncate"
          :class="item.status === 'broken' ? 'text-orange-500' : 'text-gray-600 dark:text-gray-400'"
        >
          {{ item.resource_name || item.resource_id }}
        </span>
        <!-- 分组标签 -->
        <span
          v-if="item.group_name"
          class="text-[10px] px-1 py-0.5 rounded flex-shrink-0 text-white leading-none inline-flex items-center gap-0.5"
          :style="{ backgroundColor: item.group_color || '#9CA3AF' }"
        >
          <span v-if="item.track" title="跟踪分组变化">🔗</span>
          {{ item.group_name }}
        </span>
        <span class="flex-1"></span>
        <button
          v-if="item.status === 'broken'"
          class="text-[10px] text-blue-500 hover:text-blue-600"
          @click.stop="handleRepair(item.deployment_id, item.id)"
        >修复</button>
        <button
          class="text-[10px] text-gray-400 hover:text-red-500"
          @click.stop="handleClean(item.deployment_id, item.id, item.status)"
        >移除</button>
      </div>
    </div>

    <!-- 编辑菜单弹窗 -->
    <el-dialog
      v-model="editMenuVisible"
      title="路径操作"
      width="320px"
      :close-on-click-modal="true"
    >
      <div class="flex flex-col gap-2">
        <button
          class="w-full text-left px-3 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded"
          @click="handleClear"
        >
          🗑️ 清空当前类型部署
        </button>
        <button
          class="w-full text-left px-3 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded"
          @click="handleDelete"
        >
          ❌ 删除此路径
        </button>
        <button
          v-if="!matchedAlias"
          class="w-full text-left px-3 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded"
          @click="handleFavorite"
        >
          ⭐ 转为别名目录
        </button>
        <button
          class="w-full text-left px-3 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded"
          @click="handleRevealInFinder"
        >
          📂 在文件管理器中打开
        </button>
      </div>
    </el-dialog>

    <!-- 收藏弹窗 -->
    <el-dialog
      v-model="favoriteDialogVisible"
      title="转为别名目录"
      width="400px"
      :close-on-click-modal="false"
    >
      <div class="flex flex-col gap-4">
        <div>
          <div class="text-sm text-gray-600 dark:text-gray-400 mb-1">路径</div>
          <el-input :model-value="targetInfo.target_path" disabled />
        </div>
        <div>
          <div class="text-sm text-gray-600 dark:text-gray-400 mb-1">别名名称</div>
          <el-input v-model="favoriteAliasName" placeholder="输入别名名称" maxlength="50" />
        </div>
      </div>
      <template #footer>
        <el-button @click="favoriteDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="favoriteSaving" @click="handleFavoriteConfirm">确认</el-button>
      </template>
    </el-dialog>
  </div>
</template>
