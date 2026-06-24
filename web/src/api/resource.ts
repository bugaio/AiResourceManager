import request from './request'
import type { ResourceType, Resource, PaginatedList } from '@/types/resource'

/** 查询资源列表参数 */
interface FetchResourcesParams {
  type: ResourceType
  search?: string
  group_id?: string
  /** 私有资源归属过滤：传 preset_id 仅查该 preset 私有；传 "" 不过滤；不传 → 后端默认只返回全局 */
  owner_preset_id?: string
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

/** 删除单个资源
 * @param confirm true=已确认部署提示
 * @param unlink  true=级联解除所有 preset 关联后再删
 */
export function deleteResource(
  id: string,
  options?: { confirm?: boolean; unlink?: boolean },
): Promise<unknown> {
  const params: Record<string, boolean> = {}
  if (options?.confirm) params.confirm = true
  if (options?.unlink) params.unlink = true
  return request.delete(`/resources/${id}`, { params })
}

/** 批量删除资源 */
export function batchDelete(ids: string[], confirm?: boolean): Promise<unknown> {
  return request.delete('/resources/batch', { data: { ids, confirm } })
}

/** 导入 skill: 整目录原样上传,后端落到 ~/.aiManager/skills/{uuid}/
 * @param params.name 从 SKILL.md frontmatter 解析的名称
 * @param params.description 从 frontmatter 解析的描述(可选)
 * @param params.groupId 关联分组(可选)
 * @param params.files 该 skill 子目录下所有文件(File 来自 webkitdirectory)
 * @param params.relPaths 与 files 一一对应的相对路径(相对子目录,如 "SKILL.md"、"assets/x.png")
 */
export function importSkill(params: {
  name: string
  description?: string
  groupId?: string
  files: File[]
  relPaths: string[]
}): Promise<Resource> {
  const fd = new FormData()
  fd.append('name', params.name)
  if (params.description) fd.append('description', params.description)
  if (params.groupId) fd.append('group_id', params.groupId)
  for (let i = 0; i < params.files.length; i++) {
    fd.append('files', params.files[i])
    fd.append('paths', params.relPaths[i])
  }
  return request.post('/resources/import-skill', fd, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
}

/** 导入 agent: 单个 .md 文件原样上传,后端落到 ~/.aiManager/agents/{uuid}.md
 * @param params.name 从 frontmatter 解析的名称
 * @param params.description 从 frontmatter 解析的描述(可空)
 * @param params.groupId 关联分组(可选)
 * @param params.file 源 .md 文件
 */
export function importAgent(params: {
  name: string
  description?: string
  groupId?: string
  file: File
}): Promise<Resource> {
  const fd = new FormData()
  fd.append('name', params.name)
  if (params.description) fd.append('description', params.description)
  if (params.groupId) fd.append('group_id', params.groupId)
  fd.append('file', params.file)
  return request.post('/resources/import-agent', fd, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
}
