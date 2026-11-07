<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import AppLayout from '@/components/layout/AppLayout.vue'
import ResourcePanel from '@/components/resource/ResourcePanel.vue'
import { useUiStore } from '@/stores/ui'
import { useResourceStore } from '@/stores/resource'
import { useDeployStore } from '@/stores/deploy'
import wsManager from '@/api/ws'

/** 资源管理主视图 — 协调器
 *
 * 每个资源类型(skill/mdconfig/config/prompt)由一个独立的 ResourcePanel 承载,
 * 用 <keep-alive> 缓存。切换类型即切换组件实例,各面板的搜索框/分组/分页
 * 状态原地保留,互不干扰,切换时不会触发其他模块的筛选闪动。
 *
 * WebSocket 监听在此单点注册,避免 4 个面板重复注册重复刷新。
 */
const uiStore = useUiStore()
const resourceStore = useResourceStore()
const deployStore = useDeployStore()

// resource:updated 刷新防抖：批量导入/外部批量改动会短时间内触发多条 watcher 广播，
// 逐条刷新会导致列表反复重绘闪烁。合并到一次刷新。
let refreshTimer: ReturnType<typeof setTimeout> | null = null
function scheduleRefresh() {
  if (refreshTimer) clearTimeout(refreshTimer)
  refreshTimer = setTimeout(() => {
    refreshTimer = null
    resourceStore.fetchResources(uiStore.currentType)
  }, 250)
}

function handleWsMessage(data: unknown) {
  if (!data || typeof data !== 'object') return
  const msg = data as Record<string, unknown>

  if (msg.type === 'deploy:synced') {
    deployStore.fetchTargets()
    ElMessage.info('部署已自动同步')
  }

  if (msg.type === 'resource:updated') {
    // 仅刷新当前显示的类型(其他类型在激活时会自行重新加载)；防抖合并连续事件
    scheduleRefresh()
  }

  if (msg.type === 'resource:deleted') {
    const payload = msg.data as Record<string, unknown> | undefined
    const id = payload?.id as string | undefined
    const uuid = payload?.uuid as string | undefined
    const name = payload?.name as string | undefined
    const removed = resourceStore.removeResourceLocally(id, uuid)
    if (removed) ElMessage.warning(`资源 ${name || removed} 已被外部删除`)
  }
}

onMounted(() => {
  wsManager.onMessage(handleWsMessage)
})

onUnmounted(() => {
  if (refreshTimer) clearTimeout(refreshTimer)
  wsManager.offMessage(handleWsMessage)
})
</script>

<template>
  <AppLayout>
    <keep-alive>
      <ResourcePanel :key="uiStore.currentType" :type="uiStore.currentType" />
    </keep-alive>
  </AppLayout>
</template>
