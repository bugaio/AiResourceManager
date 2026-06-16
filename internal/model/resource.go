// Package model resource.go 定义资源相关的数据模型和请求/响应结构
package model

import "time"

// Resource 资源实体，对应数据库 resource 表
type Resource struct {
	ID          string    `json:"id"`          // UUID 主键
	Name        string    `json:"name"`        // 资源名称
	Type        string    `json:"type"`        // 资源类型: skill/agent/mcp
	Path        string    `json:"path"`        // 文件系统路径
	Description string    `json:"description"` // 资源描述
	Metadata    string    `json:"metadata"`    // 扩展元数据 JSON
	CreatedAt   time.Time `json:"created_at"`  // 创建时间
	UpdatedAt   time.Time `json:"updated_at"`  // 更新时间
}

// CreateResourceReq 创建资源请求
type CreateResourceReq struct {
	Type        string `json:"type" binding:"required"`        // 资源类型: skill/agent/mcp
	Name        string `json:"name" binding:"required"`        // 资源名称
	Description string `json:"description"`                    // 资源描述（可选）
	GroupID     string `json:"group_id"`                       // 所属分组 ID（可选）
}

// UpdateResourceReq 更新资源元数据请求
type UpdateResourceReq struct {
	Name        *string `json:"name"`        // 资源名称（可选）
	Description *string `json:"description"` // 资源描述（可选）
}

// UpdateContentReq 更新资源文件内容请求
type UpdateContentReq struct {
	Content string `json:"content"` // 文件内容
}

// BatchDeleteReq 批量删除请求
type BatchDeleteReq struct {
	IDs     []string `json:"ids" binding:"required"`     // 资源 ID 列表
	Confirm bool     `json:"confirm"`                    // 是否确认级联删除
}

// ResourceListResp 资源列表分页响应
type ResourceListResp struct {
	List     []Resource `json:"list"`      // 资源列表
	Total    int        `json:"total"`     // 总数
	Page     int        `json:"page"`      // 当前页码
	PageSize int        `json:"page_size"` // 每页数量
}

// DeploymentInfo 部署关联信息，用于删除前检查
type DeploymentInfo struct {
	ID         string `json:"id"`          // 部署 ID
	TargetPath string `json:"target_path"` // 部署目标路径
}

// BatchDeleteResult 批量删除单项结果
type BatchDeleteResult struct {
	ID          string           `json:"id"`                    // 资源 ID
	Success     bool             `json:"success"`               // 是否成功
	Code        int              `json:"code,omitempty"`        // 错误码（失败时）
	Msg         string           `json:"msg,omitempty"`         // 错误消息（失败时）
	Deployments []DeploymentInfo `json:"deployments,omitempty"` // 关联部署（有关联时）
}
