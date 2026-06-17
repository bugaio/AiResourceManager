/** 资源类型 */
export type ResourceType = 'skill' | 'agent' | 'config'

/** 资源接口 */
export interface Resource {
  id: string
  uuid: string
  type: ResourceType
  name: string
  description: string
  path: string
  file_path: string
  created_at: string
  updated_at: string
}

/** 分页响应 */
export interface PaginatedList<T> {
  list: T[]
  total: number
  page: number
  page_size: number
}
