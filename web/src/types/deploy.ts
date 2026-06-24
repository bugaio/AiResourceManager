/** 部署记录 */
export interface Deployment {
  id: string
  group_id: string | null
  resource_id: string | null
  target_path: string
  alias_id: string | null
  deploy_type: 'symlink' | 'merge'
  track: number // 0 or 1
  created_at: string
  preset_id?: string | null
}

/** 部署目标下单个资源的部署状态 */
export interface DeployResourceStatus {
  resource_id: string
  resource_name: string
  type: 'skill' | 'agent' | 'config' | 'prompt' | string
  deployed: boolean
  stale: boolean
}

/** preset 在某路径组下、单个类型子路径的部署状态 */
export interface PresetTargetStatus {
  type: 'skill' | 'agent' | 'config' | 'prompt' | string
  target_path: string
  deployment_id: string
  track: number
  deploy_type: string
  has_deployment: boolean
  resources: DeployResourceStatus[]
}

/** preset 在某路径组下的完整部署状态 */
export interface PresetGroupStatus {
  group_id: string
  group_name: string
  targets: PresetTargetStatus[]
  pending: number
  stale: number
}

/** 部署明细项 */
export interface DeploymentItem {
  id: string
  deployment_id: string
  resource_id: string
  resource_name: string
  link_path: string
  status: 'ok' | 'broken'
  group_name: string
  group_color: string
}

/** 目标路径聚合 */
export interface TargetInfo {
  target_path: string
  deployments: DeploymentDetail[]
}

export interface DeploymentDetail extends Deployment {
  items: DeploymentItem[]
}

/** 部署请求 */
export interface DeployRequest {
  group_id?: string
  resource_id?: string
  resource_ids?: string[]
  target_path?: string
  alias_id?: string
  force?: boolean
  track?: boolean
  preset_id?: string
}
