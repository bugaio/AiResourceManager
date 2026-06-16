<script setup lang="ts">
/** 侧边栏：分组列表 + 目标路径，均支持折叠 */
import { ref } from "vue"
import { useRouter } from "vue-router"
import { ElMessage, ElPopover, ElInput, ElButton } from 'element-plus'
import GroupList from '@/components/sidebar/GroupList.vue'
import TargetList from '@/components/sidebar/TargetList.vue'
import { useGroupStore } from '@/stores/group'

const groupStore = useGroupStore()
const router = useRouter()

const groupCollapsed = ref(false)
const targetCollapsed = ref(false)

// 新建分组
const showCreatePopover = ref(false)
const newGroupName = ref('')

async function handleCreateGroup() {
  const name = newGroupName.value.trim()
  if (!name) return
  try {
    await groupStore.createGroup(name)
    ElMessage.success('分组创建成功')
  } catch (e: any) {
    ElMessage.error(e?.message || '创建失败')
  }
  newGroupName.value = ''
  showCreatePopover.value = false
}
</script>

<template>
  <aside
    class="flex flex-col h-full overflow-y-auto bg-slate-50 dark:bg-slate-900 border-r border-gray-200 dark:border-gray-700 flex-shrink-0"
  >
    <!-- 上部：分组列表 -->
    <div class="flex flex-col px-3 pt-3 pb-1">
      <div class="flex items-center justify-between mb-1">
        <div
          class="flex items-center gap-1 cursor-pointer select-none"
          @click="groupCollapsed = !groupCollapsed"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="w-3.5 h-3.5 text-gray-400 transition-transform duration-200"
            :class="{ '-rotate-90': groupCollapsed }"
            viewBox="0 0 20 20"
            fill="currentColor"
          >
            <path fill-rule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clip-rule="evenodd" />
          </svg>
          <span class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">分组</span>
        </div>
        <!-- 新建分组按钮 -->
        <el-popover
          v-model:visible="showCreatePopover"
          placement="bottom-end"
          :width="220"
          trigger="click"
        >
          <template #reference>
            <button
              class="p-0.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
              title="新建分组"
              @click.stop
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" viewBox="0 0 20 20" fill="currentColor">
                <path fill-rule="evenodd" d="M10 3a1 1 0 011 1v5h5a1 1 0 110 2h-5v5a1 1 0 11-2 0v-5H4a1 1 0 110-2h5V4a1 1 0 011-1z" clip-rule="evenodd" />
              </svg>
            </button>
          </template>
          <div class="flex flex-col gap-2">
            <span class="text-sm font-medium">新建分组</span>
            <el-input
              v-model="newGroupName"
              size="small"
              placeholder="分组名称"
              @keyup.enter="handleCreateGroup"
            />
            <el-button
              size="small"
              type="primary"
              :disabled="!newGroupName.trim()"
              @click="handleCreateGroup"
            >
              确定
            </el-button>
          </div>
        </el-popover>
      </div>
      <div v-show="!groupCollapsed" class="overflow-y-auto">
        <GroupList />
      </div>
    </div>

    <!-- 下部：目标路径 -->
    <div class="px-3 pt-1 pb-3 border-t border-gray-200 dark:border-gray-700">
      <div class="flex items-center justify-between mb-1">
        <div
          class="flex items-center gap-1 cursor-pointer select-none"
          @click="targetCollapsed = !targetCollapsed"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="w-3.5 h-3.5 text-gray-400 transition-transform duration-200"
            :class="{ '-rotate-90': targetCollapsed }"
            viewBox="0 0 20 20"
            fill="currentColor"
          >
            <path fill-rule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clip-rule="evenodd" />
          </svg>
          <span class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">目标路径</span>
        </div>
        <button
          class="p-0.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
          title="目录别名管理"
          @click="router.push('/aliases')"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" viewBox="0 0 20 20" fill="currentColor">
            <path d="M10 3.5a1.5 1.5 0 013 0V5h2a1 1 0 011 1v2a1.5 1.5 0 010 3v5a1 1 0 01-1 1H5a1 1 0 01-1-1v-5a1.5 1.5 0 010-3V6a1 1 0 011-1h2V3.5a1.5 1.5 0 013 0V5h0z" opacity="0"/>
            <path fill-rule="evenodd" d="M3 6a2 2 0 012-2h2.5a.5.5 0 01.4.2l1.2 1.6a.5.5 0 00.4.2H15a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2V6z" clip-rule="evenodd"/>
          </svg>
        </button>
      </div>
      <div v-show="!targetCollapsed">
        <TargetList />
      </div>
    </div>
  </aside>
</template>
