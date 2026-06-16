// Package model data.go 定义数据导入导出相关的请求和响应结构
package model

// ExportRequest 数据导出请求
type ExportRequest struct {
	TargetPath string `json:"target_path" binding:"required"` // 导出目标目录
}

// ExportResult 数据导出结果
type ExportResult struct {
	FileCount int   `json:"file_count"` // 导出文件数
	TotalSize int64 `json:"total_size"` // 导出总大小（字节）
}

// ImportRequest 数据导入请求
type ImportRequest struct {
	SourcePath string `json:"source_path" binding:"required"` // 导入源目录
	Strategy   string `json:"strategy" binding:"required"`    // 策略: overwrite / skip / keep_both
}

// ImportResult 数据导入结果
type ImportResult struct {
	Added       int `json:"added"`       // 新增数量
	Overwritten int `json:"overwritten"` // 覆盖数量
	Skipped     int `json:"skipped"`     // 跳过数量
}
