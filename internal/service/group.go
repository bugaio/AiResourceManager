// Package service group.go 实现分组的业务逻辑
// 包括分组 CRUD、资源关联、追踪部署联动等
package service

import (
	"encoding/json"
	"math/rand"
	"strings"

	"github.com/anthropic/airesourcemanager/internal/model"
	"github.com/anthropic/airesourcemanager/internal/repo"
	"github.com/anthropic/airesourcemanager/internal/util"
	"go.uber.org/zap"
)

// 预定义的分组颜色池（柔和且可区分的颜色）
var groupColors = []string{
	"#3B82F6", "#10B981", "#F59E0B", "#EF4444", "#8B5CF6",
	"#EC4899", "#06B6D4", "#84CC16", "#F97316", "#6366F1",
	"#14B8A6", "#D946EF", "#0EA5E9", "#22C55E", "#E11D48",
	"#7C3AED", "#2DD4BF", "#FBBF24", "#FB7185", "#A78BFA",
}

// GroupService 分组业务服务
type GroupService struct {
	repo      *repo.GroupRepo
	hub       *Hub
	logger    *zap.Logger
	deploySvc *DeployService
}

// NewGroupService 创建分组服务实例
// 参数 repo: 分组数据仓库
// 参数 hub: WebSocket Hub（用于广播事件）
// 参数 logger: 日志实例
// 返回: GroupService 指针
func NewGroupService(repo *repo.GroupRepo, hub *Hub, logger *zap.Logger) *GroupService {
	return &GroupService{repo: repo, hub: hub, logger: logger}
}

// SetDeployService 注入部署服务（解决循环依赖）
// 参数 svc: 部署服务实例
func (s *GroupService) SetDeployService(svc *DeployService) {
	s.deploySvc = svc
}

// CreateGroup 创建分组
// 参数 req: 创建请求
// 返回: 创建的分组实体、错误信息
// 逻辑: 校验类型 → 检查名称唯一 → 生成UUID → 写入数据库
func (s *GroupService) CreateGroup(req *model.CreateGroupReq) (*model.Group, error) {
	// 校验类型
	if !isValidType(req.Type) {
		return nil, model.NewBizError(model.ErrGroupInvalid, "type 必须为 skill/agent/config")
	}
	// 校验名称
	if strings.TrimSpace(req.Name) == "" {
		return nil, model.NewBizError(model.ErrGroupInvalid, "name 不能为空")
	}

	// 检查同类型下名称重复
	exists, err := s.repo.CheckGroupNameExists(req.Name, req.Type, "")
	if err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if exists {
		return nil, model.NewBizError(model.ErrGroupExists, "同类型下分组名称已存在")
	}

	// 构造分组实体
	now := timeNow()
	group := &model.Group{
		ID:        util.NewUUID(),
		Name:      req.Name,
		Type:      req.Type,
		Color:     s.pickColor(req.Type),
		SortOrder: 0,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.InsertGroup(group); err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}

	return group, nil
}

