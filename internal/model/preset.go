// Package model preset.go 定义 Preset 模块的数据模型与请求结构
package model

import "time"

// Preset 预设：跨 4 种资源类型的合集，作为独立的部署单元
type Preset struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	// 运行时统计字段
	ResourceCount int          `json:"resource_count"` // 关联 + 私有
	PrivateCount  int          `json:"private_count"`  // 私有资源数
	LinkedCount   int          `json:"linked_count"`   // 关联资源数
	Deployments   []Deployment `json:"deployments"`    // 该 preset 的所有部署记录
	// GroupDrifts 该 preset 在每个已部署路径组下的漂移汇总，key=路径组 ID
	GroupDrifts map[string]PresetGroupDrift `json:"group_drifts,omitempty"`
}

// PresetConfigConflict 一个候选 config 与 preset 中已有 config 的冲突详情
type PresetConfigConflict struct {
	ResourceID    string                     `json:"resource_id"`    // 候选 config 资源 ID（私有草稿场景为空）
	ResourceName  string                     `json:"resource_name"`  // 候选 config 名称
	ConflictsWith []PresetConfigConflictItem `json:"conflicts_with"` // 与之冲突的已有 config 列表
}

// PresetConfigConflictItem 已有 config 的冲突项
type PresetConfigConflictItem struct {
	ResourceID   string `json:"resource_id"`
	ResourceName string `json:"resource_name"`
}

// CheckPresetConfigConflictsReq 检测候选 config 与 preset 已有 config 冲突的请求
type CheckPresetConfigConflictsReq struct {
	// CandidateIDs 待检测的候选 config 资源 ID（关联场景：待关联的全局 config；
	// 编辑场景：正在保存的私有 config 自身 ID）
	CandidateIDs []string `json:"candidate_ids"`
}

// CheckPresetConfigConflictsResp 冲突检测响应
type CheckPresetConfigConflictsResp struct {
	HasConflict bool                   `json:"has_conflict"`
	Conflicts   []PresetConfigConflict `json:"conflicts"` // 仅含真正冲突的候选
}

// CreatePresetReq 创建 Preset 请求
type CreatePresetReq struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// UpdatePresetReq 更新 Preset 请求
type UpdatePresetReq struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

// LinkResourcesReq 关联资源到 Preset 请求
type LinkResourcesReq struct {
	ResourceIDs []string `json:"resource_ids" binding:"required"`
}

// UnlinkResourcesReq 解除资源与 Preset 关联请求
type UnlinkResourcesReq struct {
	ResourceIDs []string `json:"resource_ids" binding:"required"`
}

// PresetLinkInfo 资源被 preset 锁定时返回的精简信息
type PresetLinkInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ResourceDeleteBlockedData 资源删除被 preset 拦截时附带的数据
type ResourceDeleteBlockedData struct {
	Presets []PresetLinkInfo `json:"presets"`
}

// PathSpec 部署 Preset 时手动填写的路径四元组
//
// ConfigPaths 为多条 config 目标（手动模式多路径）；ConfigPath 为单条兼容值。
// 优先使用 ConfigPaths。
type PathSpec struct {
	SkillPath   string   `json:"skill_path"`
	AgentPath   string   `json:"agent_path"`
	ConfigPath  string   `json:"config_path"`
	ConfigPaths []string `json:"config_paths"`
	PromptPath  string   `json:"prompt_path"`
}

// DeployPresetReq 部署 Preset 请求
type DeployPresetReq struct {
	PathGroupID *string   `json:"path_group_id"` // 选择已有路径组
	ManualPaths *PathSpec `json:"manual_paths"`  // 或手动填写
	Track       bool      `json:"track"`         // 是否开启追踪
	// ConfigAssignments config 资源 ID → 目标路径的分配映射。
	// 当路径组/手动填写有多条 config 路径时，前端弹窗让每个 config 选定目标后回传。
	// 单条 config 路径时可为空（后端自动归一到那一条）。
	ConfigAssignments map[string]string `json:"config_assignments"`
}

// CreatePrivateResourceReq 创建 Preset 私有资源请求
type CreatePrivateResourceReq struct {
	Type        string `json:"type" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}
