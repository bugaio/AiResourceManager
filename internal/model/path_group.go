// Package model path_group.go 定义路径组 (Preset 专用) 的数据模型
package model

import "time"

// PathGroup 路径组：4 类资源各对应一个目标路径，配合 Preset 部署
type PathGroup struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	SkillPath  string    `json:"skill_path"`
	AgentPath  string    `json:"agent_path"`
	ConfigPath string    `json:"config_path"`
	PromptPath string    `json:"prompt_path"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// CreatePathGroupReq 创建路径组请求
type CreatePathGroupReq struct {
	Name       string `json:"name" binding:"required"`
	SkillPath  string `json:"skill_path"`
	AgentPath  string `json:"agent_path"`
	ConfigPath string `json:"config_path"`
	PromptPath string `json:"prompt_path"`
}

// UpdatePathGroupReq 更新路径组请求
// 任一字段为 nil 表示不更新；为 *"" 表示置空
type UpdatePathGroupReq struct {
	Name       *string `json:"name"`
	SkillPath  *string `json:"skill_path"`
	AgentPath  *string `json:"agent_path"`
	ConfigPath *string `json:"config_path"`
	PromptPath *string `json:"prompt_path"`
}
