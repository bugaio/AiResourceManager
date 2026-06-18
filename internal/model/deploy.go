// Package model deploy.go 定义部署相关的数据模型和请求/响应结构
package model

// Deployment 部署记录
type Deployment struct {
	ID         string  `json:"id"`
	GroupID    *string `json:"group_id"`    // 整组部署时非空
	ResourceID *string `json:"resource_id"` // 单资源部署时非空
	TargetPath string  `json:"target_path"`
	AliasID    *string `json:"alias_id"`
	DeployType string  `json:"deploy_type"` // symlink / merge
	Track      int     `json:"track"`       // 0=static, 1=track
	CreatedAt  string  `json:"created_at"`
}

// DeploymentItem 部署明细
type DeploymentItem struct {
	ID           string `json:"id"`
	DeploymentID string `json:"deployment_id"`
	ResourceID   string `json:"resource_id"`
	ResourceName string `json:"resource_name"`  // 资源名称（运行时填充）
	LinkPath     string `json:"link_path"`
	Status       string `json:"status"`         // ok / broken (runtime, not stored)
	GroupName    string `json:"group_name"`     // 关联分组名称（运行时填充）
	GroupColor   string `json:"group_color"`    // 关联分组颜色（运行时填充）
}

// DeployRequest 部署请求
type DeployRequest struct {
	GroupID     *string  `json:"group_id"`
	ResourceID  *string  `json:"resource_id"`
	ResourceIDs []string `json:"resource_ids"` // 批量部署多个资源
	TargetPath  string   `json:"target_path"`
	AliasID     *string  `json:"alias_id"`
	Force       bool     `json:"force"`
	Track       bool     `json:"track"`
}

// TargetInfo 目标路径聚合信息
type TargetInfo struct {
	TargetPath  string             `json:"target_path"`
	Deployments []DeploymentDetail `json:"deployments"`
}

// DeploymentDetail 部署详情（含明细列表）
type DeploymentDetail struct {
	Deployment
	Items []DeploymentItem `json:"items"`
}

// MetaJSON _meta.json 结构
type MetaJSON struct {
	ManagedBy   string           `json:"managed_by"`
	Version     int              `json:"version"`
	Deployments []MetaDeployment `json:"deployments"`
}

// MetaDeployment _meta.json 中单条部署记录
type MetaDeployment struct {
	DeploymentID string `json:"deployment_id"`
	Type         string `json:"type"`
	ResourceUUID string `json:"resource_uuid"`
	ResourceName string `json:"resource_name"`
	LinkPath     string `json:"link_path,omitempty"`
	MCPKey       string `json:"mcp_key,omitempty"`
	DeployedAt   string `json:"deployed_at"`
}

// DeployListResp 部署列表分页响应
type DeployListResp struct {
	List     []Deployment `json:"list"`      // 部署列表
	Total    int          `json:"total"`     // 总数
	Page     int          `json:"page"`      // 当前页码
	PageSize int          `json:"page_size"` // 每页数量
}

// RepairRequest 修复请求
type RepairRequest struct {
	ItemID string `json:"item_id"` // 部署子项 ID
}

// ResourceDeployTarget 资源已部署到的目标信息（用于内容保存后同步提示）
type ResourceDeployTarget struct {
	DeploymentID string `json:"deployment_id"`
	TargetPath   string `json:"target_path"`
	AliasName    string `json:"alias_name,omitempty"` // 若该路径有别名，填别名名称
	HasConflict  bool   `json:"has_conflict"`         // 目标中已存在同 key（冲突）
}

// CheckConflictsReq 预检冲突请求
//
// 字段语义:
//   - target_path / alias_id: 单目标场景 (config 模块沿用)
//   - target_paths:           多目标场景 (prompt 模块, 一次部署到多个 .md 文件)
//     非空时优先生效, 与 target_path/alias_id 互斥
type CheckConflictsReq struct {
	ResourceIDs []string `json:"resource_ids" binding:"required"`
	TargetPath  string   `json:"target_path"`
	TargetPaths []string `json:"target_paths"`
	AliasID     *string  `json:"alias_id"`
}

// ConflictItem 单条冲突信息
type ConflictItem struct {
	ResourceID   string `json:"resource_id,omitempty"` // 资源 ID（待部署资源有，已有内容无）
	ResourceName string `json:"resource_name"`         // 冲突方名称
	TargetPath   string `json:"target_path,omitempty"` // 关联目标路径 (prompt 多目标场景)
	Status       string `json:"status"`                // ignored=冲突被忽略(红), applied=实际应用(绿), existing=已有内容冲突
	Group        int    `json:"group"`                 // 冲突组号（>0 表示属于同一冲突组，0=无冲突/已有内容）
}

// CheckConflictsResp 预检冲突响应
type CheckConflictsResp struct {
	HasConflict bool           `json:"has_conflict"`
	Conflicts   []ConflictItem `json:"conflicts"`
}
