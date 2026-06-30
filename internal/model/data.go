// Package model data.go 定义数据导入导出相关的请求和响应结构
//
// 设计目标：导出/导入是一对可逆操作，产物为 git 友好的展开目录：
//
//	{targetPath}/
//	├── manifest.json   — 格式版本 / 导出时间 / 数量统计
//	├── data.json       — 全部关系数据(资源/分组/preset 及其关联)
//	└── files/          — 实体文件，按 ~/.aiManager 下的相对路径镜像存放
//
// 仅同步「资源实体 + 分组 + preset 关联(含私有资源)」，
// 不含任何部署信息(deployment / deployment_item / path_group / path_alias)。
package model

// ExportRequest 数据导出请求
type ExportRequest struct {
	TargetPath string `json:"target_path" binding:"required"` // 导出目标目录(用户可对其 git init)
	Clear      bool   `json:"clear"`                          // 目标非空时是否先清除(隐藏文件如 .git 保留)
}

// ExportResult 数据导出结果
type ExportResult struct {
	ResourceCount int   `json:"resource_count"` // 导出资源数(含私有)
	GroupCount    int   `json:"group_count"`    // 导出分组数
	PresetCount   int   `json:"preset_count"`   // 导出 preset 数
	FileCount     int   `json:"file_count"`     // 导出实体文件数
	TotalSize     int64 `json:"total_size"`     // 实体文件总大小(字节)
}

// ImportRequest 数据导入请求
type ImportRequest struct {
	SourcePath string `json:"source_path" binding:"required"` // 导入源目录(含 data.json)
	Strategy   string `json:"strategy" binding:"required"`    // 冲突策略: skip / overwrite / keep_both
}

// ImportResult 数据导入结果
type ImportResult struct {
	Added       int `json:"added"`       // 新增数量
	Overwritten int `json:"overwritten"` // 覆盖数量
	Skipped     int `json:"skipped"`     // 跳过数量
	Renamed     int `json:"renamed"`     // keep_both 下生成副本的数量
}

// GroupResourceLink 分组↔资源关联(导入导出用)
type GroupResourceLink struct {
	GroupID    string `json:"group_id"`
	ResourceID string `json:"resource_id"`
}

// PresetResourceLink preset↔资源关联(导入导出用)
type PresetResourceLink struct {
	PresetID   string `json:"preset_id"`
	ResourceID string `json:"resource_id"`
}
