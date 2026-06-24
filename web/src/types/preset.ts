import type { Resource } from './resource'
import type { Deployment } from './deploy'

/** Preset 主体 */
export interface Preset {
  id: string
  name: string
  description: string
  created_at: string
  updated_at: string
  /** 关联+私有资源总数 */
  resource_count: number
  /** 私有资源数 */
  private_count: number
  /** 关联资源数 */
  linked_count: number
  /** 该 preset 的所有部署记录 */
  deployments?: Deployment[]
  /** 该 preset 在每个已部署路径组下的漂移汇总，key=路径组 ID */
  group_drifts?: Record<string, PresetGroupDrift>
}

/** preset 在某路径组下的漂移汇总 */
export interface PresetGroupDrift {
  group_id: string
  group_name: string
  /** preset 已有但该路径组尚未部署的资源数（含缺失类型） */
  pending: number
  /** 该路径组残留但已不在 preset 的资源数 */
  stale: number
}

/** Preset 视图下的资源（带归属信息） */
export interface PresetResource extends Resource {
  /** owner_preset_id 非空 → 私有；空 → 关联 */
  owner_preset_id?: string | null
}

/** Preset 关联信息（用于资源被锁后展示） */
export interface PresetLinkInfo {
  id: string
  name: string
}

/** 创建 Preset 请求 */
export interface CreatePresetReq {
  name: string
  description?: string
}

/** 更新 Preset 请求 */
export interface UpdatePresetReq {
  name?: string
  description?: string
}

/** 部署 Preset 请求 */
export interface DeployPresetReq {
  /** 已有路径组 ID（与 manual_paths 二选一） */
  path_group_id?: string
  /** 手动填写的 4 个子路径（与 path_group_id 二选一） */
  manual_paths?: {
    skill_path?: string
    agent_path?: string
    config_path?: string
    prompt_path?: string
  }
  /** 是否启用跟踪 */
  track?: boolean
  /** 强制覆盖（重新部署时使用） */
  force?: boolean
}
