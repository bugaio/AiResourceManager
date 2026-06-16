import type { ResourceType } from './resource'

/** 分组接口 */
export interface Group {
  id: string
  name: string
  type: ResourceType
  color: string
  sort_order: number
  resource_count: number
  created_at: string
}
