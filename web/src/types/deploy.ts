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
}
