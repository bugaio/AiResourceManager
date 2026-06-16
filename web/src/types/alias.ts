import type { ResourceType } from '@/types/resource'

export interface PathAlias {
  id: string
  name: string
  type: ResourceType
  path: string
  created_at: string
}
