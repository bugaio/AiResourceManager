import { defineStore } from 'pinia'
import { ref } from 'vue'
import * as deployApi from '@/api/deploy'
import { useUiStore } from '@/stores/ui'
import type { TargetInfo, DeployRequest, DeploymentItem } from '@/types/deploy'

/** 部署状态管理 */
export const useDeployStore = defineStore('deploy', () => {
  const targets = ref<TargetInfo[]>([])
  const loading = ref(false)
  const deploying = ref(false)

  /** 获取目标路径聚合列表（按当前资源类型过滤） */
  async function fetchTargets() {
    loading.value = true
    try {
      const uiStore = useUiStore()
      targets.value = await deployApi.getTargets(uiStore.currentType)
    } catch (_e) {
      targets.value = []
    } finally {
      loading.value = false
    }
  }

  /** 执行部署 */
  async function deploy(req: DeployRequest) {
    deploying.value = true
    try {
      const result = await deployApi.deploy(req)
      await fetchTargets()
      return result
    } finally {
      deploying.value = false
    }
  }

  /** 撤销部署 */
  async function undeploy(id: string) {
    await deployApi.undeploy(id)
    await fetchTargets()
  }

  /** 健康检查 */
  async function checkHealth(): Promise<DeploymentItem[]> {
    const broken = await deployApi.checkHealth()
    // 检查后刷新targets以更新状态
    await fetchTargets()
    return broken
  }

  /** 修复部署 */
  async function repair(deploymentId: string, itemId: string) {
    await deployApi.repairItem(deploymentId, itemId)
    await fetchTargets()
  }

  /** 清理部署项（undeploy=true 时同时撤销文件） */
  async function cleanItem(deploymentId: string, itemId: string, undeploy = false) {
    await deployApi.cleanItem(deploymentId, itemId, undeploy)
    await fetchTargets()
  }

  return {
    targets,
    loading,
    deploying,
    fetchTargets,
    deploy,
    undeploy,
    checkHealth,
    repair,
    cleanItem,
  }
})
