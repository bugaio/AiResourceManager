import request from './request'
import type { PathAlias } from '@/types/alias'

/** 获取别名列表（可按资源类型过滤） */
export function fetchAliases(type?: string): Promise<PathAlias[]> {
  return request.get('/aliases', { params: { type } })
}

/** 创建别名 */
export function createAlias(data: { name: string; type: string; path: string }): Promise<PathAlias> {
  return request.post('/aliases', data)
}

/** 更新别名 */
export function updateAlias(id: string, data: { name?: string; path?: string }): Promise<void> {
  return request.put(`/aliases/${id}`, data)
}

/** 删除别名 */
export function deleteAlias(id: string): Promise<void> {
  return request.delete(`/aliases/${id}`)
}
