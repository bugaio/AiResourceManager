<script setup lang="ts">
/** Preset 主视图：顶栏 + 左侧 Preset/PathGroup 侧栏 + 右侧 4 列资源区 */
import { ref, onMounted, onUnmounted } from 'vue'
import TopBar from '@/components/layout/TopBar.vue'
import ResizeDivider from '@/components/layout/ResizeDivider.vue'
import PresetSidebar from '@/components/preset/PresetSidebar.vue'
import PresetMain from '@/components/preset/PresetMain.vue'
import PresetDeployManageDialog from '@/components/preset/PresetDeployManageDialog.vue'
import { useUiStore } from '@/stores/ui'
import { usePresetStore } from '@/stores/preset'
import { usePathGroupStore } from '@/stores/pathGroup'
import wsManager from '@/api/ws'

const uiStore = useUiStore()
const presetStore = usePresetStore()
const pathGroupStore = usePathGroupStore()

// 从侧栏打开的部署管理对话框
const sidebarDeployManageVisible = ref(false)
const sidebarDeployPresetID = ref('')
const sidebarDeployPresetName = ref('')
const sidebarGroupID = ref('')
const sidebarGroupName = ref('')

/** 侧栏点击已部署 preset → 打开管理对话框（按路径组维度） */
function handleOpenDeployManage(payload: { presetID: string; presetName: string; groupID: string; groupName: string }) {
  sidebarDeployPresetID.value = payload.presetID
  sidebarDeployPresetName.value = payload.presetName
  sidebarGroupID.value = payload.groupID
  sidebarGroupName.value = payload.groupName
  sidebarDeployManageVisible.value = true
}

/** 弹窗内重新部署后：刷新 preset 列表（更新侧栏「未同步」标识） */
async function handleDeployChanged() {
  await presetStore.fetchPresets()
}

/** WebSocket 事件分发 */
function handleWsMessage(data: unknown) {
  if (!data || typeof data !== 'object') return
  const msg = data as Record<string, unknown>
  const type = msg.type as string | undefined
  if (!type) return

  if (type.startsWith('preset:')) {
    // preset:created/updated/deleted/deployed/undeployed
    if (
      type === 'preset:created' ||
      type === 'preset:updated' ||
      type === 'preset:deleted' ||
      type === 'preset:deployed' ||
      type === 'preset:undeployed'
    ) {
      presetStore.fetchPresets()
    }
    if (type === 'preset:resource_changed') {
      presetStore.loadCurrentResources()
      presetStore.fetchPresets()
    }
    return
  }

  if (type.startsWith('path_group:')) {
    pathGroupStore.fetchPathGroups()
    return
  }
}

onMounted(() => {
  wsManager.onMessage(handleWsMessage)
})

onUnmounted(() => {
  wsManager.offMessage(handleWsMessage)
})
</script>

<template>
  <div class="flex flex-col h-screen overflow-hidden">
    <TopBar />
    <div class="flex flex-1 overflow-hidden">
      <PresetSidebar
        :style="{ width: uiStore.sidebarWidth + 'px' }"
        @open-deploy-manage="handleOpenDeployManage"
      />
      <ResizeDivider />
      <main class="flex-1 overflow-hidden">
        <PresetMain />
      </main>
    </div>

    <!-- 从侧栏打开的部署管理对话框 -->
    <PresetDeployManageDialog
      v-if="sidebarDeployPresetID"
      v-model:visible="sidebarDeployManageVisible"
      :preset-i-d="sidebarDeployPresetID"
      :preset-name="sidebarDeployPresetName"
      :group-i-d="sidebarGroupID"
      :group-name="sidebarGroupName"
      @changed="handleDeployChanged"
    />
  </div>
</template>
