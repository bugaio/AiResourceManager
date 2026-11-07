/** 路径组：4 个子路径的命名组合（config / mdconfig 支持多条） */
export interface PathGroup {
  id: string
  name: string
  skill_path: string
  mdconfig_path: string
  mdconfig_paths: string[]
  config_path: string
  config_paths: string[]
  prompt_path: string
  created_at: string
  updated_at: string
}

export interface CreatePathGroupReq {
  name: string
  skill_path?: string
  mdconfig_path?: string
  mdconfig_paths?: string[]
  config_path?: string
  config_paths?: string[]
  prompt_path?: string
}

export interface UpdatePathGroupReq {
  name?: string
  skill_path?: string
  mdconfig_path?: string
  mdconfig_paths?: string[]
  config_path?: string
  config_paths?: string[]
  prompt_path?: string
}
