import request from './request'

/** 导出结果 */
interface ExportResult {
  resource_count: number
  group_count: number
  preset_count: number
  file_count: number
  total_size: number
}

/** 导入结果 */
interface ImportResult {
  added: number
  overwritten: number
  skipped: number
  renamed: number
}

/** 导出数据到指定目录(产物为 git 友好的展开目录)
 *  clear=true 时，目标目录非空会先清除非隐藏文件再导出 */
export function exportData(targetPath: string, clear = false): Promise<ExportResult> {
  return request.post('/data/export', { target_path: targetPath, clear })
}

/** 从指定目录导入数据 */
export function importData(
  sourcePath: string,
  strategy: 'overwrite' | 'skip' | 'keep_both'
): Promise<ImportResult> {
  return request.post('/data/import', { source_path: sourcePath, strategy })
}
