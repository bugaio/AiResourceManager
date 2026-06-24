// Package service preset.go 实现 Preset 模块的业务逻辑
//
// 包括: preset CRUD、关联/取消关联资源、私有资源增删、Preset 部署
// 与 ResourceService / DeployService 协作完成级联与部署
package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropic/airesourcemanager/internal/model"
	"github.com/anthropic/airesourcemanager/internal/repo"
	"github.com/anthropic/airesourcemanager/internal/util"
	"go.uber.org/zap"
)

// PresetService Preset 业务服务
type PresetService struct {
	repo        *repo.PresetRepo
	resourceRepo *repo.ResourceRepo
	resourceSvc *ResourceService
	deploySvc   *DeployService
	hub         *Hub
	logger      *zap.Logger
}

// NewPresetService 创建 PresetService
func NewPresetService(repo *repo.PresetRepo, resourceRepo *repo.ResourceRepo, resourceSvc *ResourceService, deploySvc *DeployService, hub *Hub, logger *zap.Logger) *PresetService {
	return &PresetService{
		repo:         repo,
		resourceRepo: resourceRepo,
		resourceSvc:  resourceSvc,
		deploySvc:    deploySvc,
		hub:          hub,
		logger:       logger,
	}
}

// broadcast 广播 preset 相关事件
func (s *PresetService) broadcast(eventType string, data map[string]interface{}) {
	if s.hub == nil {
		return
	}
	msg, _ := json.Marshal(map[string]interface{}{"type": eventType, "data": data})
	s.hub.Broadcast(msg)
}

// CreatePreset 创建 preset
func (s *PresetService) CreatePreset(req *model.CreatePresetReq) (*model.Preset, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, model.NewBizError(model.ErrPresetInvalid, "name 不能为空")
	}
	exists, err := s.repo.CheckPresetNameExists(req.Name, "")
	if err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if exists {
		return nil, model.NewBizError(model.ErrPresetDuplicateName, "preset 名称已存在")
	}
	now := timeNow()
	p := &model.Preset{
		ID:          util.NewUUID(),
		Name:        req.Name,
		Description: req.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.InsertPreset(p); err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}
	s.broadcast("preset:created", map[string]interface{}{"id": p.ID})
	return p, nil
}

// UpdatePreset 更新 preset
func (s *PresetService) UpdatePreset(id string, req *model.UpdatePresetReq) (*model.Preset, error) {
	p, err := s.repo.GetPresetByID(id)
	if err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if p == nil {
		return nil, model.NewBizError(model.ErrPresetNotFound, "preset 不存在")
	}
	if req.Name != nil && *req.Name != p.Name {
		if strings.TrimSpace(*req.Name) == "" {
			return nil, model.NewBizError(model.ErrPresetInvalid, "name 不能为空")
		}
		exists, err := s.repo.CheckPresetNameExists(*req.Name, id)
		if err != nil {
			return nil, model.NewBizError(model.ErrSystemDB, err.Error())
		}
		if exists {
			return nil, model.NewBizError(model.ErrPresetDuplicateName, "preset 名称已存在")
		}
	}
	if err := s.repo.UpdatePreset(id, req.Name, req.Description); err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}
	updated, _ := s.repo.GetPresetByID(id)
	s.broadcast("preset:updated", map[string]interface{}{"id": id})
	return updated, nil
}

