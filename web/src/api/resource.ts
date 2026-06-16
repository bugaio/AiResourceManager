import request from './request'
import type { ResourceType, Resource, PaginatedList } from '@/types/resource'

/** 查询资源列表参数 */
interface FetchResourcesParams {
  type: ResourceType
  search?: string
  group_id?: string
  page: number
  page_size: number
}

/** 获取资源列表（分页） */
export function fetchResources(params: FetchResourcesParams): Promise<PaginatedList<Resource>> {
  return request.get('/resources', { params })
}

/** 创建资源 */
export function createResource(data: { type: ResourceType; name: string; description?: string }): Promise<Resource> {
  return request.post('/resources', data)
}

/** 获取单个资源详情 */
export function getResource(id: string): Promise<Resource> {
  return request.get(`/resources/${id}`)
}

/** 更新资源信息 */
export function updateResource(id: string, data: { name?: string; description?: string }): Promise<void> {
  return request.put(`/resources/${id}`, data)
}

/** 获取资源内容（解包 content 字段） */
export async function getContent(id: string): Promise<string> {
  const res = await request.get(`/resources/${id}/content`)
  // 后端返回 {content: "..."}, 拦截器解包 data 后是 {content}，再取 .content
  if (res && typeof res === 'object' && 'content' in (res as any)) {
    return (res as any).content
  }
  return res as unknown as string
}

/** 更新资源内容 */
export function updateContent(id: string, content: string): Promise<void> {
  return request.put(`/resources/${id}/content`, { content })
}

/** 删除单个资源 */
export function deleteResource(id: string, confirm?: boolean): Promise<unknown> {
  return request.delete(`/resources/${id}`, { params: { confirm } })
}

/** 批量删除资源 */
export function batchDelete(ids: string[], confirm?: boolean): Promise<unknown> {
  return request.delete('/resources/batch', { data: { ids, confirm } })
}
