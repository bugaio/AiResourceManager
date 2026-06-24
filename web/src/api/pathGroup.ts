import request from './request'
import type {
  PathGroup,
  CreatePathGroupReq,
  UpdatePathGroupReq,
} from '@/types/pathGroup'

/** 路径组列表 */
export function listPathGroups(): Promise<PathGroup[]> {
  return request.get('/path-groups')
}

/** 路径组详情 */
export function getPathGroup(id: string): Promise<PathGroup> {
  return request.get(`/path-groups/${id}`)
}

/** 创建路径组 */
export function createPathGroup(data: CreatePathGroupReq): Promise<PathGroup> {
  return request.post('/path-groups', data)
}

/** 更新路径组 */
export function updatePathGroup(id: string, data: UpdatePathGroupReq): Promise<void> {
  return request.put(`/path-groups/${id}`, data)
}

/** 删除路径组 */
export function deletePathGroup(id: string): Promise<void> {
  return request.delete(`/path-groups/${id}`)
}
