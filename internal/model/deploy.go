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
	PresetID   *string `json:"preset_id"`   // preset 整体部署时非空
}

// DeployResourceStatus 单个部署目标下的资源及其部署状态（重新部署预览用）
type DeployResourceStatus struct {
	ResourceID   string `json:"resource_id"`
	ResourceName string `json:"resource_name"`
	Type         string `json:"type"`
	Deployed     bool   `json:"deployed"` // 已部署到该目标
	Stale        bool   `json:"stale"`    // 已从 preset 移除但目标仍残留
}

// PresetGroupDrift preset 在某路径组下的漂移汇总（运行时计算，附在 Preset 上）
type PresetGroupDrift struct {
	GroupID   string `json:"group_id"`
	GroupName string `json:"group_name"`
	Pending   int    `json:"pending"` // preset 已有但该路径组尚未部署的资源数（含缺失类型）
	Stale     int    `json:"stale"`   // 该路径组残留但已不在 preset 的资源数
}

// PresetTargetStatus preset 在某路径组下、单个类型子路径的部署状态
type PresetTargetStatus struct {
	Type          string                 `json:"type"`
	TargetPath    string                 `json:"target_path"`
	DeploymentID  string                 `json:"deployment_id"` // 空=该类型尚未部署
	Track         int                    `json:"track"`
	DeployType    string                 `json:"deploy_type"`
	HasDeployment bool                   `json:"has_deployment"`
	Resources     []DeployResourceStatus `json:"resources"`
}

// PresetGroupStatus preset 在某路径组下的完整部署状态（部署管理弹窗用）
type PresetGroupStatus struct {
	GroupID   string               `json:"group_id"`
	GroupName string               `json:"group_name"`
	Targets   []PresetTargetStatus `json:"targets"`
	Pending   int                  `json:"pending"`
	Stale     int                  `json:"stale"`
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
	PresetID    *string  `json:"preset_id"` // preset 整体部署时非空
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

// ResourceDeployTarget 资源保存后同步提示用 —— 只返回「包含当前资源」的部署
// 前端按路径组分组展示：路径组名作主标题,子行显示当前资源名 + 部署子路径
type ResourceDeployTarget struct {
	PresetID      string   `json:"preset_id"`               // 所属 preset ID(直接部署无 preset 时为空)
	PresetName    string   `json:"preset_name"`             // preset 别名
	PathGroupName string   `json:"path_group_name"`         // 该部署 target_path 匹配到的路径组名(无则空)
	DeploymentID  string   `json:"deployment_id"`           // 对应 deployment ID
	DeployType    string   `json:"deploy_type"`             // symlink / merge
	TargetPath    string   `json:"target_path"`             // 目标路径(部署子路径)
	AliasName     string   `json:"alias_name,omitempty"`    // 若该路径有别名,填别名名称
	ResourceIDs   []string `json:"resource_ids"`            // 重新部署用(仅当前资源)
	ResourceNames []string `json:"resource_names"`          // 子行展示(仅当前资源)
	HasConflict   bool     `json:"has_conflict"`            // 目标已存在同 key / 分隔符块(需覆盖)
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
	// ConflictFor: 仅 existing 冲突填充——本次待部署的哪个资源(名)与已有内容/已部署资源撞车。
	// 用于按"具体哪个 config/prompt"归属展示;旧消费方可忽略该字段。
	ConflictFor string `json:"conflict_for,omitempty"`
}

// CheckConflictsResp 预检冲突响应
type CheckConflictsResp struct {
	HasConflict bool           `json:"has_conflict"`
	Conflicts   []ConflictItem `json:"conflicts"`
}
