import request from './request'
import type { ResourceType } from '@/types/resource'
import type { Group } from '@/types/group'

/** 分组列表分页响应 */
export interface GroupListResp {
  list: Group[]
  total: number
  page: number
  page_size: number
}

/** 获取分组列表 */
export function fetchGroups(params: { type: ResourceType; page?: number; page_size?: number }): Promise<GroupListResp> {
  return request.get('/groups', { params })
}

/** 创建分组 */
export function createGroup(data: { name: string; type: ResourceType }): Promise<Group> {
  return request.post('/groups', data)
}

/** 更新分组 */
export function updateGroup(id: string, data: { name?: string; sort_order?: number }): Promise<void> {
  return request.put(`/groups/${id}`, data)
}

/** 删除分组 */
export function deleteGroup(id: string): Promise<void> {
  return request.delete(`/groups/${id}`)
}

/** 向分组添加资源 */
export function addResources(groupId: string, resourceIds: string[]): Promise<void> {
  return request.post(`/groups/${groupId}/resources`, { resource_ids: resourceIds })
}

/** 从分组移除资源 */
export function removeResource(groupId: string, resourceId: string): Promise<void> {
  return request.delete(`/groups/${groupId}/resources/${resourceId}`)
}
