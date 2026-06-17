// Package model alias.go 定义路径别名相关的数据模型和请求结构
package model

import "time"

// PathAlias 路径别名实体，对应数据库 path_alias 表
type PathAlias struct {
	ID        string    `json:"id"`         // UUID 主键
	Name      string    `json:"name"`       // 别名名称
	Type      string    `json:"type"`       // 资源类型 skill/agent/config，别名按类型隔离
	Path      string    `json:"path"`       // 目标路径
	CreatedAt time.Time `json:"created_at"` // 创建时间
}

// CreateAliasReq 创建路径别名请求
type CreateAliasReq struct {
	Name string `json:"name" binding:"required"` // 别名名称
	Type string `json:"type" binding:"required"` // 资源类型 skill/agent/config
	Path string `json:"path" binding:"required"` // 目标路径
}

// UpdateAliasReq 更新路径别名请求
type UpdateAliasReq struct {
	Name string `json:"name" binding:"required"` // 别名名称
	Path string `json:"path" binding:"required"` // 目标路径
}
