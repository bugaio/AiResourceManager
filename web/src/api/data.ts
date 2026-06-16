import request from './request'

/** 导出结果 */
interface ExportResult {
  file_count: number
  total_size: number
}

/** 导入结果 */
interface ImportResult {
  added: number
  overwritten: number
  skipped: number
}

/** 导出数据到指定目录 */
export function exportData(targetPath: string): Promise<ExportResult> {
  return request.post('/data/export', { target_path: targetPath })
}

/** 从指定目录导入数据 */
export function importData(
  sourcePath: string,
  strategy: 'overwrite' | 'skip' | 'keep_both'
): Promise<ImportResult> {
  return request.post('/data/import', { source_path: sourcePath, strategy })
}