// DeletePreset 删除 preset（自动撤销部署 + 物理删除私有资源 + 解除关联 + 删本体）
func (s *PresetService) DeletePreset(id string) error {
	p, err := s.repo.GetPresetByID(id)
	if err != nil {
		return model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if p == nil {
		return model.NewBizError(model.ErrPresetNotFound, "preset 不存在")
	}

	// 1. 撤销所有部署
	if s.deploySvc != nil {
		if err := s.deploySvc.UndeployAllPreset(id); err != nil {
			s.logger.Warn("撤销 preset 部署失败", zap.String("id", id), zap.Error(err))
		}
	}

	// 2. 物理删除私有资源（含文件）
	privateIDs, _ := s.repo.ListPrivateResourceIDs(id)
	if s.resourceSvc != nil {
		for _, rid := range privateIDs {
			if err := s.resourceSvc.DeletePrivateResource(rid); err != nil {
				s.logger.Warn("删除私有资源失败", zap.String("rid", rid), zap.Error(err))
			}
		}
	}

	// 3. 解除所有 preset_resource 关联（外键 CASCADE 也会自动清理,这里显式删确保稳定）
	linkedIDs, _ := s.repo.ListPresetResources(id)
	if len(linkedIDs) > 0 {
		_ = s.repo.UnlinkResources(id, linkedIDs)
	}

	// 4. 删 preset 本体
	if err := s.repo.DeletePreset(id); err != nil {
		return model.NewBizError(model.ErrSystemDB, err.Error())
	}
	s.broadcast("preset:deleted", map[string]interface{}{"id": id})
	return nil
}

// DeletePrivateResource 删除 preset 下的单个私有资源
// 校验: 资源必须存在且 owner_preset_id == presetID,否则拒绝
func (s *PresetService) DeletePrivateResource(presetID, resourceID string) error {
	r, err := s.resourceRepo.GetResourceByID(resourceID)
	if err != nil {
		return model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if r == nil {
		return model.NewBizError(model.ErrResourceNotFound, "资源不存在")
	}
	if r.OwnerPresetID == nil || *r.OwnerPresetID != presetID {
		return model.NewBizError(model.ErrResourceLockedByPreset,
			"该资源不是此 preset 的私有资源,无权删除")
	}

	if s.resourceSvc != nil {
		if err := s.resourceSvc.DeletePrivateResource(resourceID); err != nil {
			return err
		}
	}
	s.broadcast("preset:updated", map[string]interface{}{"id": presetID})
	return nil
}

// ListPresets 列出所有 preset 并填充资源统计与部署记录
func (s *PresetService) ListPresets() ([]model.Preset, error) {
	presets, err := s.repo.ListPresets()
	if err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}
	for i := range presets {
		s.fillCounts(&presets[i])
		s.fillDeployments(&presets[i])
	}
	return presets, nil
}

// GetPreset 详情（含资源统计）
func (s *PresetService) GetPreset(id string) (*model.Preset, error) {
	p, err := s.repo.GetPresetByID(id)
	if err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if p == nil {
		return nil, model.NewBizError(model.ErrPresetNotFound, "preset 不存在")
	}
	s.fillCounts(p)
	return p, nil
}

// fillCounts 填充 ResourceCount / PrivateCount / LinkedCount
func (s *PresetService) fillCounts(p *model.Preset) {
	linked, _ := s.repo.ListPresetResources(p.ID)
	priv, _ := s.repo.ListPrivateResourceIDs(p.ID)
	p.LinkedCount = len(linked)
	p.PrivateCount = len(priv)
	p.ResourceCount = p.LinkedCount + p.PrivateCount
}

// fillDeployments 填充 Preset 的 Deployments 字段
func (s *PresetService) fillDeployments(p *model.Preset) {
	if s.deploySvc == nil {
		return
	}
	deps, err := s.deploySvc.ListPresetDeployments(p.ID)
	if err != nil {
		s.logger.Warn("填充 preset 部署记录失败", zap.String("id", p.ID), zap.Error(err))
		return
	}
	if deps == nil {
		deps = []model.Deployment{}
	}
	p.Deployments = deps
	// 填充按路径组聚合的漂移信息（侧栏「未同步」标识用）
	if drifts, dErr := s.deploySvc.ListPresetGroupDrifts(p.ID); dErr == nil {
		p.GroupDrifts = drifts
	}
}

// ListPresetDeployments 返回某 preset 的所有部署记录
func (s *PresetService) ListPresetDeployments(presetID string) ([]model.Deployment, error) {
	if s.deploySvc == nil {
		return []model.Deployment{}, nil
	}
	return s.deploySvc.ListPresetDeployments(presetID)
}

// GetPresetGroupStatus preset 在某路径组下的完整部署状态（部署管理弹窗用）
func (s *PresetService) GetPresetGroupStatus(presetID, groupID string) (*model.PresetGroupStatus, error) {
	if s.deploySvc == nil {
		return &model.PresetGroupStatus{}, nil
	}
	return s.deploySvc.GetPresetGroupStatus(presetID, groupID)
}

// RedeployPresetGroup 将 preset 以最新全量资源重新部署到指定路径组
func (s *PresetService) RedeployPresetGroup(presetID, groupID string) ([]model.Deployment, error) {
	if s.deploySvc == nil {
		return nil, model.NewBizError(model.ErrSystemInternal, "deploySvc 未注入")
	}
	return s.deploySvc.RedeployPresetGroup(presetID, groupID)
}

// LinkResources 关联资源到 preset
// 校验: 每个资源必须是全局资源（owner_preset_id IS NULL），否则报 1106
func (s *PresetService) LinkResources(presetID string, resourceIDs []string) error {
	p, err := s.repo.GetPresetByID(presetID)
	if err != nil {
		return model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if p == nil {
		return model.NewBizError(model.ErrPresetNotFound, "preset 不存在")
	}
	for _, rid := range resourceIDs {
		// 需查 resource owner_preset_id；私有资源不可被其他 preset 引用
		r, err := s.resourceRepo.GetResourceByID(rid)
		if err != nil {
			return model.NewBizError(model.ErrSystemDB, err.Error())
		}
		if r == nil {
			return model.NewBizError(model.ErrResourceNotFound, "资源不存在: "+rid)
		}
		if r.OwnerPresetID != nil && *r.OwnerPresetID != "" {
			return model.NewBizError(model.ErrPrivateResourceCross,
				fmt.Sprintf("资源 %s 是其他 preset 的私有资源,不能被引用", r.Name))
		}
	}
	// config 冲突防线：待关联的 config 不得与 preset 已有 config 路径树冲突
	if conf, cErr := s.CheckPresetConfigConflicts(presetID, resourceIDs); cErr == nil && conf.HasConflict {
		names := make([]string, 0, len(conf.Conflicts))
		for _, c := range conf.Conflicts {
			names = append(names, c.ResourceName)
		}
		return model.NewBizErrorWithData(
			model.ErrDeployFailed,
			"存在与已有 config 冲突的资源,请移除后重试",
			map[string]interface{}{"config_conflicts": conf.Conflicts, "names": names},
		)
	}
	if err := s.repo.LinkResources(presetID, resourceIDs); err != nil {
		return model.NewBizError(model.ErrSystemDB, err.Error())
	}
	// 同步追踪部署（新增资源需要部署到现有 track=1 的 deployment）
	if s.deploySvc != nil {
		s.deploySvc.SyncPresetDeployments(presetID)
	}
	s.broadcast("preset:resource_changed", map[string]interface{}{"preset_id": presetID})
	return nil
}

// UnlinkResources 解除关联并重新部署受影响的 deployment
func (s *PresetService) UnlinkResources(presetID string, resourceIDs []string) error {
	p, err := s.repo.GetPresetByID(presetID)
	if err != nil {
		return model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if p == nil {
		return model.NewBizError(model.ErrPresetNotFound, "preset 不存在")
	}
	if err := s.repo.UnlinkResources(presetID, resourceIDs); err != nil {
		return model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if s.deploySvc != nil {
		s.deploySvc.SyncPresetDeployments(presetID)
	}
	s.broadcast("preset:resource_changed", map[string]interface{}{"preset_id": presetID})
	return nil
}

// CreatePrivateResource 在 preset 下创建私有资源
func (s *PresetService) CreatePrivateResource(presetID string, req *model.CreateResourceReq) (*model.Resource, error) {
	p, err := s.repo.GetPresetByID(presetID)
	if err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if p == nil {
		return nil, model.NewBizError(model.ErrPresetNotFound, "preset 不存在")
	}
	resource, err := s.resourceSvc.CreatePrivateResource(presetID, req)
	if err != nil {
		return nil, err
	}
	s.broadcast("preset:resource_changed", map[string]interface{}{"preset_id": presetID})
	return resource, nil
}

// presetConfigResourceIDs 返回 preset 当前所有 config 资源 ID(关联 + 私有)
func (s *PresetService) presetConfigResourceIDs(presetID string) ([]string, error) {
	linked, err := s.repo.ListPresetResources(presetID)
	if err != nil {
		return nil, err
	}
	private, err := s.repo.ListPrivateResourceIDs(presetID)
	if err != nil {
		return nil, err
	}
	all := append(append([]string{}, linked...), private...)
	var configIDs []string
	for _, rid := range all {
		r, gErr := s.resourceRepo.GetResourceByID(rid)
		if gErr != nil || r == nil {
			continue
		}
		if r.Type == "config" {
			configIDs = append(configIDs, r.ID)
		}
	}
	return configIDs, nil
}

// CheckPresetConfigConflicts 检测候选 config 与 preset 中【已有的其他】config 是否冲突。
//
// 统一服务两个场景:
//   - 关联场景: candidateIDs = 待关联的全局 config 资源 ID
//   - 编辑场景: candidateIDs = 正在保存的私有 config 自身 ID(已落库,读其当前文件内容)
//
// "已有" = preset 当前所有 config 资源(关联+私有) 减去候选自身。
// 双层循环逐个候选 × 已有比对,使用路径树语义 configsConflict。
func (s *PresetService) CheckPresetConfigConflicts(presetID string, candidateIDs []string) (*model.CheckPresetConfigConflictsResp, error) {
	if s.deploySvc == nil {
		return &model.CheckPresetConfigConflictsResp{}, nil
	}
	p, err := s.repo.GetPresetByID(presetID)
	if err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if p == nil {
		return nil, model.NewBizError(model.ErrPresetNotFound, "preset 不存在")
	}

	// preset 当前已有 config 资源 ID
	existingIDs, err := s.presetConfigResourceIDs(presetID)
	if err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}
	candidateSet := map[string]bool{}
	for _, id := range candidateIDs {
		candidateSet[id] = true
	}

	resp := &model.CheckPresetConfigConflictsResp{Conflicts: []model.PresetConfigConflict{}}
	for _, cid := range candidateIDs {
		cr, gErr := s.resourceRepo.GetResourceByID(cid)
		if gErr != nil || cr == nil || cr.Type != "config" {
			continue // 非 config 候选跳过
		}
		candidateCfg := s.deploySvc.readConfigFragmentByID(cid)
		if len(candidateCfg) == 0 {
			continue // 空内容不可能冲突
		}
		var hits []model.PresetConfigConflictItem
		for _, eid := range existingIDs {
			if eid == cid || candidateSet[eid] {
				continue // 跳过自身、跳过同批候选(候选间不在此处比，避免重复阻断)
			}
			existingCfg := s.deploySvc.readConfigFragmentByID(eid)
			if configsConflict(candidateCfg, existingCfg) {
				er, _ := s.resourceRepo.GetResourceByID(eid)
				name := eid
				if er != nil {
					name = er.Name
				}
				hits = append(hits, model.PresetConfigConflictItem{ResourceID: eid, ResourceName: name})
			}
		}
		if len(hits) > 0 {
			resp.HasConflict = true
			resp.Conflicts = append(resp.Conflicts, model.PresetConfigConflict{
				ResourceID:    cid,
				ResourceName:  cr.Name,
				ConflictsWith: hits,
			})
		}
	}
	return resp, nil
}

// ImportPrivateSkill 导入私有 skill
func (s *PresetService) ImportPrivateSkill(presetID string, name, description string, files []ImportedSkillFile) (*model.Resource, error) {
	p, err := s.repo.GetPresetByID(presetID)
	if err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if p == nil {
		return nil, model.NewBizError(model.ErrPresetNotFound, "preset 不存在")
	}
	resource, err := s.resourceSvc.ImportPrivateSkill(presetID, name, description, files)
	if err != nil {
		return nil, err
	}
	s.broadcast("preset:resource_changed", map[string]interface{}{"preset_id": presetID})
	return resource, nil
}

// ImportPrivateAgent 导入私有 agent
func (s *PresetService) ImportPrivateAgent(presetID string, name, description string, data []byte) (*model.Resource, error) {
	p, err := s.repo.GetPresetByID(presetID)
	if err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if p == nil {
		return nil, model.NewBizError(model.ErrPresetNotFound, "preset 不存在")
	}
	resource, err := s.resourceSvc.ImportPrivateAgent(presetID, name, description, data)
	if err != nil {
		return nil, err
	}
	s.broadcast("preset:resource_changed", map[string]interface{}{"preset_id": presetID})
	return resource, nil
}

// ListPresetResources 返回 preset 下所有资源（关联 + 私有）
func (s *PresetService) ListPresetResources(presetID string) ([]model.Resource, error) {
	p, err := s.repo.GetPresetByID(presetID)
	if err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if p == nil {
		return nil, model.NewBizError(model.ErrPresetNotFound, "preset 不存在")
	}
	linked, err := s.repo.ListPresetResources(presetID)
	if err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}
	private, err := s.repo.ListPrivateResourceIDs(presetID)
	if err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}
	all := append(append([]string{}, linked...), private...)
	list := make([]model.Resource, 0, len(all))
	for _, rid := range all {
		r, err := s.resourceRepo.GetResourceByID(rid)
		if err != nil || r == nil {
			continue
		}
		list = append(list, *r)
	}
	return list, nil
}

// DeployPreset 部署 preset
func (s *PresetService) DeployPreset(presetID string, req *model.DeployPresetReq) ([]model.Deployment, error) {
	p, err := s.repo.GetPresetByID(presetID)
	if err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if p == nil {
		return nil, model.NewBizError(model.ErrPresetNotFound, "preset 不存在")
	}
	return s.deploySvc.DeployPreset(req, presetID)
}

// UndeployPresetDeployment 撤销某次 preset 部署
func (s *PresetService) UndeployPresetDeployment(presetID, deploymentID string) error {
	return s.deploySvc.UndeployPresetDeployment(presetID, deploymentID)
}

// RedeployPreset 重新部署整个 preset（复用已有 target_path）
func (s *PresetService) RedeployPreset(presetID string) ([]model.Deployment, error) {
	p, err := s.repo.GetPresetByID(presetID)
	if err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if p == nil {
		return nil, model.NewBizError(model.ErrPresetNotFound, "preset 不存在")
	}
	return s.deploySvc.RedeployPreset(presetID)
}