// ListGroups 分页查询分组列表
// 参数 groupType: 类型筛选
// 参数 page: 页码
// 参数 pageSize: 每页数量
// 返回: 分页响应、错误信息
func (s *GroupService) ListGroups(groupType string, page, pageSize int) (*model.GroupListResp, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	list, total, err := s.repo.ListGroups(groupType, page, pageSize)
	if err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}

	// 填充每个分组的资源数量
	for i := range list {
		ids, _ := s.repo.GetGroupResources(list[i].ID)
		list[i].ResourceCount = len(ids)
	}

	return &model.GroupListResp{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// UpdateGroup 更新分组
// 参数 id: 分组 ID
// 参数 req: 更新请求
// 返回: 错误信息
func (s *GroupService) UpdateGroup(id string, req *model.UpdateGroupReq) error {
	// 检查分组是否存在
	g, err := s.repo.GetGroupByID(id)
	if err != nil {
		return model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if g == nil {
		return model.NewBizError(model.ErrGroupNotFound, "分组不存在")
	}

	// 名称重复检查
	if req.Name != nil && *req.Name != g.Name {
		if strings.TrimSpace(*req.Name) == "" {
			return model.NewBizError(model.ErrGroupInvalid, "name 不能为空")
		}
		exists, err := s.repo.CheckGroupNameExists(*req.Name, g.Type, id)
		if err != nil {
			return model.NewBizError(model.ErrSystemDB, err.Error())
		}
		if exists {
			return model.NewBizError(model.ErrGroupExists, "同类型下分组名称已存在")
		}
	}

	if err := s.repo.UpdateGroup(id, req.Name, req.SortOrder); err != nil {
		return model.NewBizError(model.ErrSystemDB, err.Error())
	}

	return nil
}

// DeleteGroup 删除分组
// 参数 id: 分组 ID
// 返回: 错误信息
// 逻辑: 检查存在 → 处理追踪部署 → 删除分组及关联
func (s *GroupService) DeleteGroup(id string) error {
	g, err := s.repo.GetGroupByID(id)
	if err != nil {
		return model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if g == nil {
		return model.NewBizError(model.ErrGroupNotFound, "分组不存在")
	}

	// 检查追踪部署
	deployments, err := s.repo.GetTrackDeployments(id)
	if err != nil {
		return model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if len(deployments) > 0 {
		s.logger.Info("删除分组时发现追踪部署，将级联清理",
			zap.String("group_id", id),
			zap.Int("deployment_count", len(deployments)),
		)
		if s.deploySvc != nil {
			for _, dep := range deployments {
				s.deploySvc.Undeploy(dep.ID)
			}
		}
	}

	if err := s.repo.DeleteGroup(id); err != nil {
		return model.NewBizError(model.ErrSystemDB, err.Error())
	}

	return nil
}

// AddResources 添加资源到分组
// 参数 groupID: 分组 ID
// 参数 resourceIDs: 资源 ID 列表
// 返回: 错误信息
// 逻辑: 校验分组存在 → 插入关联 → 检查追踪部署 → 广播事件
func (s *GroupService) AddResources(groupID string, resourceIDs []string) error {
	// 检查分组是否存在
	g, err := s.repo.GetGroupByID(groupID)
	if err != nil {
		return model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if g == nil {
		return model.NewBizError(model.ErrGroupNotFound, "分组不存在")
	}

	// 批量插入关联
	if err := s.repo.AddResourcesToGroup(groupID, resourceIDs); err != nil {
		return model.NewBizError(model.ErrSystemDB, err.Error())
	}

	// 检查追踪部署，自动部署新资源
	deployments, err := s.repo.GetTrackDeployments(groupID)
	if err != nil {
		s.logger.Error("查询追踪部署失败", zap.Error(err))
	}
	if len(deployments) > 0 && s.deploySvc != nil {
		s.logger.Info("分组添加资源触发追踪部署同步",
			zap.String("group_id", groupID),
			zap.Int("new_resources", len(resourceIDs)),
			zap.Int("track_deployments", len(deployments)),
		)
		for _, dep := range deployments {
			for _, rid := range resourceIDs {
				r, rErr := s.deploySvc.resourceRepo.GetResourceByID(rid)
				if rErr != nil || r == nil {
					continue
				}
				if dErr := s.deploySvc.DeploySingleResourceToTarget(dep.ID, r, dep.TargetPath); dErr != nil {
					s.logger.Error("追踪部署资源失败",
						zap.String("deployment_id", dep.ID),
						zap.String("resource_id", rid),
						zap.Error(dErr),
					)
				}
			}
		}
	}

	// 广播 deploy:synced 事件
	s.broadcastSynced(groupID)

	return nil
}

// RemoveResource 从分组中移除资源
// 参数 groupID: 分组 ID
// 参数 resourceID: 资源 ID
// 返回: 错误信息
// 逻辑: 检查追踪部署 → 删除关联 → 广播事件
func (s *GroupService) RemoveResource(groupID, resourceID string) error {
	// 检查分组是否存在
	g, err := s.repo.GetGroupByID(groupID)
	if err != nil {
		return model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if g == nil {
		return model.NewBizError(model.ErrGroupNotFound, "分组不存在")
	}

	// 检查追踪部署，自动取消部署（仅 track=1 的才撤销实际文件）
	deployments, err := s.repo.GetTrackDeployments(groupID)
	if err != nil {
		s.logger.Error("查询追踪部署失败", zap.Error(err))
	}
	if len(deployments) > 0 && s.deploySvc != nil {
		s.logger.Info("分组移除资源触发追踪部署同步",
			zap.String("group_id", groupID),
			zap.String("resource_id", resourceID),
			zap.Int("track_deployments", len(deployments)),
		)
		for _, dep := range deployments {
			if uErr := s.deploySvc.UndeployResourceFromTarget(resourceID, dep.ID, dep.TargetPath, ""); uErr != nil {
				s.logger.Error("追踪取消部署失败",
					zap.String("deployment_id", dep.ID),
					zap.String("resource_id", resourceID),
					zap.Error(uErr),
				)
			}
		}
	}

	// 删除关联
	if err := s.repo.RemoveResourceFromGroup(groupID, resourceID); err != nil {
		return model.NewBizError(model.ErrSystemDB, err.Error())
	}

	// 广播 deploy:synced 事件
	s.broadcastSynced(groupID)

	return nil
}

// broadcastSynced 广播 deploy:synced 事件
// 参数 groupID: 触发事件的分组 ID
func (s *GroupService) broadcastSynced(groupID string) {
	msg, _ := json.Marshal(map[string]interface{}{
		"type":     "deploy:synced",
		"group_id": groupID,
	})
	s.hub.Broadcast(msg)
}

// pickColor 从颜色池中选取一个未被同类型分组使用的颜色
// 如果所有颜色都已使用，则随机选一个
func (s *GroupService) pickColor(groupType string) string {
	// 获取同类型已有分组的颜色
	groups, _, _ := s.repo.ListGroups(groupType, 1, 1000)
	usedColors := make(map[string]bool)
	for _, g := range groups {
		if g.Color != "" {
			usedColors[g.Color] = true
		}
	}

	// 找未使用的颜色
	var available []string
	for _, c := range groupColors {
		if !usedColors[c] {
			available = append(available, c)
		}
	}

	if len(available) > 0 {
		return available[rand.Intn(len(available))]
	}
	// 全部用完，随机返回一个
	return groupColors[rand.Intn(len(groupColors))]
}
