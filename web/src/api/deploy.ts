import request from './request'
import type { Deployment, DeployRequest, DeploymentItem, TargetInfo } from '@/types/deploy'
import type { PaginatedList } from '@/types/resource'

/** 执行部署 */
export function deploy(req: DeployRequest): Promise<Deployment> {
  return request.post('/deployments', req)
}

/** 获取部署列表（分页） */
export function listDeployments(page?: number, pageSize?: number): Promise<PaginatedList<Deployment>> {
  return request.get('/deployments', { params: { page, page_size: pageSize } })
}

/** 撤销部署 */
export function undeploy(id: string): Promise<void> {
  return request.delete(`/deployments/${id}`)
}

/** 获取目标路径聚合列表（可按资源类型过滤） */
export function getTargets(type?: string): Promise<TargetInfo[]> {
  return request.get('/deployments/targets', { params: { type } })
}

/** 健康检查（返回broken项） */
export function checkHealth(): Promise<DeploymentItem[]> {
  return request.get('/deployments/health')
}

/** 修复部署项 */
export function repairItem(deploymentId: string, itemId: string): Promise<void> {
  return request.post(`/deployments/${deploymentId}/repair`, { item_id: itemId })
}

/** 清理部署项（undeploy=true 时同时撤销实际文件） */
export function cleanItem(deploymentId: string, itemId: string, undeploy = false): Promise<void> {
  return request.delete(`/deployments/${deploymentId}/items/${itemId}`, {
    params: undeploy ? { undeploy: 'true' } : undefined,
  })
}

/** 检查路径是否存在 */
export function checkPathExists(path: string): Promise<{ exists: boolean }> {
  return request.post('/deployments/check-path', { path })
}

/** 在系统文件管理器中打开目录 */
export function openFolder(path: string): Promise<void> {
  return request.post('/deployments/open-folder', { path })
}

/** 统计给定目标路径下的部署内容（编辑路径组删 config 路径前用）
 *  返回 { 路径: [资源名...] }，无部署的路径不出现 */
export function summarizeDeploymentsAtPaths(paths: string[]): Promise<Record<string, string[]>> {
  return request.post('/deployments/summarize-paths', { paths })
}

/** 撤销给定目标路径下的所有部署（确认移除时用） */
export function undeployAtPaths(paths: string[]): Promise<{ undeployed: number }> {
  return request.post('/deployments/undeploy-paths', { paths })
}

/** 资源保存后同步提示子项 —— 只含「包含当前资源」的部署，按路径组分组展示 */
export interface ResourceDeployTarget {
  preset_id: string
  preset_name: string
  path_group_name: string // 该部署 target_path 匹配到的路径组名（无则空）
  deployment_id: string
  deploy_type: string // symlink / merge
  target_path: string
  alias_name?: string
  resource_ids: string[]
  resource_names: string[]
  has_conflict: boolean
}
export function getResourceDeployTargets(resourceId: string): Promise<ResourceDeployTarget[]> {
  return request.get(`/deployments/by-resource/${resourceId}`)
}

/** 预检 Config 部署冲突（不写入文件） */
export interface ConflictItem {
  resource_id?: string
  resource_name: string
  status: 'ignored' | 'applied' | 'existing'
  group: number  // >0=冲突组号(同组内互冲突), 0=无冲突或已有内容
  conflict_for?: string  // 仅 existing 冲突：本次待部署且触发冲突的资源名
}
export interface CheckConflictsResp {
  has_conflict: boolean
  conflicts: ConflictItem[]
}
export function checkConflicts(data: { resource_ids: string[]; target_path?: string; alias_id?: string }): Promise<CheckConflictsResp> {
  return request.post('/deployments/check-conflicts', data)
}
