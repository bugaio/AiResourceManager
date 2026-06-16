// Package model group.go 定义分组相关的数据模型和请求/响应结构
package model

import "time"

// Group 分组实体，对应数据库 group 表
type Group struct {
	ID            string    `json:"id"`             // UUID 主键
	Name          string    `json:"name"`           // 分组名称
	Type          string    `json:"type"`           // 分组类型: skill/agent/mcp
	Color         string    `json:"color"`          // 分组颜色（hex）
	SortOrder     int       `json:"sort_order"`     // 排序权重
	ResourceCount int       `json:"resource_count"` // 分组内资源数量（运行时填充）
	CreatedAt     time.Time `json:"created_at"`     // 创建时间
	UpdatedAt     time.Time `json:"updated_at"`     // 更新时间
}

// CreateGroupReq 创建分组请求
type CreateGroupReq struct {
	Name string `json:"name" binding:"required"` // 分组名称
	Type string `json:"type" binding:"required"` // 分组类型: skill/agent/mcp
}

// UpdateGroupReq 更新分组请求
type UpdateGroupReq struct {
	Name      *string `json:"name"`       // 分组名称（可选）
	SortOrder *int    `json:"sort_order"` // 排序权重（可选）
}

// AddResourcesReq 添加资源到分组请求
type AddResourcesReq struct {
	ResourceIDs []string `json:"resource_ids" binding:"required"` // 资源 ID 列表
}

// GroupListResp 分组列表分页响应
type GroupListResp struct {
	List     []Group `json:"list"`      // 分组列表
	Total    int     `json:"total"`     // 总数
	Page     int     `json:"page"`      // 当前页码
	PageSize int     `json:"page_size"` // 每页数量
}
