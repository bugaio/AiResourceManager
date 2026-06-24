/** 路径格式校验工具：与后端 alias 校验同口径 */

/** 支持的配置文件后缀 */
export const CONFIG_SUFFIXES = ['.json', '.jsonc', '.yaml', '.yml', '.toml']

/** 是否为配置文件路径（支持的后缀） */
export function isConfigFilePath(p: string): boolean {
  const lower = p.toLowerCase().trim()
  return CONFIG_SUFFIXES.some((ext) => lower.endsWith(ext))
}

/** 是否为 Markdown 文件路径 */
export function isPromptFilePath(p: string): boolean {
  return /\.md$/i.test(p.trim())
}

/** skill / agent 路径必须是目录（不能以已知文件后缀结尾） */
const FILE_LIKE_SUFFIXES = [
  ...CONFIG_SUFFIXES,
  '.md',
  '.txt',
  '.sh',
  '.py',
  '.js',
  '.ts',
]
export function isDirectoryPath(p: string): boolean {
  const lower = p.toLowerCase().trim()
  if (!lower) return false
  return !FILE_LIKE_SUFFIXES.some((ext) => lower.endsWith(ext))
}

/** 校验路径组 4 子路径，返回错误消息(空字符串表示通过) */
export function validatePathGroupPaths(p: {
  skill_path: string
  agent_path: string
  config_path: string
  prompt_path: string
}): string {
  const skill = (p.skill_path || '').trim()
  const agent = (p.agent_path || '').trim()
  const config = (p.config_path || '').trim()
  const prompt = (p.prompt_path || '').trim()

  if (!skill && !agent && !config && !prompt) return '至少需要填写一个子路径'

  if (skill && !isDirectoryPath(skill)) return 'skill_path 必须是目录路径（不能带文件后缀）'
  if (agent && !isDirectoryPath(agent)) return 'agent_path 必须是目录路径（不能带文件后缀）'
  if (config && !isConfigFilePath(config))
    return 'config_path 后缀必须是 .json/.jsonc/.yaml/.yml/.toml'
  if (prompt && !isPromptFilePath(prompt)) return 'prompt_path 后缀必须是 .md'

  return ''
}

/** 随机路径组名前缀（手动部署自动建组时使用） */
export const RANDOM_GROUP_PREFIX = '部署-'

/** 生成随机路径组名（时间戳 + 随机后缀，保证唯一） */
export function genRandomGroupName(): string {
  const ts = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  const stamp = `${ts.getFullYear()}${pad(ts.getMonth() + 1)}${pad(ts.getDate())}${pad(ts.getHours())}${pad(ts.getMinutes())}${pad(ts.getSeconds())}`
  const rand = Math.random().toString(36).slice(2, 6)
  return `${RANDOM_GROUP_PREFIX}${stamp}-${rand}`
}

/** 判断某路径组名是否为系统生成的随机名（非用户绑定的正式别名） */
export function isRandomGroupName(name: string): boolean {
  return /^部署-\d{14}-[a-z0-9]{4}$/.test(name || '')
}
