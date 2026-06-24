/** 资源类型 */
export type ResourceType = 'skill' | 'agent' | 'config' | 'prompt'

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
  /** 私有资源归属的 preset（NULL=全局） */
  owner_preset_id?: string | null
  /** 关联此资源的 preset 列表（list 接口附带，用于显示 🔒） */
  preset_links?: Array<{ id: string; name: string }>
}

/** 分页响应 */
export interface PaginatedList<T> {
  list: T[]
  total: number
  page: number
  page_size: number
}
