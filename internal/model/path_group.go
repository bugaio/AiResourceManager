// Package model path_group.go 定义路径组 (Preset 专用) 的数据模型
package model

import "time"

// PathGroup 路径组：4 类资源各对应一个目标路径，配合 Preset 部署
//
// ConfigPaths 是 config 目标的真相源（可多条，preset 内多个 config 可分别部署）。
// ConfigPath 保留为 ConfigPaths[0] 的镜像，向后兼容仍按单值读取的旧逻辑。
type PathGroup struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	SkillPath   string    `json:"skill_path"`
	AgentPath   string    `json:"agent_path"`
	ConfigPath  string    `json:"config_path"`  // = ConfigPaths[0]（兼容镜像）
	ConfigPaths []string  `json:"config_paths"` // config 目标路径列表（真相源）
	PromptPath  string    `json:"prompt_path"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreatePathGroupReq 创建路径组请求
type CreatePathGroupReq struct {
	Name        string   `json:"name" binding:"required"`
	SkillPath   string   `json:"skill_path"`
	AgentPath   string   `json:"agent_path"`
	ConfigPath  string   `json:"config_path"`  // 兼容旧客户端：单条 config
	ConfigPaths []string `json:"config_paths"` // 多条 config（优先生效）
	PromptPath  string   `json:"prompt_path"`
}

// UpdatePathGroupReq 更新路径组请求
// 任一字段为 nil 表示不更新；为 *"" 表示置空
type UpdatePathGroupReq struct {
	Name        *string   `json:"name"`
	SkillPath   *string   `json:"skill_path"`
	AgentPath   *string   `json:"agent_path"`
	ConfigPath  *string   `json:"config_path"`
	ConfigPaths *[]string `json:"config_paths"` // 非 nil 时整体替换 config 路径列表
	PromptPath  *string   `json:"prompt_path"`
}
