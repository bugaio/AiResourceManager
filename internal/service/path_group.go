// Package service path_group.go 实现 PathGroup 模块的业务逻辑
//
// 校验口径与 alias 一致（见 util/path_format.go）：
//   - skill_path/agent_path: 目录（不带文件后缀）
//   - config_path: 后缀 .json/.jsonc/.yaml/.yml/.toml
//   - prompt_path: 后缀 .md
//   - 4 个子路径至少 1 个非空
package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/anthropic/airesourcemanager/internal/model"
	"github.com/anthropic/airesourcemanager/internal/repo"
	"github.com/anthropic/airesourcemanager/internal/util"
)

// PathGroupService 路径组业务服务
type PathGroupService struct {
	repo *repo.PathGroupRepo
	hub  *Hub
}

// NewPathGroupService 创建 PathGroupService
func NewPathGroupService(repo *repo.PathGroupRepo, hub *Hub) *PathGroupService {
	return &PathGroupService{repo: repo, hub: hub}
}

func (s *PathGroupService) broadcast(eventType string, data map[string]interface{}) {
	if s.hub == nil {
		return
	}
	msg, _ := json.Marshal(map[string]interface{}{"type": eventType, "data": data})
	s.hub.Broadcast(msg)
}

// validateSpec 校验子路径四元组（统一供 Create/Update 用）
func validateSpec(skill, agent, config, prompt string) error {
	if strings.TrimSpace(skill) == "" && strings.TrimSpace(agent) == "" &&
		strings.TrimSpace(config) == "" && strings.TrimSpace(prompt) == "" {
		return model.NewBizError(model.ErrPathGroupEmpty, "4 个子路径不能全为空")
	}
	if msg := util.ValidatePathByType("skill", skill); msg != "" {
		return model.NewBizError(model.ErrPathGroupBadFormat, "skill_path: "+msg)
	}
	if msg := util.ValidatePathByType("agent", agent); msg != "" {
		return model.NewBizError(model.ErrPathGroupBadFormat, "agent_path: "+msg)
	}
	if msg := util.ValidatePathByType("config", config); msg != "" {
		return model.NewBizError(model.ErrPathGroupBadFormat, "config_path: "+msg)
	}
	if msg := util.ValidatePathByType("prompt", prompt); msg != "" {
		return model.NewBizError(model.ErrPathGroupBadFormat, "prompt_path: "+msg)
	}
	return nil
}

// expandAndClean 路径展开 ~ 并 Clean（与 alias 行为一致）
func expandAndClean(p string) string {
	if p == "" {
		return ""
	}
	if strings.HasPrefix(p, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			p = home + p[1:]
		}
	}
	return filepath.Clean(p)
}

// CreatePathGroup 创建路径组
func (s *PathGroupService) CreatePathGroup(req *model.CreatePathGroupReq) (*model.PathGroup, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, model.NewBizError(model.ErrPathGroupBadFormat, "name 不能为空")
	}
	if err := validateSpec(req.SkillPath, req.AgentPath, req.ConfigPath, req.PromptPath); err != nil {
		return nil, err
	}
	exists, err := s.repo.CheckPathGroupNameExists(req.Name, "")
	if err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if exists {
		return nil, model.NewBizError(model.ErrPathGroupDuplicateName, "路径组名称已存在")
	}
	now := timeNow()
	g := &model.PathGroup{
		ID:         util.NewUUID(),
		Name:       req.Name,
		SkillPath:  expandAndClean(req.SkillPath),
		AgentPath:  expandAndClean(req.AgentPath),
		ConfigPath: expandAndClean(req.ConfigPath),
		PromptPath: expandAndClean(req.PromptPath),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.repo.InsertPathGroup(g); err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}
	s.broadcast("path_group:created", map[string]interface{}{"id": g.ID})
	return g, nil
}

// GetPathGroup 查询详情
func (s *PathGroupService) GetPathGroup(id string) (*model.PathGroup, error) {
	g, err := s.repo.GetPathGroupByID(id)
	if err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if g == nil {
		return nil, model.NewBizError(model.ErrPathGroupNotFound, "路径组不存在")
	}
	return g, nil
}

// ListPathGroups 列表
func (s *PathGroupService) ListPathGroups() ([]model.PathGroup, error) {
	return s.repo.ListPathGroups()
}

// UpdatePathGroup 更新路径组（任一字段 nil 表示不更新）
func (s *PathGroupService) UpdatePathGroup(id string, req *model.UpdatePathGroupReq) (*model.PathGroup, error) {
	g, err := s.repo.GetPathGroupByID(id)
	if err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if g == nil {
		return nil, model.NewBizError(model.ErrPathGroupNotFound, "路径组不存在")
	}

	// 计算应用更新后的最终值用于校验
	name := g.Name
	skill := g.SkillPath
	agent := g.AgentPath
	config := g.ConfigPath
	prompt := g.PromptPath
	if req.Name != nil {
		name = *req.Name
	}
	if req.SkillPath != nil {
		skill = *req.SkillPath
	}
	if req.AgentPath != nil {
		agent = *req.AgentPath
	}
	if req.ConfigPath != nil {
		config = *req.ConfigPath
	}
	if req.PromptPath != nil {
		prompt = *req.PromptPath
	}

	if strings.TrimSpace(name) == "" {
		return nil, model.NewBizError(model.ErrPathGroupBadFormat, "name 不能为空")
	}
	if err := validateSpec(skill, agent, config, prompt); err != nil {
		return nil, err
	}
	if name != g.Name {
		exists, err := s.repo.CheckPathGroupNameExists(name, id)
		if err != nil {
			return nil, model.NewBizError(model.ErrSystemDB, err.Error())
		}
		if exists {
			return nil, model.NewBizError(model.ErrPathGroupDuplicateName, "路径组名称已存在")
		}
	}

	// 展开路径后再写入
	if req.SkillPath != nil {
		v := expandAndClean(*req.SkillPath)
		req.SkillPath = &v
	}
	if req.AgentPath != nil {
		v := expandAndClean(*req.AgentPath)
		req.AgentPath = &v
	}
	if req.ConfigPath != nil {
		v := expandAndClean(*req.ConfigPath)
		req.ConfigPath = &v
	}
	if req.PromptPath != nil {
		v := expandAndClean(*req.PromptPath)
		req.PromptPath = &v
	}

	if err := s.repo.UpdatePathGroup(id, req); err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}
	updated, _ := s.repo.GetPathGroupByID(id)
	s.broadcast("path_group:updated", map[string]interface{}{"id": id})
	return updated, nil
}

// DeletePathGroup 删除路径组
func (s *PathGroupService) DeletePathGroup(id string) error {
	g, err := s.repo.GetPathGroupByID(id)
	if err != nil {
		return model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if g == nil {
		return model.NewBizError(model.ErrPathGroupNotFound, "路径组不存在")
	}
	if err := s.repo.DeletePathGroup(id); err != nil {
		return model.NewBizError(model.ErrSystemDB, err.Error())
	}
	s.broadcast("path_group:deleted", map[string]interface{}{"id": id})
	return nil
}
