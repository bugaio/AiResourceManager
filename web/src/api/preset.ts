import request from './request'
import type {
  Preset,
  CreatePresetReq,
  UpdatePresetReq,
  DeployPresetReq,
  PresetResource,
} from '@/types/preset'
import type { Deployment, PresetGroupStatus } from '@/types/deploy'
import type { Resource, ResourceType } from '@/types/resource'

/** Preset 列表 */
export function listPresets(): Promise<Preset[]> {
  return request.get('/presets')
}

/** Preset 详情 */
export function getPreset(id: string): Promise<Preset & { resources: PresetResource[] }> {
  return request.get(`/presets/${id}`)
}

/** 创建 Preset */
export function createPreset(data: CreatePresetReq): Promise<Preset> {
  return request.post('/presets', data)
}

/** 更新 Preset */
export function updatePreset(id: string, data: UpdatePresetReq): Promise<void> {
  return request.put(`/presets/${id}`, data)
}

/** 删除 Preset */
export function deletePreset(id: string): Promise<void> {
  return request.delete(`/presets/${id}`)
}

/** Preset 关联资源（仅全局资源可关联） */
export function linkResources(presetID: string, resourceIDs: string[]): Promise<void> {
  return request.post(`/presets/${presetID}/resources`, { resource_ids: resourceIDs })
}

/** Preset 取消关联资源 */
export function unlinkResources(presetID: string, resourceIDs: string[]): Promise<void> {
  return request.delete(`/presets/${presetID}/resources`, { data: { resource_ids: resourceIDs } })
}

/** 候选 config 与 preset 已有 config 的冲突项 */
export interface PresetConfigConflict {
  resource_id: string
  resource_name: string
  conflicts_with: { resource_id: string; resource_name: string }[]
}

export interface CheckConfigConflictsResp {
  has_conflict: boolean
  conflicts: PresetConfigConflict[]
}

/** 检测候选 config 与 preset 已有 config 是否冲突（关联/编辑前预检） */
export function checkPresetConfigConflicts(
  presetID: string,
  candidateIDs: string[],
): Promise<CheckConfigConflictsResp> {
  return request.post(`/presets/${presetID}/check-config-conflicts`, { candidate_ids: candidateIDs })
}

/** Preset 下的资源列表 */
export function listPresetResources(presetID: string): Promise<PresetResource[]> {
  return request.get(`/presets/${presetID}/resources`)
}

/** 创建 preset 私有资源（普通字段，不含文件） */
export function createPrivateResource(
  presetID: string,
  data: { type: ResourceType; name: string; description?: string },
): Promise<Resource> {
  return request.post(`/presets/${presetID}/private-resources`, data)
}

/** 删除 preset 下的单个私有资源 */
export function deletePrivateResource(presetID: string, resourceID: string): Promise<void> {
  return request.delete(`/presets/${presetID}/private-resources/${resourceID}`)
}

/** 导入私有 skill（multipart） */
export function importPrivateSkill(
  presetID: string,
  params: { name: string; description?: string; files: File[]; relPaths: string[] },
): Promise<Resource> {
  const fd = new FormData()
  fd.append('name', params.name)
  if (params.description) fd.append('description', params.description)
  for (let i = 0; i < params.files.length; i++) {
    fd.append('files', params.files[i])
    fd.append('paths', params.relPaths[i])
  }
  return request.post(`/presets/${presetID}/import-private-skill`, fd, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
}

/** 导入私有 agent（multipart） */
export function importPrivateAgent(
  presetID: string,
  params: { name: string; description?: string; file: File },
): Promise<Resource> {
  const fd = new FormData()
  fd.append('name', params.name)
  if (params.description) fd.append('description', params.description)
  fd.append('file', params.file)
  return request.post(`/presets/${presetID}/import-private-agent`, fd, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
}

/** 部署 preset */
export function deployPreset(presetID: string, body: DeployPresetReq): Promise<Deployment[]> {
  return request.post(`/presets/${presetID}/deploy`, body)
}

/** 重新部署 preset（复用已有 target_path） */
export function redeployPreset(presetID: string): Promise<Deployment[]> {
  return request.post(`/presets/${presetID}/redeploy`)
}

/** 撤销某次 preset 部署 */
export function undeployPresetDeployment(presetID: string, deploymentID: string): Promise<void> {
  return request.delete(`/presets/${presetID}/deploy/${deploymentID}`)
}

/** preset 在某路径组下的完整部署状态（部署管理弹窗：已部署/新增/残留） */
export function getPresetGroupStatus(
  presetID: string,
  groupID: string,
): Promise<PresetGroupStatus> {
  return request.get(`/presets/${presetID}/groups/${groupID}/status`)
}

/** 将 preset 以最新全量资源重新部署到指定路径组（补齐新增类型，如新加的 prompt）
 *  configAssignments: config 资源 ID → 目标路径（多条 config 路径时由弹窗回传，可空） */
export function redeployPresetGroup(
  presetID: string,
  groupID: string,
  configAssignments?: Record<string, string>,
): Promise<Deployment[]> {
  return request.post(`/presets/${presetID}/groups/${groupID}/redeploy`, {
    config_assignments: configAssignments,
  })
}
