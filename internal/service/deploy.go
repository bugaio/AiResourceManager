// Package service deploy.go 实现部署管理的业务逻辑
// 包括执行部署、撤销部署、健康检查、修复、清理等操作
package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/anthropic/airesourcemanager/internal/model"
	"github.com/anthropic/airesourcemanager/internal/repo"
	"github.com/anthropic/airesourcemanager/internal/util"
	yamlv3 "go.yaml.in/yaml/v3"

	"github.com/pelletier/go-toml/v2"
)

// DeployService 部署管理业务服务
type DeployService struct {
	deployRepo   *repo.DeployRepo
	resourceRepo *repo.ResourceRepo
	aliasRepo    *repo.AliasRepo
	groupRepo    *repo.GroupRepo
	presetRepo   *repo.PresetRepo
	pathGroupRepo *repo.PathGroupRepo
	hub          *Hub
	baseDir      string // ~/.aiManager
}

// NewDeployService 创建部署服务实例
// 参数 deployRepo: 部署数据仓库
// 参数 resourceRepo: 资源数据仓库
// 参数 aliasRepo: 别名数据仓库
// 参数 groupRepo: 分组数据仓库
// 参数 baseDir: 资源文件根目录
// 返回: DeployService 指针
func NewDeployService(deployRepo *repo.DeployRepo, resourceRepo *repo.ResourceRepo, aliasRepo *repo.AliasRepo, groupRepo *repo.GroupRepo, baseDir string) *DeployService {
	return &DeployService{
		deployRepo:   deployRepo,
		resourceRepo: resourceRepo,
		aliasRepo:    aliasRepo,
		groupRepo:    groupRepo,
		baseDir:      baseDir,
	}
}

// SetPresetRepo 注入 preset 数据仓库（用于 SyncPresetDeployments / 重新部署时加载关联）
func (s *DeployService) SetPresetRepo(r *repo.PresetRepo) {
	s.presetRepo = r
}

// SetPathGroupRepo 注入路径组数据仓库（用于把部署 target_path 匹配回所属路径组名）
func (s *DeployService) SetPathGroupRepo(r *repo.PathGroupRepo) {
	s.pathGroupRepo = r
}

// SetHub 注入 WebSocket Hub（用于广播 preset 部署事件）
func (s *DeployService) SetHub(h *Hub) {
	s.hub = h
}

// broadcastEvent 向所有 WS 客户端广播事件
func (s *DeployService) broadcastEvent(eventType string, data map[string]interface{}) {
	if s.hub == nil {
		return
	}
	msg, _ := json.Marshal(map[string]interface{}{
		"type": eventType,
		"data": data,
	})
	s.hub.Broadcast(msg)
}

// deployResultItem 部署结果内部结构
type deployResultItem struct {
	resource   *model.Resource
	linkPath   string
	deployType string
}

// Deploy 执行部署操作
// 参数 req: 部署请求
// 返回: 创建的部署记录、错误信息
func (s *DeployService) Deploy(req *model.DeployRequest) (*model.Deployment, error) {
	// 1. 解析目标路径
	targetPath, err := s.resolveTargetPath(req)
	if err != nil {
		return nil, err
	}

	// 2. 确定要部署的资源
	resources, err := s.resolveResources(req)
	if err != nil {
		return nil, err
	}
	if len(resources) == 0 {
		return nil, model.NewBizError(model.ErrDeployInvalid, "没有找到可部署的资源")
	}

	// 3. 按资源类型处理目标路径
	//    - config: 目标路径必须是一个已存在的配置文件（.json/.yaml/.yml/.toml）
	//    - prompt: 目标路径必须是一个已存在的 .md 文件
	//    - skill/agent: 目标路径是目录，不存在则创建
	switch resources[0].Type {
	case "config":
		if !util.IsConfigFile(targetPath) {
			return nil, model.NewBizError(model.ErrDeployInvalid,
				fmt.Sprintf("Config 目标文件后缀必须是 .json/.yaml/.yml/.toml: %s", targetPath))
		}
		if !util.FileExists(targetPath) {
			return nil, model.NewBizError(model.ErrDeployFailed, fmt.Sprintf("目标文件不存在，请先创建: %s", targetPath))
		}
		if util.IsDir(targetPath) {
			return nil, model.NewBizError(model.ErrDeployInvalid, fmt.Sprintf("Config 目标路径必须是文件而非目录: %s", targetPath))
		}
	case "prompt":
		if !util.IsPromptFile(targetPath) {
			return nil, model.NewBizError(model.ErrDeployInvalid,
				fmt.Sprintf("Prompt 目标文件后缀必须是 .md: %s", targetPath))
		}
		if !util.FileExists(targetPath) {
			return nil, model.NewBizError(model.ErrDeployFailed, fmt.Sprintf("目标文件不存在，请先创建: %s", targetPath))
		}
		if util.IsDir(targetPath) {
			return nil, model.NewBizError(model.ErrDeployInvalid, fmt.Sprintf("Prompt 目标路径必须是文件而非目录: %s", targetPath))
		}
	default:
		if err := util.EnsureDir(targetPath); err != nil {
			return nil, model.NewBizError(model.ErrDeployFailed, fmt.Sprintf("创建目标目录失败: %s, %v", targetPath, err))
		}
	}

	// 4. 执行部署并收集结果（force 时先清理旧的部署记录）
	var results []deployResultItem

	for _, r := range resources {
		res := r
		var linkPath string
		var deployType string

		// force 模式下，清理该资源在同一目标路径的旧部署记录
		if req.Force {
			s.cleanOldDeploymentItems(res.ID, targetPath)
		}

		switch res.Type {
		case "skill":
			linkPath, err = s.deploySkill(&res, targetPath, req.Force)
			deployType = "symlink"
		case "agent":
			linkPath, err = s.deployAgent(&res, targetPath, req.Force)
			deployType = "symlink"
		case "config":
			linkPath, err = s.deployConfig(&res, targetPath, req.Force)
			deployType = "merge"
		case "prompt":
			linkPath, err = s.deployPrompt(&res, targetPath, req.Force)
			deployType = "merge"
		default:
			err = model.NewBizError(model.ErrDeployInvalid, fmt.Sprintf("不支持的资源类型: %s", res.Type))
		}

		if err != nil {
			return nil, err
		}
		results = append(results, deployResultItem{resource: &res, linkPath: linkPath, deployType: deployType})
	}

	// 5. 确定 deploy_type
	finalDeployType := "symlink"
	hasMerge := false
	hasSymlink := false
	for _, r := range results {
		if r.deployType == "merge" {
			hasMerge = true
		} else {
			hasSymlink = true
		}
	}
	if hasMerge && !hasSymlink {
		finalDeployType = "merge"
	}

	// 6. 创建部署记录
	track := 0
	if req.Track {
		track = 1
	}
	deployment := &model.Deployment{
		ID:         util.NewUUID(),
		GroupID:    req.GroupID,
		ResourceID: req.ResourceID,
		TargetPath: targetPath,
		AliasID:    req.AliasID,
		DeployType: finalDeployType,
		Track:      track,
		CreatedAt:  time.Now().Format(time.RFC3339),
		PresetID:   req.PresetID,
	}

	if _, err := s.deployRepo.InsertDeployment(deployment); err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}

	// 7. 创建部署明细
	for _, r := range results {
		item := &model.DeploymentItem{
			ID:           util.NewUUID(),
			DeploymentID: deployment.ID,
			ResourceID:   r.resource.ID,
			LinkPath:     r.linkPath,
		}
		if err := s.deployRepo.InsertDeploymentItem(item); err != nil {
			return nil, model.NewBizError(model.ErrSystemDB, err.Error())
		}
	}

	// 8. 更新 _meta.json
	s.writeMetaJSON(targetPath, deployment, results)

	return deployment, nil
}

// ListDeployments 查询部署记录列表（分页）
// 参数 page: 页码
// 参数 pageSize: 每页数量
// 返回: 分页响应、错误信息
func (s *DeployService) ListDeployments(page, pageSize int) (*model.DeployListResp, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	list, total, err := s.deployRepo.ListDeployments(page, pageSize)
	if err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}

	return &model.DeployListResp{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// Undeploy 撤销部署
// 参数 deploymentID: 部署记录 ID
// 返回: 错误信息
func (s *DeployService) Undeploy(deploymentID string) error {
	deployment, err := s.deployRepo.GetDeploymentByID(deploymentID)
	if err != nil {
		return model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if deployment == nil {
		return model.NewBizError(model.ErrDeployNotFound, "部署记录不存在")
	}

	items, err := s.deployRepo.GetDeploymentItems(deploymentID)
	if err != nil {
		return model.NewBizError(model.ErrSystemDB, err.Error())
	}

	// 逐项撤销
	for _, item := range items {
		if deployment.DeployType == "merge" {
			s.removeMergeResource(deployment.TargetPath, item.ResourceID)
		} else {
			if _, e := os.Lstat(item.LinkPath); e == nil {
				os.Remove(item.LinkPath)
			}
		}
	}

	// 更新 _meta.json
	s.removeFromMetaJSON(deployment.TargetPath, deploymentID)

	// 删除数据库记录
	if err := s.deployRepo.DeleteDeployment(deploymentID); err != nil {
		return model.NewBizError(model.ErrSystemDB, err.Error())
	}

	return nil
}

// SummarizeDeploymentsAtPaths 统计给定目标路径下的部署内容（编辑路径组删 config 路径前用）。
// 返回 map[path] -> 该路径下所有部署的资源名汇总；无部署的路径不出现在结果中。
func (s *DeployService) SummarizeDeploymentsAtPaths(paths []string) (map[string][]string, error) {
	byTarget, err := s.deployRepo.GetDeploymentsByTarget()
	if err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}
	result := map[string][]string{}
	for _, p := range paths {
		deps, ok := byTarget[p]
		if !ok || len(deps) == 0 {
			continue
		}
		var names []string
		for _, d := range deps {
			items, _ := s.deployRepo.GetDeploymentItems(d.ID)
			for _, it := range items {
				name := it.ResourceID
				if r, rErr := s.resourceRepo.GetResourceByID(it.ResourceID); rErr == nil && r != nil {
					name = r.Name
				}
				names = append(names, name)
			}
		}
		if len(names) > 0 {
			result[p] = names
		}
	}
	return result, nil
}

// UndeployAtPaths 撤销给定目标路径下的所有部署（编辑路径组删 config 路径并确认移除时用）。
// 返回成功撤销的 deployment 数。
func (s *DeployService) UndeployAtPaths(paths []string) (int, error) {
	byTarget, err := s.deployRepo.GetDeploymentsByTarget()
	if err != nil {
		return 0, model.NewBizError(model.ErrSystemDB, err.Error())
	}
	count := 0
	for _, p := range paths {
		deps, ok := byTarget[p]
		if !ok {
			continue
		}
		for _, d := range deps {
			if err := s.Undeploy(d.ID); err != nil {
				continue // 单条失败不阻断其余
			}
			count++
		}
	}
	return count, nil
}

// GetTargets 获取目标路径聚合信息
// 参数 resourceType: 资源类型过滤（skill/agent/config）；为空则不过滤返回全部
// 返回: 目标路径列表、错误信息
// 说明: deployment 表本身无 type 字段（skill 与 agent 的 deploy_type 同为 symlink，
//
//	无法区分），故按 deployment_item 关联资源的实际 type 过滤。
//	过滤后无 item 的 deployment 跳过，无 deployment 的目标路径不返回。
func (s *DeployService) GetTargets(resourceType string) ([]model.TargetInfo, error) {
	deploymentsByTarget, err := s.deployRepo.GetDeploymentsByTarget()
	if err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}

	var targets []model.TargetInfo
	for targetPath, deployments := range deploymentsByTarget {
		ti := model.TargetInfo{
			TargetPath:  targetPath,
			Deployments: make([]model.DeploymentDetail, 0, len(deployments)),
		}

		for _, d := range deployments {
			// 隔离：preset 部署是独立单元，仅在 Preset 模块（按 preset_id 查询）展示，
			// 不能泄漏到 skill/agent/config/prompt 等资源类型模块的目标路径列表。
			if d.PresetID != nil && *d.PresetID != "" {
				continue
			}

			items, err := s.deployRepo.GetDeploymentItems(d.ID)
			if err != nil {
				continue
			}

			// 仅当 deployment 是从分组部署的，才查分组信息
			var deployGroup *model.Group
			if d.GroupID != nil && *d.GroupID != "" {
				deployGroup, _ = s.groupRepo.GetGroupByID(*d.GroupID)
			}

			filtered := make([]model.DeploymentItem, 0, len(items))
			for i := range items {
				// 按资源 type 过滤（同时填充资源名称）
				r, rErr := s.resourceRepo.GetResourceByID(items[i].ResourceID)
				if rErr == nil && r != nil {
					if resourceType != "" && r.Type != resourceType {
						continue
					}
					items[i].ResourceName = r.Name
				} else if resourceType != "" {
					// 资源已不存在且要求按类型过滤时，无法判定类型，跳过
					continue
				}
				items[i].Status = s.checkItemHealth(&d, &items[i])
				// 只有从分组部署且资源当前仍在该分组中才显示分组名称
				if deployGroup != nil && s.groupRepo.IsResourceInGroup(*d.GroupID, items[i].ResourceID) {
					items[i].GroupName = deployGroup.Name
					items[i].GroupColor = deployGroup.Color
				}
				filtered = append(filtered, items[i])
			}

			// 该 deployment 在当前类型下无任何 item，跳过
			if len(filtered) == 0 {
				continue
			}
			ti.Deployments = append(ti.Deployments, model.DeploymentDetail{
				Deployment: d,
				Items:      filtered,
			})
		}

		// 过滤后该路径下没有任何匹配的部署，不返回该目标
		if len(ti.Deployments) == 0 {
			continue
		}
		targets = append(targets, ti)
	}

	if targets == nil {
		targets = []model.TargetInfo{}
	}

	// 按最新部署时间倒序排列（最新路径在前）
	sort.Slice(targets, func(i, j int) bool {
		ti := latestDeployTime(targets[i])
		tj := latestDeployTime(targets[j])
		return ti > tj
	})

	return targets, nil
}

// latestDeployTime 取该目标路径下最新的部署时间字符串（用于排序）
func latestDeployTime(ti model.TargetInfo) string {
	latest := ""
	for _, d := range ti.Deployments {
		if d.CreatedAt > latest {
			latest = d.CreatedAt
		}
	}
	return latest
}

// CheckHealth 执行健康检查，返回异常部署子项
// 返回: broken 状态的明细列表、错误信息
func (s *DeployService) CheckHealth() ([]model.DeploymentItem, error) {
	allItems, err := s.deployRepo.GetAllDeploymentItems()
	if err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}

	var broken []model.DeploymentItem
	for _, item := range allItems {
		deployment, err := s.deployRepo.GetDeploymentByID(item.DeploymentID)
		if err != nil || deployment == nil {
			item.Status = "broken"
			broken = append(broken, item)
			continue
		}

		status := s.checkItemHealth(deployment, &item)
		if status == "broken" {
			item.Status = "broken"
			broken = append(broken, item)
		}
	}

	if broken == nil {
		broken = []model.DeploymentItem{}
	}
	return broken, nil
}

// RepairItem 修复异常部署子项
// 参数 deploymentID: 部署 ID
// 参数 itemID: 明细 ID
// 返回: 错误信息
func (s *DeployService) RepairItem(deploymentID, itemID string) error {
	deployment, err := s.deployRepo.GetDeploymentByID(deploymentID)
	if err != nil {
		return model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if deployment == nil {
		return model.NewBizError(model.ErrDeployNotFound, "部署记录不存在")
	}

	item, err := s.deployRepo.GetDeploymentItemByID(itemID)
	if err != nil {
		return model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if item == nil {
		return model.NewBizError(model.ErrDeployNotFound, "部署明细不存在")
	}

	resource, err := s.resourceRepo.GetResourceByID(item.ResourceID)
	if err != nil {
		return model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if resource == nil {
		return model.NewBizError(model.ErrResourceNotFound, "关联资源已被删除，无法修复")
	}

	switch resource.Type {
	case "skill":
		_, err = s.deploySkill(resource, deployment.TargetPath, true)
	case "agent":
		_, err = s.deployAgent(resource, deployment.TargetPath, true)
	case "config":
		_, err = s.deployConfig(resource, deployment.TargetPath, true)
	case "prompt":
		_, err = s.deployPrompt(resource, deployment.TargetPath, true)
	}

	return err
}

// CleanItem 清理部署子项记录（仅删除记录，不撤销部署）
// 参数 itemID: 明细 ID
// 返回: 错误信息
func (s *DeployService) CleanItem(itemID string, undeploy bool) error {
	item, err := s.deployRepo.GetDeploymentItemByID(itemID)
	if err != nil {
		return model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if item == nil {
		return model.NewBizError(model.ErrDeployNotFound, "部署明细不存在")
	}

	// undeploy=true 时先撤销实际文件（删 symlink 或从 json 移除 key）
	if undeploy {
		dep, _ := s.deployRepo.GetDeploymentByID(item.DeploymentID)
		if dep != nil {
			if dep.DeployType == "merge" {
				s.removeMergeResource(dep.TargetPath, item.ResourceID)
			} else {
				if _, e := os.Lstat(item.LinkPath); e == nil {
					os.Remove(item.LinkPath)
				}
			}
			s.removeFromMetaJSON(dep.TargetPath, dep.ID)
		}
	}

	if err := s.deployRepo.DeleteDeploymentItem(itemID); err != nil {
		return model.NewBizError(model.ErrSystemDB, err.Error())
	}

	// 部署下无更多明细时，删除部署记录
	count, err := s.deployRepo.GetDeploymentItemCount(item.DeploymentID)
	if err != nil {
		return model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if count == 0 {
		if err := s.deployRepo.DeleteDeployment(item.DeploymentID); err != nil {
			return model.NewBizError(model.ErrSystemDB, err.Error())
		}
	}

	return nil
}

// DeploySingleResourceToTarget 部署单个资源到指定目标（追踪联动用）
// 参数 deploymentID: 追踪部署 ID
// 参数 resource: 资源实体
// 参数 targetPath: 目标路径
// 返回: 错误信息
// 说明: 幂等——若该 deployment 下已存在此资源的 item，直接跳过，避免重复添加
//
//	（向已含该资源的分组重复关联同一资源时会触发，需按 resource_id 去重）
func (s *DeployService) DeploySingleResourceToTarget(deploymentID string, resource *model.Resource, targetPath string) error {
	// 幂等检查：该资源是否已在此 deployment 中部署
	existing, err := s.deployRepo.GetDeploymentItems(deploymentID)
	if err == nil {
		for _, it := range existing {
			if it.ResourceID == resource.ID {
				return nil
			}
		}
	}

	var linkPath string

	switch resource.Type {
	case "skill":
		linkPath, err = s.deploySkill(resource, targetPath, true)
	case "agent":
		linkPath, err = s.deployAgent(resource, targetPath, true)
	case "config":
		linkPath, err = s.deployConfig(resource, targetPath, true)
	case "prompt":
		linkPath, err = s.deployPrompt(resource, targetPath, true)
	default:
		return nil
	}
	if err != nil {
		return err
	}

	item := &model.DeploymentItem{
		ID:           util.NewUUID(),
		DeploymentID: deploymentID,
		ResourceID:   resource.ID,
		LinkPath:     linkPath,
	}
	return s.deployRepo.InsertDeploymentItem(item)
}

// UndeployResourceFromTarget 从目标撤销单个资源的部署（追踪联动用）
// 参数 resourceID: 资源 ID
// 参数 deploymentID: 追踪部署 ID
// 参数 targetPath: 目标路径
// 参数 deployType: 部署类型（空字符串时自动从 deployment 记录获取）
// 返回: 错误信息
func (s *DeployService) UndeployResourceFromTarget(resourceID, deploymentID, targetPath, deployType string) error {
	// 如果未指定 deployType，从 deployment 记录获取
	if deployType == "" {
		d, err := s.deployRepo.GetDeploymentByID(deploymentID)
		if err == nil && d != nil {
			deployType = d.DeployType
		}
	}

	items, err := s.deployRepo.GetDeploymentItemsByResourceID(resourceID)
	if err != nil {
		return err
	}

	for _, item := range items {
		if item.DeploymentID != deploymentID {
			continue
		}
		if deployType == "merge" {
			s.removeMergeResource(targetPath, item.ResourceID)
		} else {
			if _, e := os.Lstat(item.LinkPath); e == nil {
				os.Remove(item.LinkPath)
			}
		}
		s.deployRepo.DeleteDeploymentItem(item.ID)
	}

	// 如果 deployment 下没有其他 item 了，删除整条 deployment 记录
	count, err := s.deployRepo.GetDeploymentItemCount(deploymentID)
	if err == nil && count == 0 {
		s.removeFromMetaJSON(targetPath, deploymentID)
		s.deployRepo.DeleteDeployment(deploymentID)
	}

	return nil
}

// --- 内部方法 ---

// resolveTargetPath 解析部署目标路径
func (s *DeployService) resolveTargetPath(req *model.DeployRequest) (string, error) {
	if req.AliasID != nil && *req.AliasID != "" {
		alias, err := s.aliasRepo.GetAliasByID(*req.AliasID)
		if err != nil {
			return "", model.NewBizError(model.ErrSystemDB, err.Error())
		}
		if alias == nil {
			return "", model.NewBizError(model.ErrAliasNotFound, "路径别名不存在")
		}
		expanded, err := util.ExpandPath(alias.Path)
		if err != nil {
			return "", model.NewBizError(model.ErrDeployInvalid, fmt.Sprintf("展开别名路径失败: %v", err))
		}
		return filepath.Clean(expanded), nil
	}

	if req.TargetPath == "" {
		return "", model.NewBizError(model.ErrDeployInvalid, "target_path 和 alias_id 不能同时为空")
	}

	expanded, err := util.ExpandPath(req.TargetPath)
	if err != nil {
		return "", model.NewBizError(model.ErrDeployInvalid, fmt.Sprintf("展开目标路径失败: %v", err))
	}
	return filepath.Clean(expanded), nil
}

// resolveResources 确定要部署的资源列表
func (s *DeployService) resolveResources(req *model.DeployRequest) ([]model.Resource, error) {
	// 优先处理批量资源 ID
	if len(req.ResourceIDs) > 0 {
		var resources []model.Resource
		for _, rid := range req.ResourceIDs {
			r, err := s.resourceRepo.GetResourceByID(rid)
			if err != nil {
				return nil, model.NewBizError(model.ErrSystemDB, err.Error())
			}
			if r == nil {
				return nil, model.NewBizError(model.ErrResourceNotFound, fmt.Sprintf("资源不存在: %s", rid))
			}
			resources = append(resources, *r)
		}
		return resources, nil
	}

	if req.ResourceID != nil && *req.ResourceID != "" {
		r, err := s.resourceRepo.GetResourceByID(*req.ResourceID)
		if err != nil {
			return nil, model.NewBizError(model.ErrSystemDB, err.Error())
		}
		if r == nil {
			return nil, model.NewBizError(model.ErrResourceNotFound, "资源不存在")
		}
		return []model.Resource{*r}, nil
	}

	if req.GroupID != nil && *req.GroupID != "" {
		resourceIDs, err := s.groupRepo.GetGroupResources(*req.GroupID)
		if err != nil {
			return nil, model.NewBizError(model.ErrSystemDB, err.Error())
		}
		var resources []model.Resource
		for _, rid := range resourceIDs {
			r, err := s.resourceRepo.GetResourceByID(rid)
			if err != nil {
				return nil, model.NewBizError(model.ErrSystemDB, err.Error())
			}
			if r != nil {
				resources = append(resources, *r)
			}
		}
		return resources, nil
	}

	return nil, model.NewBizError(model.ErrDeployInvalid, "group_id、resource_id、resource_ids 不能全为空")
}

// deploySkill 部署 skill 类型资源（创建符号链接到目标目录）
func (s *DeployService) deploySkill(resource *model.Resource, targetPath string, force bool) (string, error) {
	if err := util.EnsureDir(targetPath); err != nil {
		return "", model.NewBizError(model.ErrDeployFailed, fmt.Sprintf("创建目标目录失败: %v", err))
	}

	linkDst := filepath.Join(targetPath, resource.Name)

	if _, err := os.Lstat(linkDst); err == nil {
		if !force {
			return "", model.NewBizError(model.ErrDeployFailed, fmt.Sprintf("目标已存在: %s，使用 force=true 覆盖", linkDst))
		}
		os.RemoveAll(linkDst)
	}

	if resource.Path == "" {
		return "", model.NewBizError(model.ErrDeployFailed, "资源存储路径为空")
	}
	source := resource.Path
	if err := util.CreateSymlink(source, linkDst); err != nil {
		return "", model.NewBizError(model.ErrDeployFailed, fmt.Sprintf("创建符号链接失败: %v", err))
	}

	return linkDst, nil
}

// deployAgent 部署 agent 类型资源（创建符号链接到目标目录）
func (s *DeployService) deployAgent(resource *model.Resource, targetPath string, force bool) (string, error) {
	if err := util.EnsureDir(targetPath); err != nil {
		return "", model.NewBizError(model.ErrDeployFailed, fmt.Sprintf("创建目标目录失败: %v", err))
	}

	linkDst := filepath.Join(targetPath, resource.Name+".md")

	if _, err := os.Lstat(linkDst); err == nil {
		if !force {
			return "", model.NewBizError(model.ErrDeployFailed, fmt.Sprintf("目标已存在: %s，使用 force=true 覆盖", linkDst))
		}
		os.Remove(linkDst)
	}

	if resource.Path == "" {
		return "", model.NewBizError(model.ErrDeployFailed, "资源存储路径为空")
	}
	source := resource.Path
	if err := util.CreateSymlink(source, linkDst); err != nil {
		return "", model.NewBizError(model.ErrDeployFailed, fmt.Sprintf("创建符号链接失败: %v", err))
	}

	return linkDst, nil
}

// deployConfig 部署 config 类型资源（深度合并到目标配置文件中）
// targetPath 是一个已存在的配置文件（.json/.yaml/.yml/.toml）
// 采用 lodash 式深度合并：嵌套对象递归合并，标量/数组由源覆盖目标
// 返回 link_path 用 key（顶层第一个 key；为兼容历史 MCP 用法，若存在 mcpServers 子键则优先取其下第一项）
func (s *DeployService) deployConfig(resource *model.Resource, targetPath string, force bool) (string, error) {
	// 1. 检测目标格式
	format := util.DetectConfigFormat(targetPath)
	if format == "" {
		return "", model.NewBizError(model.ErrDeployInvalid,
			fmt.Sprintf("Config 目标文件后缀必须是 .json/.yaml/.yml/.toml: %s", targetPath))
	}

	// 2. 读取资源自身的配置文件（路径在 DB 中以 {uuid}.{ext} 形式存储,后缀由创建时决定）
	cfgFragment, _, err := s.readConfigFragment(resource)
	if err != nil {
		return "", model.NewBizError(model.ErrDeployFailed, err.Error())
	}
	// 空片段视为 {}：无需合并,目标内容不变(新建未编辑的 config 即此情形)。
	// 返回空 link_path,健康检查对空 config 直接判定 ok(无 key 可校验)。
	if len(cfgFragment) == 0 {
		return "", nil
	}

	// 4. 链接 path(健康检查/撤销的锚点)
	//    优先 mcpServers 子键下的第一项(兼容历史),否则取顶层第一项
	linkPath := pickLinkPath(cfgFragment)

	// 5. 读取目标现有内容(用于冲突检测)
	var targetKeys map[string]interface{}
	if util.FileExists(targetPath) {
		data, rErr := os.ReadFile(targetPath)
		if rErr == nil && len(data) > 0 {
			targetKeys, _ = util.ParseConfigBytes(data, format)
		}
	}
	if targetKeys == nil {
		targetKeys = map[string]interface{}{}
	}

	// 6. 检测冲突(路径树语义:相同位置相同 key 且至少一方非 map 才算冲突)
	conflictResources := s.findConfigConflicts(targetPath, resource.ID, cfgFragment)
	// 与目标原始内容(非已部署 MCP 管理的部分)按同样语义比对
	managedKeys := s.getManagedKeys(targetPath, resource.ID)
	originalOnly := map[string]interface{}{}
	for k, v := range targetKeys {
		if managedKeys[k] {
			continue
		}
		originalOnly[k] = v
	}
	hasOriginalConflict := configsConflict(cfgFragment, originalOnly)
	hasConflict := len(conflictResources) > 0 || hasOriginalConflict

	if hasConflict && !force {
		names := make([]string, 0)
		for _, cr := range conflictResources {
			names = append(names, cr.name)
		}
		if hasOriginalConflict {
			names = append(names, "原始内容")
		}
		return "", model.NewBizErrorWithData(
			model.ErrDeployFailed,
			"与已有内容存在 key 冲突",
			map[string]interface{}{"conflicts": names},
		)
	}

	// 7. force 覆盖：先撤销冲突资源的部署
	if len(conflictResources) > 0 && force {
		for _, cr := range conflictResources {
			s.undeployConfigResource(cr.resourceID, cr.deploymentID, targetPath)
		}
	}

	// 7.5 force 重部署幂等：先移除本资源自身上次的贡献，再重新合并。
	//     数组合并为"拼接"语义（非覆盖），若不先减自己，重复部署会让数组元素翻倍。
	//     map/标量减后重merge结果不变，此步对它们是安全的 no-op。
	if force && util.FileExists(targetPath) {
		_ = s.removeConfigKeysFromTarget(targetPath, cfgFragment)
	}

	// 8. 实际合并写入(由 format 决定 JSON/YAML/TOML 分支)
	if err := util.MergeConfigToFile(targetPath, format, cfgFragment); err != nil {
		return "", model.NewBizError(model.ErrDeployFailed, fmt.Sprintf("合并写入失败: %v", err))
	}

	return linkPath, nil
}

// deployPrompt 部署 prompt 类型资源 (追加资源全文到目标 .md 末尾,前后包裹分隔符标记)
//
// 流程:
//  1. 读取资源源文件 (storage/prompts/{uuid}.md) 全文
//  2. force=true 时,先 StripPromptBlock 删除目标中已有的同 UUID 块 (保证幂等)
//  3. 拼接 BuildPromptBlock,以 append 模式写入目标 .md
//  4. linkPath = targetPath (健康检查/撤销直接用 targetPath + uuid 定位)
//
// 注意:
//   - force=false 且目标已含分隔符块时,正常应由前端 checkConflicts 提前拦截;
//     这里若再遇到不会重复检测,直接追加(等同两次部署 → 两个块,健康检查仍 ok)。
//     但 force 路径必须保证幂等: 连续 force N 次,文件中只存在一个分隔符块。
func (s *DeployService) deployPrompt(resource *model.Resource, targetPath string, force bool) (string, error) {
	// 校验目标文件
	if !util.IsPromptFile(targetPath) {
		return "", model.NewBizError(model.ErrDeployInvalid,
			fmt.Sprintf("Prompt 目标文件后缀必须是 .md: %s", targetPath))
	}
	if !util.FileExists(targetPath) {
		return "", model.NewBizError(model.ErrDeployFailed,
			fmt.Sprintf("目标文件不存在,请先创建: %s", targetPath))
	}
	if util.IsDir(targetPath) {
		return "", model.NewBizError(model.ErrDeployInvalid,
			fmt.Sprintf("Prompt 目标路径必须是文件而非目录: %s", targetPath))
	}

	// 读资源源文件
	if resource.Path == "" {
		return "", model.NewBizError(model.ErrDeployFailed, "资源 path 为空")
	}
	srcBytes, err := os.ReadFile(resource.Path)
	if err != nil {
		return "", model.NewBizError(model.ErrDeployFailed,
			fmt.Sprintf("读取 Prompt 资源文件失败: %v", err))
	}

	// 读目标当前内容
	dstBytes, err := os.ReadFile(targetPath)
	if err != nil {
		return "", model.NewBizError(model.ErrDeployFailed,
			fmt.Sprintf("读取 Prompt 目标文件失败: %v", err))
	}
	dstText := string(dstBytes)

	// force 模式: 先删旧块,保证幂等
	if force {
		dstText = util.StripPromptBlock(dstText, resource.ID)
	}

	// 规范化目标末尾: 保证文件以 \n 结尾,后续追加块前的 \n 不会粘连
	dstText = util.EnsureTrailingNewline(dstText)

	// 拼追加块
	block := util.BuildPromptBlock(resource.ID, string(srcBytes))
	out := dstText + block

	// 末尾仅保留一个 \n
	out = strings.TrimRight(out, "\n") + "\n"

	if err := os.WriteFile(targetPath, []byte(out), 0644); err != nil {
		return "", model.NewBizError(model.ErrDeployFailed,
			fmt.Sprintf("写入 Prompt 目标文件失败: %v", err))
	}

	return targetPath, nil
}

// pickLinkPath 选择 link_path: 优先 mcpServers 子键下第一项,否则顶层第一项
func pickLinkPath(m map[string]interface{}) string {
	if servers, ok := m["mcpServers"].(map[string]interface{}); ok {
		for k := range servers {
			return k
		}
	}
	for k := range m {
		return k
	}
	return ""
}

// configsConflict 按"相同位置相同 key"的路径树语义判断两个 config 片段是否冲突。
//
// 规则(与 util.DeepMerge 的合并语义对称):
//   - 沿相同路径下行,某一层两边都存在同一个 key 时:
//       · 两边该 key 的值【都是 map】 → 可深度合并,继续向更深一层递归比较
//       · 两边该 key 的值【都是 array】 → 可拼接合并,不冲突(数组永不冲突)
//       · 否则(任一方为标量,或一方 map/array 另一方非同类) → 该位置发生覆盖,即【冲突】
//   - 某层 key 不同 → 该分支分叉,不再深入(不冲突)
//
// 示例:
//   {a:{a1:[{a51}]}}     vs {a:{a1:[{a52}]}}        → a 深入, a1 两边都是数组 → 拼接, 不冲突
//   {a:{a1:{...}}}       vs {a:{b1:{...}}}          → a 同名且都是 map 深入, a1≠b1 分叉 → 不冲突
//   {a:{a1:1}}           vs {a:{a1:2}}              → a 深入, a1 同名但都是标量 → 冲突
//   {a:{a1:{...}}}       vs {a:{a1:[...]}}          → a1 一方 map 一方 array → 类型不一致, 冲突
func configsConflict(a, b map[string]interface{}) bool {
	for k, av := range a {
		bv, ok := b[k]
		if !ok {
			continue // 该位置 b 没有同名 key → 分叉,跳过
		}
		amap, aMapOk := av.(map[string]interface{})
		bmap, bMapOk := bv.(map[string]interface{})
		if aMapOk && bMapOk {
			// 两边都是 map → 继续向更深一层比较
			if configsConflict(amap, bmap) {
				return true
			}
			continue
		}
		_, aArrOk := av.([]interface{})
		_, bArrOk := bv.([]interface{})
		if aArrOk && bArrOk {
			// 两边都是 array → 拼接合并,不冲突
			continue
		}
		// 同名 key 但非(同为 map / 同为 array) → 覆盖,冲突
		return true
	}
	return false
}

// readConfigFragmentByID 按资源 ID 读取其 config 片段 map(失败返回空 map)
func (s *DeployService) readConfigFragmentByID(resourceID string) map[string]interface{} {
	r, err := s.resourceRepo.GetResourceByID(resourceID)
	if err != nil || r == nil {
		return map[string]interface{}{}
	}
	cfg, _, err := s.readConfigFragment(r)
	if err != nil || cfg == nil {
		return map[string]interface{}{}
	}
	return cfg
}

// readConfigFragment 从 config 资源存储路径读取片段 map
// 资源 path 由 createFiles 写入,后缀由用户创建时选定(默认 .json)
func (s *DeployService) readConfigFragment(resource *model.Resource) (map[string]interface{}, util.ConfigFormat, error) {
	if resource.Path == "" {
		return nil, "", fmt.Errorf("资源 path 为空")
	}
	return util.ReadConfigFragment(resource.Path)
}

// configConflictInfo 冲突的 Config 资源信息
type configConflictInfo struct {
	resourceID   string
	deploymentID string
	name         string
}

// findConfigConflicts 查找目标路径下已部署的 Config 中与新 config 片段(按路径树语义)冲突的资源
func (s *DeployService) findConfigConflicts(targetPath, selfResourceID string, newCfg map[string]interface{}) []configConflictInfo {
	allDeps, err := s.deployRepo.GetDeploymentsByTarget()
	if err != nil {
		return nil
	}
	deployments, ok := allDeps[targetPath]
	if !ok {
		return nil
	}

	var conflicts []configConflictInfo
	seen := map[string]bool{}

	for _, dep := range deployments {
		if dep.DeployType != "merge" {
			continue
		}
		items, err := s.deployRepo.GetDeploymentItems(dep.ID)
		if err != nil {
			continue
		}
		for _, item := range items {
			if item.ResourceID == selfResourceID {
				continue
			}
			if seen[item.ResourceID] {
				continue
			}
			existingCfg := s.readConfigFragmentByID(item.ResourceID)
			if configsConflict(newCfg, existingCfg) {
				seen[item.ResourceID] = true
				resName := item.ResourceID
				if r, rErr := s.resourceRepo.GetResourceByID(item.ResourceID); rErr == nil && r != nil {
					resName = r.Name
				}
				conflicts = append(conflicts, configConflictInfo{
					resourceID:   item.ResourceID,
					deploymentID: dep.ID,
					name:         resName,
				})
			}
		}
	}
	return conflicts
}

// getConfigResourceKeys 读取 Config 资源文件，返回其所有顶层 key + 一层子 key
func (s *DeployService) getConfigResourceKeys(resourceID string) map[string]bool {
	keys := map[string]bool{}
	r, err := s.resourceRepo.GetResourceByID(resourceID)
	if err != nil || r == nil {
		return keys
	}
	cfg, _, err := s.readConfigFragment(r)
	if err != nil || cfg == nil {
		return keys
	}
	for k, v := range cfg {
		keys[k] = true
		if sub, ok := v.(map[string]interface{}); ok {
			for sk := range sub {
				keys[sk] = true
			}
		}
	}
	return keys
}

// getManagedKeys 获取 targetPath 下所有已部署 Config 管理的 key 集合（排除 selfResourceID）
func (s *DeployService) getManagedKeys(targetPath, selfResourceID string) map[string]bool {
	managed := map[string]bool{}
	allDeps, err := s.deployRepo.GetDeploymentsByTarget()
	if err != nil {
		return managed
	}
	deployments, ok := allDeps[targetPath]
	if !ok {
		return managed
	}
	for _, dep := range deployments {
		if dep.DeployType != "merge" {
			continue
		}
		items, _ := s.deployRepo.GetDeploymentItems(dep.ID)
		for _, item := range items {
			if item.ResourceID == selfResourceID {
				continue
			}
			for k := range s.getConfigResourceKeys(item.ResourceID) {
				managed[k] = true
			}
		}
	}
	return managed
}

// undeployConfigResource 撤销某 Config 资源在指定目标的部署（删其写入的所有 key + 清 DB）
func (s *DeployService) undeployConfigResource(resourceID, deploymentID, targetPath string) {
	r, err := s.resourceRepo.GetResourceByID(resourceID)
	cfg := map[string]interface{}{}
	if err == nil && r != nil {
		cfg, _, _ = s.readConfigFragment(r)
	}

	if util.FileExists(targetPath) {
		if err := s.removeConfigKeysFromTarget(targetPath, cfg); err != nil {
			// 忽略具体写盘错误,仍清 DB
		}
	}
	s.UndeployResourceFromTarget(resourceID, deploymentID, targetPath, "merge")
}

// removeConfigKeysFromTarget 从目标文件中移除 cfg 包含的所有顶层 key + 一层子 key
// 自动按目标格式分派(JSON/YAML AST patch / TOML 整体重写)
func (s *DeployService) removeConfigKeysFromTarget(targetPath string, cfg map[string]interface{}) error {
	format := util.DetectConfigFormat(targetPath)
	if format == "" {
		return fmt.Errorf("未知的目标文件格式: %s", targetPath)
	}
	data, err := os.ReadFile(targetPath)
	if err != nil {
		return err
	}
	if len(cfg) == 0 {
		return nil
	}

	switch format {
	case util.FormatJSON:
		std, pErr := util.ParseJSONC(data)
		if pErr != nil {
			std = data
		}
		var obj map[string]interface{}
		if err := json.Unmarshal(std, &obj); err != nil {
			return err
		}
		subtractConfig(obj, cfg)
		out, err := json.MarshalIndent(obj, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(targetPath, out, 0644)
	case util.FormatYAML:
		var root yamlv3.Node
		if err := yamlv3.Unmarshal(data, &root); err != nil {
			return err
		}
		rootMap := util.PickRootMapping(&root)
		if rootMap != nil {
			subtractConfigFromYAML(rootMap, cfg)
		}
		var buf bytes.Buffer
		enc := yamlv3.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(&root); err != nil {
			return err
		}
		_ = enc.Close()
		return os.WriteFile(targetPath, buf.Bytes(), 0644)
	case util.FormatTOML:
		var obj map[string]interface{}
		if err := toml.Unmarshal(data, &obj); err != nil {
			return err
		}
		subtractConfig(obj, cfg)
		out, err := toml.Marshal(obj)
		if err != nil {
			return err
		}
		return os.WriteFile(targetPath, out, 0644)
	}
	return nil
}

// subtractConfig 从 target 中按路径树精确相减 frag 写入的内容（与 util.DeepMerge 对称）。
//
// 规则:
//   - 两边同 key 且都是 map → 递归相减;子 map 减空后才删除该父 key（保留有兄弟的父节点）
//   - 两边同 key 且都是 array → 按值逐个移除 frag 的元素（拼接的逆操作）;数组减空后删除该 key
//   - 同 key 但类型不一致（标量/类型不匹配）→ 删除该 key
//   - target 中 frag 没有的 key → 保留不动
//
// 这保证多个 config 合并到同一深层路径时，移除其一只删自己的贡献，
// 不会误删共享父节点下其他 config 的兄弟 key / 数组元素。
func subtractConfig(target, frag map[string]interface{}) {
	if target == nil || frag == nil {
		return
	}
	for k, fv := range frag {
		tv, ok := target[k]
		if !ok {
			continue
		}
		fMap, fMapOk := fv.(map[string]interface{})
		tMap, tMapOk := tv.(map[string]interface{})
		if fMapOk && tMapOk {
			subtractConfig(tMap, fMap)
			if len(tMap) == 0 {
				delete(target, k)
			}
			continue
		}
		fArr, fArrOk := fv.([]interface{})
		tArr, tArrOk := tv.([]interface{})
		if fArrOk && tArrOk {
			rest := subtractArray(tArr, fArr)
			if len(rest) == 0 {
				delete(target, k)
			} else {
				target[k] = rest
			}
			continue
		}
		delete(target, k)
	}
}

// configContains 深度子集校验:target 是否完整包含 frag 描述的所有路径与值。
// 是 subtractConfig 的"只读版"——用于部署健康检查,判断资源当初写入的
// 整棵嵌套结构是否仍存在于目标文件中(深层 key 被改/删都应判 broken)。
//   - frag 的 map → target 同 key 必须是 map 且递归包含
//   - frag 的 array → target 同 key 必须是 array 且按值(looseEqual)逐个包含
//     (拼接合并语义:frag 每个元素都能在 target 中一对一找到)
//   - frag 的标量 → target 同 key 必须 looseEqual
func configContains(target, frag map[string]interface{}) bool {
	if frag == nil {
		return true
	}
	if target == nil {
		return false
	}
	for k, fv := range frag {
		tv, ok := target[k]
		if !ok {
			return false
		}
		fMap, fMapOk := fv.(map[string]interface{})
		if fMapOk {
			tMap, tMapOk := tv.(map[string]interface{})
			if !tMapOk || !configContains(tMap, fMap) {
				return false
			}
			continue
		}
		fArr, fArrOk := fv.([]interface{})
		if fArrOk {
			tArr, tArrOk := tv.([]interface{})
			if !tArrOk || !arrayContains(tArr, fArr) {
				return false
			}
			continue
		}
		if !looseEqual(tv, fv) {
			return false
		}
	}
	return true
}

// arrayContains 判断 target 数组是否按值(looseEqual)一对一包含 frag 的每个元素。
func arrayContains(target, frag []interface{}) bool {
	used := make([]bool, len(target))
	for _, fe := range frag {
		found := false
		for i := range target {
			if used[i] {
				continue
			}
			if looseEqual(target[i], fe) {
				used[i] = true
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// subtractArray 从 target 数组中按值移除 frag 的每个元素（拼接合并的逆操作）。
// 对 frag 中的每个元素,在 target 中找到第一个值相等的元素删除（一对一,
// 重复元素只删等量的份数）。值相等用 looseEqual 做跨格式(JSON float64 / YAML int)归一比较。
func subtractArray(target, frag []interface{}) []interface{} {
	used := make([]bool, len(target))
	for _, fe := range frag {
		for i := range target {
			if used[i] {
				continue
			}
			if looseEqual(target[i], fe) {
				used[i] = true
				break
			}
		}
	}
	rest := make([]interface{}, 0, len(target))
	for i, e := range target {
		if !used[i] {
			rest = append(rest, e)
		}
	}
	return rest
}

// looseEqual 跨格式宽松深度相等:数字统一按 float64 比较(JSON 解析为 float64,
// YAML/TOML 可能为 int/int64),map/slice 递归,其余用 ==。
func looseEqual(a, b interface{}) bool {
	an, aok := toFloat(a)
	bn, bok := toFloat(b)
	if aok && bok {
		return an == bn
	}
	am, amok := a.(map[string]interface{})
	bm, bmok := b.(map[string]interface{})
	if amok && bmok {
		if len(am) != len(bm) {
			return false
		}
		for k, av := range am {
			bv, ok := bm[k]
			if !ok || !looseEqual(av, bv) {
				return false
			}
		}
		return true
	}
	as, asok := a.([]interface{})
	bs, bsok := b.([]interface{})
	if asok && bsok {
		if len(as) != len(bs) {
			return false
		}
		for i := range as {
			if !looseEqual(as[i], bs[i]) {
				return false
			}
		}
		return true
	}
	return a == b
}

// toFloat 把各种数值类型归一为 float64
func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	default:
		return 0, false
	}
}

// subtractConfigFromYAML 是 subtractConfig 的 yaml.Node 版本（保留注释/格式/key 顺序）。
// m 为 mapping 节点；frag 为待相减的资源片段。
func subtractConfigFromYAML(m *yamlv3.Node, frag map[string]interface{}) {
	if m == nil || m.Kind != yamlv3.MappingNode {
		return
	}
	// 倒序收集要删除的 key 下标（避免删除时下标位移）
	var delIdx []int
	for i := 0; i < len(m.Content); i += 2 {
		key := m.Content[i].Value
		fv, ok := frag[key]
		if !ok {
			continue
		}
		val := m.Content[i+1]
		fMap, fMapOk := fv.(map[string]interface{})
		if fMapOk && val.Kind == yamlv3.MappingNode {
			// 两边都是 map → 递归;子 map 减空后再删该父 key
			subtractConfigFromYAML(val, fMap)
			if len(val.Content) == 0 {
				delIdx = append([]int{i}, delIdx...)
			}
			continue
		}
		fArr, fArrOk := fv.([]interface{})
		if fArrOk && val.Kind == yamlv3.SequenceNode {
			// 两边都是 array → 按值移除 frag 的元素;数组减空后删该 key
			subtractSequenceNode(val, fArr)
			if len(val.Content) == 0 {
				delIdx = append([]int{i}, delIdx...)
			}
			continue
		}
		// 标量/类型不一致 → 删除
		delIdx = append([]int{i}, delIdx...)
	}
	for _, i := range delIdx {
		m.Content = append(m.Content[:i], m.Content[i+2:]...)
	}
}

// subtractSequenceNode 从 yaml 序列节点中按值移除 frag 的每个元素(拼接的逆操作)。
// 把每个子节点 Decode 成 interface{} 后用 looseEqual 与 frag 元素一对一比对删除。
func subtractSequenceNode(seq *yamlv3.Node, frag []interface{}) {
	if seq == nil || seq.Kind != yamlv3.SequenceNode {
		return
	}
	used := make([]bool, len(seq.Content))
	for _, fe := range frag {
		for i, child := range seq.Content {
			if used[i] {
				continue
			}
			var decoded interface{}
			if err := child.Decode(&decoded); err != nil {
				continue
			}
			if looseEqual(decoded, fe) {
				used[i] = true
				break
			}
		}
	}
	kept := make([]*yamlv3.Node, 0, len(seq.Content))
	for i, child := range seq.Content {
		if !used[i] {
			kept = append(kept, child)
		}
	}
	seq.Content = kept
}

// removeConfigResourceKeys 根据资源 ID 读取其 Config 片段，从目标文件中移除该资源写入的所有 key
// (兼容旧 API 调用; 复用 removeConfigKeysFromTarget)
func (s *DeployService) removeConfigResourceKeys(targetPath, resourceID string) {
	r, err := s.resourceRepo.GetResourceByID(resourceID)
	cfg := map[string]interface{}{}
	if err == nil && r != nil {
		cfg, _, _ = s.readConfigFragment(r)
	}
	if !util.FileExists(targetPath) {
		return
	}
	_ = s.removeConfigKeysFromTarget(targetPath, cfg)
}

// removePromptResourceMarkers 根据资源 ID 从目标 .md 文件中删除其分隔符标记块
// resourceID 即资源 UUID,直接作为分隔符锚点
// 目标文件不存在时静默跳过(与 config 行为一致)
func (s *DeployService) removePromptResourceMarkers(targetPath, resourceID string) {
	if !util.FileExists(targetPath) {
		return
	}
	data, err := os.ReadFile(targetPath)
	if err != nil {
		return
	}
	stripped := util.StripPromptBlock(string(data), resourceID)
	_ = os.WriteFile(targetPath, []byte(stripped), 0644)
}

// removeMergeResource 按资源 Type 分流的 merge 撤销
//   - config: 走 removeConfigResourceKeys (按 key 删除)
//   - prompt: 走 removePromptResourceMarkers (按分隔符删除)
//
// 资源已不存在时(如先删了资源)无法判定 type,默认按 config 兜底
// (历史数据多为 config; prompt 资源删除时已先级联撤销部署,不会走到此分支)
func (s *DeployService) removeMergeResource(targetPath, resourceID string) {
	r, _ := s.resourceRepo.GetResourceByID(resourceID)
	if r != nil && r.Type == "prompt" {
		s.removePromptResourceMarkers(targetPath, resourceID)
		return
	}
	s.removeConfigResourceKeys(targetPath, resourceID)
}

// isMergeResourceType 返回该资源类型的部署语义是否为 merge（深度合并写入目标文件）。
// config / prompt → merge；skill / agent → symlink（软链接）。
//
// 这是部署语义的唯一真相来源：由资源类型唯一决定。deployment.DeployType 是
// 跨多个 item 的聚合字段（混合部署时按"全 merge 才算 merge"折叠），且从
// _meta.json 还原时曾被错误推断，不能作为单个 item 行为的判据。
func isMergeResourceType(t string) bool {
	return t == "config" || t == "prompt"
}

// checkItemHealth 检查单个部署明细的健康状态
func (s *DeployService) checkItemHealth(deployment *model.Deployment, item *model.DeploymentItem) string {
	// 部署语义按资源类型判定，而非 deployment.DeployType（见 isMergeResourceType 注释）。
	r, _ := s.resourceRepo.GetResourceByID(item.ResourceID)
	var isMerge bool
	if r != nil {
		isMerge = isMergeResourceType(r.Type)
	} else {
		// 资源已被删除，无法按类型判定，回退到 deployment 记录
		isMerge = deployment.DeployType == "merge"
	}

	if isMerge {
		// prompt: 按分隔符标记判定
		if r != nil && r.Type == "prompt" {
			data, err := os.ReadFile(deployment.TargetPath)
			if err != nil {
				return "broken"
			}
			if util.HasPromptMarkers(string(data), item.ResourceID) {
				return "ok"
			}
			return "broken"
		}

		// Config 合并部署: targetPath 是配置文件,检查 link_path 对应的 key 是否仍在
		// 空 link_path 表示空 config 片段(无 key 写入目标),无可校验内容,直接判 ok
		if item.LinkPath == "" {
			return "ok"
		}
		format := util.DetectConfigFormat(deployment.TargetPath)
		if format == "" {
			return "broken"
		}
		data, err := os.ReadFile(deployment.TargetPath)
		if err != nil {
			return "broken"
		}
		settings, err := util.ParseConfigBytes(data, format)
		if err != nil || settings == nil {
			return "broken"
		}
		// 用资源自身的完整 config 片段对目标做深度子集校验:
		// 资源写入目标的是整棵嵌套路径(如 a.a1...a52),只校验顶层 key
		// 会漏判"深层 key 被改掉/删掉"的情况。
		frag := s.readConfigFragmentByID(item.ResourceID)
		if len(frag) > 0 {
			if configContains(settings, frag) {
				return "ok"
			}
			return "broken"
		}
		// 兜底(读不到资源片段时):退回旧的顶层/一层子 key 存在性判断
		if _, exists := settings[item.LinkPath]; exists {
			return "ok"
		}
		for _, v := range settings {
			if sub, ok := v.(map[string]interface{}); ok {
				if _, exists := sub[item.LinkPath]; exists {
					return "ok"
				}
			}
		}
		return "broken"
	}

	if _, err := os.Lstat(item.LinkPath); err != nil {
		return "broken"
	}
	return "ok"
}

// metaDirFor 计算目标路径对应的 .aiResource 目录
//   - MCP 目标是 .json 文件 → meta 放在该文件的父目录下（文件下不能建目录）
//   - skill/agent 目标是目录 → meta 放在该目录下
func metaDirFor(targetPath string) string {
	base := targetPath
	if filepath.Ext(targetPath) == ".json" {
		base = filepath.Dir(targetPath)
	}
	return filepath.Join(base, ".aiResource")
}

// writeMetaJSON 更新目标路径的 _meta.json
func (s *DeployService) writeMetaJSON(targetPath string, deployment *model.Deployment, results []deployResultItem) {
	metaDir := metaDirFor(targetPath)
	metaPath := filepath.Join(metaDir, "_meta.json")

	var meta model.MetaJSON
	if util.FileExists(metaPath) {
		data, err := os.ReadFile(metaPath)
		if err == nil {
			json.Unmarshal(data, &meta)
		}
	}

	meta.ManagedBy = "AiResourceManager"
	meta.Version = 1

	now := time.Now().Format(time.RFC3339)
	for _, r := range results {
		entry := model.MetaDeployment{
			DeploymentID: deployment.ID,
			Type:         r.resource.Type,
			ResourceUUID: r.resource.ID,
			ResourceName: r.resource.Name,
			DeployedAt:   now,
		}
		if r.deployType == "merge" {
			entry.MCPKey = r.linkPath
		} else {
			entry.LinkPath = r.linkPath
		}
		meta.Deployments = append(meta.Deployments, entry)
	}

	util.EnsureDir(metaDir)
	output, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(metaPath, output, 0644)
}

// removeFromMetaJSON 从 _meta.json 中移除部署记录
func (s *DeployService) removeFromMetaJSON(targetPath, deploymentID string) {
	metaPath := filepath.Join(metaDirFor(targetPath), "_meta.json")
	if !util.FileExists(metaPath) {
		return
	}

	data, err := os.ReadFile(metaPath)
	if err != nil {
		return
	}

	var meta model.MetaJSON
	if err := json.Unmarshal(data, &meta); err != nil {
		return
	}

	var filtered []model.MetaDeployment
	for _, d := range meta.Deployments {
		if d.DeploymentID != deploymentID {
			filtered = append(filtered, d)
		}
	}
	meta.Deployments = filtered

	output, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(metaPath, output, 0644)
}

// cleanOldDeploymentItems 清理资源在指定目标路径的旧部署明细记录
// 若清理后 deployment 下无其他明细，则删除整条 deployment 记录
func (s *DeployService) cleanOldDeploymentItems(resourceID, targetPath string) {
	items, err := s.deployRepo.GetDeploymentItemsByResourceAndTarget(resourceID, targetPath)
	if err != nil || len(items) == 0 {
		return
	}

	for _, item := range items {
		// 删除明细记录
		s.deployRepo.DeleteDeploymentItem(item.ID)
		// 检查该 deployment 是否还有其他 item
		count, err := s.deployRepo.GetDeploymentItemCount(item.DeploymentID)
		if err == nil && count == 0 {
			// 无其他明细，删除整条部署记录和 _meta.json 引用
			s.removeFromMetaJSON(targetPath, item.DeploymentID)
			s.deployRepo.DeleteDeployment(item.DeploymentID)
		}
	}
}

// RestoreFromMeta 从目标路径的 _meta.json 还原部署记录
// 检查每条记录引用的资源是否仍存在，存在则创建 deployment + deployment_item，
// 不存在则从 _meta.json 中移除该条目
// 参数 targetPath: 目标路径
// 参数 aliasID: 新创建的别名 ID
func (s *DeployService) RestoreFromMeta(targetPath, aliasID string) {
	// 如果该路径已有 deployment 记录（说明数据已在 DB，不需要从 _meta 还原），
	// 只需把已有 deployment 关联到新别名
	allDeps, _ := s.deployRepo.GetDeploymentsByTarget()
	if deps, ok := allDeps[targetPath]; ok && len(deps) > 0 {
		// 已有部署记录，更新 alias_id 关联
		for _, dep := range deps {
			if dep.AliasID == nil || *dep.AliasID == "" {
				s.deployRepo.UpdateDeploymentAliasID(dep.ID, aliasID)
			}
		}
		return
	}

	metaPath := filepath.Join(metaDirFor(targetPath), "_meta.json")
	if !util.FileExists(metaPath) {
		return
	}

	data, err := os.ReadFile(metaPath)
	if err != nil {
		return
	}

	var meta model.MetaJSON
	if err := json.Unmarshal(data, &meta); err != nil {
		return
	}

	if len(meta.Deployments) == 0 {
		return
	}

	// 分离有效和无效的条目
	var validEntries []model.MetaDeployment
	var invalidEntries []model.MetaDeployment

	for _, entry := range meta.Deployments {
		r, err := s.resourceRepo.GetResourceByID(entry.ResourceUUID)
		if err != nil || r == nil {
			invalidEntries = append(invalidEntries, entry)
		} else {
			validEntries = append(validEntries, entry)
		}
	}

	// 如果有无效条目，更新 _meta.json
	if len(invalidEntries) > 0 {
		meta.Deployments = validEntries
		output, err := json.MarshalIndent(meta, "", "  ")
		if err == nil {
			os.WriteFile(metaPath, output, 0644)
		}
	}

	// 为有效条目创建 deployment 记录
	if len(validEntries) == 0 {
		return
	}

	// 按 deployment_id 分组
	groupedByDeployID := make(map[string][]model.MetaDeployment)
	for _, entry := range validEntries {
		groupedByDeployID[entry.DeploymentID] = append(groupedByDeployID[entry.DeploymentID], entry)
	}

	aliasPtr := &aliasID
	for _, entries := range groupedByDeployID {
		// 确定 deploy_type：按资源类型判定（config/prompt=merge, skill/agent=symlink）。
		// 旧逻辑用 e.Type == "mcp" 判断，但 _meta.json 里 type 存的是资源类型
		// （"config" 等），永远匹配不上，导致 config 部署被错还原成 symlink。
		// 与 finalDeployType 同规则：全为 merge 才算 merge，混合则 symlink。
		hasMerge := false
		hasSymlink := false
		for _, e := range entries {
			if isMergeResourceType(e.Type) {
				hasMerge = true
			} else {
				hasSymlink = true
			}
		}
		deployType := "symlink"
		if hasMerge && !hasSymlink {
			deployType = "merge"
		}

		// 创建新的 deployment 记录
		deployment := &model.Deployment{
			ID:         util.NewUUID(),
			TargetPath: targetPath,
			AliasID:    aliasPtr,
			DeployType: deployType,
			Track:      0,
			CreatedAt:  time.Now().Format(time.RFC3339),
		}

		if _, err := s.deployRepo.InsertDeployment(deployment); err != nil {
			continue
		}

		// 创建 deployment_item
		for _, entry := range entries {
			linkPath := entry.LinkPath
			if entry.MCPKey != "" {
				linkPath = entry.MCPKey
			}
			item := &model.DeploymentItem{
				ID:           util.NewUUID(),
				DeploymentID: deployment.ID,
				ResourceID:   entry.ResourceUUID,
				LinkPath:     linkPath,
			}
			s.deployRepo.InsertDeploymentItem(item)
		}

		// 更新 _meta.json 中的 deployment_id 为新 ID
		for i, entry := range meta.Deployments {
			for _, e := range entries {
				if entry.ResourceUUID == e.ResourceUUID && entry.DeploymentID == e.DeploymentID {
					meta.Deployments[i].DeploymentID = deployment.ID
				}
			}
		}
	}

	// 写回更新后的 _meta.json（含新 deployment_id）
	output, err := json.MarshalIndent(meta, "", "  ")
	if err == nil {
		os.WriteFile(metaPath, output, 0644)
	}
}

// CheckConflicts 批量部署预检冲突（不写入任何文件）
//
// 按资源类型分流:
//   - prompt: 走 findPromptConflicts (按分隔符独立判定每对 resource×target,无 Union-Find)
//   - config: 原有逻辑 (Union-Find + key 冲突)
//
// 检测：1. 待部署资源之间的 key 冲突  2. 与目标文件已有内容（原始+已部署Config）的冲突
// 返回 has_conflict 仅在存在真正需要用户决策的冲突时为 true
// (ignored=被牺牲的资源, existing=与已有内容的冲突)；纯 applied 项不算冲突
func (s *DeployService) CheckConflicts(req *model.CheckConflictsReq) (*model.CheckConflictsResp, error) {
	// 资源类型分流: 取第一个待部署资源的 type 决定走哪条预检路径
	if len(req.ResourceIDs) > 0 {
		first, _ := s.resourceRepo.GetResourceByID(req.ResourceIDs[0])
		if first != nil && first.Type == "prompt" {
			return s.checkPromptConflicts(req)
		}
	}

	// 解析目标路径
	targetPath := req.TargetPath
	if req.AliasID != nil && *req.AliasID != "" {
		alias, err := s.aliasRepo.GetAliasByID(*req.AliasID)
		if err != nil || alias == nil {
			return nil, model.NewBizError(model.ErrAliasNotFound, "路径别名不存在")
		}
		expanded, _ := util.ExpandPath(alias.Path)
		targetPath = filepath.Clean(expanded)
	} else if targetPath != "" {
		expanded, _ := util.ExpandPath(targetPath)
		targetPath = filepath.Clean(expanded)
	}

	// 读目标文件当前内容,按目标后缀分派解析器(JSON/YAML/TOML)
	var targetJSON map[string]interface{}
	if util.FileExists(targetPath) {
		data, _ := os.ReadFile(targetPath)
		if len(data) > 0 {
			format := util.DetectConfigFormat(targetPath)
			if format != "" {
				targetJSON, _ = util.ParseConfigBytes(data, format)
			}
		}
	}
	if targetJSON == nil {
		targetJSON = map[string]interface{}{}
	}

	// 收集每个待部署资源的 config 片段(路径树语义需要完整结构,而非拍平 key 集)
	type resKeys struct {
		id   string
		name string
		cfg  map[string]interface{}
	}
	var pending []resKeys
	for _, rid := range req.ResourceIDs {
		r, err := s.resourceRepo.GetResourceByID(rid)
		if err != nil || r == nil {
			continue
		}
		pending = append(pending, resKeys{id: rid, name: r.Name, cfg: s.readConfigFragmentByID(rid)})
	}

	// 已有内容中被已部署 MCP 管理的顶层 key(用于把"原始内容"从目标里剥离)
	managedKeys := map[string]bool{}
	for _, p := range pending {
		for k, v := range s.getManagedKeys(targetPath, p.id) {
			if v {
				managedKeys[k] = true
			}
		}
	}

	var conflicts []model.ConflictItem
	seen := map[string]bool{}

	// 1. 检测待部署资源之间的冲突（Union-Find 分组）
	//    有 key 交集的归为一组，组内按顺序最后一个标 applied，其余标 ignored
	//    不在任何冲突组的资源标 applied
	n := len(pending)
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}

	// 合并有路径树冲突的资源(相同位置相同 key 且至少一方非 map)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if configsConflict(pending[i].cfg, pending[j].cfg) {
				union(i, j)
			}
		}
	}

	// 按 root 分组，找出多成员组（冲突组）
	groups := map[int][]int{}
	for i := 0; i < n; i++ {
		root := find(i)
		groups[root] = append(groups[root], i)
	}

	for _, members := range groups {
		if len(members) == 1 {
			// 无冲突的单独资源：标 applied
			idx := members[0]
			conflicts = append(conflicts, model.ConflictItem{
				ResourceID:   pending[idx].id,
				ResourceName: pending[idx].name,
				Status:       "applied",
				Group:        0,
			})
			seen[pending[idx].name] = true
		} else {
			// 冲突组：按原始顺序排序，最后一个 applied，其余 ignored
			// members 已按 groups append 顺序（即原始顺序）
			sort.Ints(members)
			lastIdx := members[len(members)-1]
			for _, idx := range members {
				status := "ignored"
				if idx == lastIdx {
					status = "applied"
				}
				if !seen[pending[idx].name] {
					seen[pending[idx].name] = true
					conflicts = append(conflicts, model.ConflictItem{
						ResourceID:   pending[idx].id,
						ResourceName: pending[idx].name,
						Status:       status,
						Group:        find(idx) + 1, // 非零表示属于冲突组
					})
				}
			}
		}
	}

	// 2. 检测与已部署 MCP 的冲突 —— 逐个待部署资源判定,精确归属到「本次的哪个资源」
	//    existing 冲突项: ResourceName=已部署的冲突对象, ConflictFor=本次待部署且触发冲突的资源名
	for _, p := range pending {
		if len(p.cfg) == 0 {
			continue
		}
		existingConflicts := s.findConfigConflicts(targetPath, "", p.cfg)
		for _, ec := range existingConflicts {
			// 跳过"冲突对象就是本次待部署资源自身"的情况
			isSelf := false
			for _, pp := range pending {
				if pp.id == ec.resourceID {
					isSelf = true
					break
				}
			}
			if isSelf {
				continue
			}
			key := "mcp__" + ec.name + "__" + p.id
			if seen[key] {
				continue
			}
			seen[key] = true
			conflicts = append(conflicts, model.ConflictItem{
				ResourceName: ec.name,
				ConflictFor:  p.name,
				Status:       "existing",
			})
		}
	}

	// 3. 检测与原始内容的冲突（目标中非已部署 MCP 管理的部分）—— 按路径树语义,逐资源归属
	originalOnly := map[string]interface{}{}
	for k, v := range targetJSON {
		if managedKeys[k] {
			continue
		}
		originalOnly[k] = v
	}
	for _, p := range pending {
		if len(p.cfg) == 0 {
			continue
		}
		if configsConflict(p.cfg, originalOnly) {
			key := "orig__" + p.id
			if !seen[key] {
				seen[key] = true
				conflicts = append(conflicts, model.ConflictItem{
					ResourceName: "原始内容",
					ConflictFor:  p.name,
					Status:       "existing",
				})
			}
		}
	}

	return &model.CheckConflictsResp{
		HasConflict: hasRealConflict(conflicts),
		Conflicts:   conflicts,
	}, nil
}

// hasRealConflict 判断 conflicts 是否包含真正需要用户决策的冲突项
// - ignored: 用户部署的多份资源有 key 重叠,被牺牲的那份
// - existing: 与目标文件已有内容或已部署 Config 资源冲突
// - applied:  "本次实际应用" 的展示项,不算冲突(单资源部署时永远会有一项)
func hasRealConflict(items []model.ConflictItem) bool {
	for _, c := range items {
		if c.Status == "ignored" || c.Status == "existing" {
			return true
		}
	}
	return false
}

// GetResourceDeployTargets 获取「包含当前资源」的全部部署(保存后同步用)
//
// 语义: 只返回真正部署了当前资源的 deployment(每条对应一个目标子路径),
// 不再返回该 preset 下其他无关类型的部署。前端按路径组分组展示:
// 路径组名作主标题,子行显示当前资源名 + 部署子路径。
func (s *DeployService) GetResourceDeployTargets(resourceID string) ([]model.ResourceDeployTarget, error) {
	// 查当前资源关联的全部部署明细(每条 item 指向一个 deployment)
	items, err := s.deployRepo.GetDeploymentItemsByResourceID(resourceID)
	if err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}

	// 资源名(子行展示用)
	resourceName := resourceID
	if r, rErr := s.resourceRepo.GetResourceByID(resourceID); rErr == nil && r != nil {
		resourceName = r.Name
	}

	// preset id -> name 缓存
	presetNameCache := map[string]string{}
	presetNameOf := func(pid string) string {
		if pid == "" {
			return ""
		}
		if n, ok := presetNameCache[pid]; ok {
			return n
		}
		n := ""
		if p, pErr := s.presetRepo.GetPresetByID(pid); pErr == nil && p != nil {
			n = p.Name
		}
		presetNameCache[pid] = n
		return n
	}

	var result []model.ResourceDeployTarget
	seenDep := map[string]bool{}

	for _, item := range items {
		if seenDep[item.DeploymentID] {
			continue
		}
		seenDep[item.DeploymentID] = true

		dep, gErr := s.deployRepo.GetDeploymentByID(item.DeploymentID)
		if gErr != nil || dep == nil {
			continue
		}

		presetID := ""
		if dep.PresetID != nil {
			presetID = *dep.PresetID
		}

		// 冲突检测: 仅当前资源(config 复用 CheckConflicts;prompt/symlink 恒无冲突)
		hasConflict := false
		if dep.DeployType == "merge" {
			r, _ := s.resourceRepo.GetResourceByID(resourceID)
			if r != nil && r.Type == "config" {
				if resp, cErr := s.CheckConflicts(&model.CheckConflictsReq{
					ResourceIDs: []string{resourceID},
					TargetPath:  dep.TargetPath,
				}); cErr == nil && resp != nil {
					hasConflict = resp.HasConflict
				}
			}
		}

		// 别名
		aliasName := ""
		if dep.AliasID != nil && *dep.AliasID != "" {
			if alias, aErr := s.aliasRepo.GetAliasByID(*dep.AliasID); aErr == nil && alias != nil {
				aliasName = alias.Name
			}
		}

		result = append(result, model.ResourceDeployTarget{
			PresetID:      presetID,
			PresetName:    presetNameOf(presetID),
			PathGroupName: s.pathGroupNameForTarget(dep.TargetPath),
			DeploymentID:  dep.ID,
			DeployType:    dep.DeployType,
			TargetPath:    dep.TargetPath,
			AliasName:     aliasName,
			ResourceIDs:   []string{resourceID},
			ResourceNames: []string{resourceName},
			HasConflict:   hasConflict,
		})
	}

	if result == nil {
		result = []model.ResourceDeployTarget{}
	}
	return result, nil
}

// pathGroupNameForTarget 把部署的 target_path 匹配回所属路径组名(任一子路径命中即归该组)
// 未注入 pathGroupRepo 或无匹配时返回空串
func (s *DeployService) pathGroupNameForTarget(targetPath string) string {
	if s.pathGroupRepo == nil {
		return ""
	}
	groups, err := s.pathGroupRepo.ListPathGroups()
	if err != nil {
		return ""
	}
	for _, g := range groups {
		if g.SkillPath == targetPath || g.AgentPath == targetPath ||
			g.PromptPath == targetPath {
			return g.Name
		}
		for _, cp := range g.ConfigPaths {
			if cp == targetPath {
				return g.Name
			}
		}
	}
	return ""
}

// checkConfigKeyConflict 检查目标文件中是否已存在该 key（顶层优先，其次扫描所有子映射）
// 通用配置语义: key 可以是顶层 key,也可以是某子映射(如 mcpServers)下的 key
func (s *DeployService) checkConfigKeyConflict(targetPath, key, _ string) bool {
	if !util.FileExists(targetPath) {
		return false
	}
	format := util.DetectConfigFormat(targetPath)
	if format == "" {
		return false
	}
	data, err := os.ReadFile(targetPath)
	if err != nil {
		return false
	}
	obj, err := util.ParseConfigBytes(data, format)
	if err != nil || obj == nil {
		return false
	}
	if _, exists := obj[key]; exists {
		return true
	}
	// 兼容历史 mcpServers 写法:扫所有子映射
	for _, v := range obj {
		if sub, ok := v.(map[string]interface{}); ok {
			if _, exists := sub[key]; exists {
				return true
			}
		}
	}
	return false
}

// checkPromptConflicts Prompt 批量部署预检冲突 (PRD §4.1)
//
// 算法:
//   - 每对 (resource, target) 独立判定,仅依赖 util.HasPromptMarkers
//   - status='existing' 当目标已含本资源的分隔符块
//   - status='applied'  否则
//   - group 始终 = 0 (UUID 唯一,资源间永不撞)
//
// 输入:
//   - ResourceIDs: 待部署的 prompt 资源 ID 列表
//   - TargetPaths: 多目标 (优先生效); 为空时回退到 TargetPath/AliasID 单目标
//
// 输出: 每个 (resource, target) 对一条 ConflictItem (含 TargetPath)
//
// has_conflict 在存在 status='existing' 项时为 true (与 config 一致语义)
func (s *DeployService) checkPromptConflicts(req *model.CheckConflictsReq) (*model.CheckConflictsResp, error) {
	// 1. 解析多目标路径列表
	targets, err := s.resolvePromptTargets(req)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return &model.CheckConflictsResp{HasConflict: false, Conflicts: []model.ConflictItem{}}, nil
	}

	// 2. 加载所有待部署 prompt 资源
	type resInfo struct {
		id   string
		name string
	}
	var resources []resInfo
	for _, rid := range req.ResourceIDs {
		r, rErr := s.resourceRepo.GetResourceByID(rid)
		if rErr != nil || r == nil {
			continue
		}
		resources = append(resources, resInfo{id: rid, name: r.Name})
	}

	// 3. 逐对判定: 读每个 target 文件全文,对每个 resource 调用 HasPromptMarkers
	conflicts := make([]model.ConflictItem, 0, len(resources)*len(targets))
	hasConflict := false
	for _, tp := range targets {
		var targetText string
		if util.FileExists(tp) {
			data, _ := os.ReadFile(tp)
			targetText = string(data)
		}
		for _, r := range resources {
			status := "applied"
			if util.HasPromptMarkers(targetText, r.id) {
				status = "existing"
				hasConflict = true
			}
			conflicts = append(conflicts, model.ConflictItem{
				ResourceID:   r.id,
				ResourceName: r.name,
				TargetPath:   tp,
				Status:       status,
				Group:        0,
			})
		}
	}

	return &model.CheckConflictsResp{
		HasConflict: hasConflict,
		Conflicts:   conflicts,
	}, nil
}

// resolvePromptTargets 解析 Prompt 预检的多目标路径列表
//
// 优先级 (互斥):
//  1. req.TargetPaths 非空 → 逐个 ExpandPath/Clean 后返回
//  2. req.AliasID 非空    → 查 alias.Path,展开返回单元素列表
//  3. req.TargetPath 非空 → ExpandPath/Clean 后返回单元素列表
//  4. 全空                → 空列表 (调用方判空)
func (s *DeployService) resolvePromptTargets(req *model.CheckConflictsReq) ([]string, error) {
	if len(req.TargetPaths) > 0 {
		out := make([]string, 0, len(req.TargetPaths))
		for _, tp := range req.TargetPaths {
			if tp == "" {
				continue
			}
			expanded, err := util.ExpandPath(tp)
			if err != nil {
				return nil, model.NewBizError(model.ErrDeployInvalid,
					fmt.Sprintf("展开目标路径失败: %v", err))
			}
			out = append(out, filepath.Clean(expanded))
		}
		return out, nil
	}

	if req.AliasID != nil && *req.AliasID != "" {
		alias, err := s.aliasRepo.GetAliasByID(*req.AliasID)
		if err != nil || alias == nil {
			return nil, model.NewBizError(model.ErrAliasNotFound, "路径别名不存在")
		}
		expanded, _ := util.ExpandPath(alias.Path)
		return []string{filepath.Clean(expanded)}, nil
	}

	if req.TargetPath != "" {
		expanded, _ := util.ExpandPath(req.TargetPath)
		return []string{filepath.Clean(expanded)}, nil
	}

	return nil, nil
}

// === Preset 部署相关 ===

// presetTypeOrder Preset 内资源类型固定按此顺序部署（symlink 类在前，merge 类在后）
var presetTypeOrder = []string{"skill", "agent", "config", "prompt"}

// typeToPathSpec 取某类型对应的子路径
func typeToPathSpec(t string, spec *model.PathSpec) string {
	if spec == nil {
		return ""
	}
	switch t {
	case "skill":
		return spec.SkillPath
	case "agent":
		return spec.AgentPath
	case "config":
		// config 已改为多路径，此处返回第一条仅作兜底（正常不应走到）
		if len(spec.ConfigPaths) > 0 {
			return spec.ConfigPaths[0]
		}
		return spec.ConfigPath
	case "prompt":
		return spec.PromptPath
	}
	return ""
}

// resolveConfigAssignments 计算每个 config 资源应部署到的目标路径。
//
// 规则:
//   - configPaths 为空 → 无 config 可部署，返回空 map（调用方在校验阶段已拦截"有 config 资源却无路径"）
//   - configPaths 恰 1 条 → 所有 config 自动归到该路径（无需前端分配）
//   - configPaths 多条 → 每个 config 必须在 assignments 中指定，且目标 ∈ configPaths
func (s *DeployService) resolveConfigAssignments(configs []model.Resource, configPaths []string, assignments map[string]string) (map[string]string, error) {
	result := map[string]string{}
	if len(configs) == 0 {
		return result, nil
	}
	if len(configPaths) == 0 {
		return nil, model.NewBizError(model.ErrDeployInvalid, "preset 含 config 资源,但目标无 config 路径")
	}
	if len(configPaths) == 1 {
		for _, r := range configs {
			result[r.ID] = configPaths[0]
		}
		return result, nil
	}
	// 多路径：必须逐个分配且合法
	valid := map[string]bool{}
	for _, p := range configPaths {
		valid[p] = true
	}
	var unassigned []string
	for _, r := range configs {
		target, ok := assignments[r.ID]
		if !ok || target == "" {
			unassigned = append(unassigned, r.Name)
			continue
		}
		if !valid[target] {
			return nil, model.NewBizError(model.ErrDeployInvalid,
				fmt.Sprintf("config「%s」分配的路径不在该路径组的 config 路径列表中", r.Name))
		}
		result[r.ID] = target
	}
	if len(unassigned) > 0 {
		return nil, model.NewBizError(model.ErrDeployInvalid,
			fmt.Sprintf("以下 config 未分配目标路径: %s", strings.Join(unassigned, "、")))
	}
	return result, nil
}

// deployConfigByAssignment 把 config 资源按分配的目标路径分组，每个目标一条 deployment
func (s *DeployService) deployConfigByAssignment(configs []model.Resource, assign map[string]string, track bool, presetID *string) ([]model.Deployment, error) {
	// 按目标路径分组，保持稳定顺序
	groups := map[string][]string{}
	var order []string
	for _, r := range configs {
		target := assign[r.ID]
		if target == "" {
			continue
		}
		if _, ok := groups[target]; !ok {
			order = append(order, target)
		}
		groups[target] = append(groups[target], r.ID)
	}
	var created []model.Deployment
	for _, target := range order {
		dep, err := s.Deploy(&model.DeployRequest{
			ResourceIDs: groups[target],
			TargetPath:  target,
			Force:       true,
			Track:       track,
			PresetID:    presetID,
		})
		if err != nil {
			return created, err
		}
		created = append(created, *dep)
	}
	return created, nil
}

// loadPresetResources 加载 preset 的全部资源（关联 + 私有），按 type 分组
func (s *DeployService) loadPresetResources(presetID string) (map[string][]model.Resource, error) {
	if s.presetRepo == nil {
		return nil, model.NewBizError(model.ErrSystemInternal, "presetRepo 未注入")
	}
	byType := map[string][]model.Resource{}

	// 关联资源
	linkedIDs, err := s.presetRepo.ListPresetResources(presetID)
	if err != nil {
		return nil, err
	}
	// 私有资源
	privateIDs, err := s.presetRepo.ListPrivateResourceIDs(presetID)
	if err != nil {
		return nil, err
	}
	all := append(append([]string{}, linkedIDs...), privateIDs...)
	for _, rid := range all {
		r, err := s.resourceRepo.GetResourceByID(rid)
		if err != nil || r == nil {
			continue
		}
		byType[r.Type] = append(byType[r.Type], *r)
	}
	return byType, nil
}

// DeployPreset 部署整个 preset 到一个路径组 / 手动路径组
//
// 流程:
//  1. 加载 preset 全部资源,按 type 分组
//  2. 解析目标路径组 (path_group_id 或 manual_paths)
//  3. 校验 preset 包含的每个类型都有对应的子路径
//  4. 对每个类型生成一条 deployment (preset_id 填充, target_path 填对应子路径),复用底层 Deploy
func (s *DeployService) DeployPreset(req *model.DeployPresetReq, presetID string) ([]model.Deployment, error) {
	presetIDPtr := &presetID

	// 1. 加载资源
	byType, err := s.loadPresetResources(presetID)
	if err != nil {
		return nil, err
	}
	if len(byType) == 0 {
		return nil, model.NewBizError(model.ErrDeployInvalid, "preset 没有任何资源,无法部署")
	}

	// 2. 解析路径组
	var spec model.PathSpec
	if req.PathGroupID != nil && *req.PathGroupID != "" {
		if s.presetRepo == nil {
			return nil, model.NewBizError(model.ErrSystemInternal, "presetRepo 未注入")
		}
		// 通过 repo.db 直接查 path_group（避免再多注入一个 repo）
		pg, pErr := s.presetRepo.GetPathGroupByIDByID(*req.PathGroupID)
		if pErr != nil {
			return nil, model.NewBizError(model.ErrSystemDB, pErr.Error())
		}
		if pg == nil {
			return nil, model.NewBizError(model.ErrPathGroupNotFound, "路径组不存在")
		}
		spec = model.PathSpec{
			SkillPath:   pg.SkillPath,
			AgentPath:   pg.AgentPath,
			ConfigPaths: pg.ConfigPaths,
			PromptPath:  pg.PromptPath,
		}
	} else if req.ManualPaths != nil {
		spec = *req.ManualPaths
		// 手动模式 config 多路径归一：ConfigPaths 优先，否则回退单值 ConfigPath
		if len(spec.ConfigPaths) == 0 && spec.ConfigPath != "" {
			spec.ConfigPaths = []string{spec.ConfigPath}
		}
	} else {
		return nil, model.NewBizError(model.ErrDeployInvalid, "path_group_id 和 manual_paths 不能同时为空")
	}

	// 3. 校验：preset 包含的每个类型都有对应子路径
	var missing []string
	for _, t := range presetTypeOrder {
		if _, ok := byType[t]; !ok {
			continue
		}
		if t == "config" {
			if len(spec.ConfigPaths) == 0 {
				missing = append(missing, t)
			}
			continue
		}
		if typeToPathSpec(t, &spec) == "" {
			missing = append(missing, t)
		}
	}
	if len(missing) > 0 {
		return nil, model.NewBizError(model.ErrDeployInvalid,
			fmt.Sprintf("缺少类型对应的子路径: %s", strings.Join(missing, ",")))
	}

	// 3.5 解析 config 资源 → 目标路径的分配
	//   - 单条 config 路径：全部 config 归到那一条（无需前端分配）
	//   - 多条 config 路径：必须每个 config 在 ConfigAssignments 指定且目标合法
	configAssign, aErr := s.resolveConfigAssignments(byType["config"], spec.ConfigPaths, req.ConfigAssignments)
	if aErr != nil {
		return nil, aErr
	}

	// 4. 对每个类型部署
	var created []model.Deployment
	for _, t := range presetTypeOrder {
		resources, ok := byType[t]
		if !ok || len(resources) == 0 {
			continue
		}
		if t == "config" {
			// config 按目标路径分组，每组一条 deployment
			deps, cErr := s.deployConfigByAssignment(resources, configAssign, req.Track, presetIDPtr)
			if cErr != nil {
				return created, cErr
			}
			created = append(created, deps...)
			continue
		}
		ids := make([]string, 0, len(resources))
		for _, r := range resources {
			ids = append(ids, r.ID)
		}
		deployReq := &model.DeployRequest{
			ResourceIDs: ids,
			TargetPath:  typeToPathSpec(t, &spec),
			Force:       true, // preset 部署默认覆盖同目标
			Track:       req.Track,
			PresetID:    presetIDPtr,
		}
		dep, dErr := s.Deploy(deployReq)
		if dErr != nil {
			return created, dErr
		}
		created = append(created, *dep)
	}

	s.broadcastEvent("preset:deployed", map[string]interface{}{"preset_id": presetID})
	return created, nil
}

// RedeployPreset 重新部署整个 preset — 复用已有的 target_path，强制覆盖
//
// 流程:
//  1. 查询该 preset 所有现有 deployment
//  2. 对每个 deployment，记录其 target_path 和 track，然后撤销
//  3. 按 target_path 重新部署（保留原 track 设置）
func (s *DeployService) RedeployPreset(presetID string) ([]model.Deployment, error) {
	presetIDPtr := &presetID

	// 1. 查询现有部署
	existing, err := s.deployRepo.ListDeploymentsByPreset(presetID)
	if err != nil {
		return nil, err
	}
	if len(existing) == 0 {
		return nil, model.NewBizError(model.ErrDeployInvalid, "该 preset 没有已部署记录,无法重新部署")
	}

	// 2. 加载当前资源
	byType, err := s.loadPresetResources(presetID)
	if err != nil {
		return nil, err
	}
	if len(byType) == 0 {
		return nil, model.NewBizError(model.ErrDeployInvalid, "preset 没有任何资源,无法部署")
	}

	// 3. 记录每个 target_path 对应的 track 状态，然后撤销所有旧部署
	type targetInfo struct {
		track bool
	}
	targets := map[string]targetInfo{}
	for _, dep := range existing {
		targets[dep.TargetPath] = targetInfo{track: dep.Track == 1}
		_ = s.Undeploy(dep.ID)
	}

	// 4. 对每个 target_path，找出匹配的资源类型重新部署
	var created []model.Deployment
	for targetPath, info := range targets {
		for _, t := range presetTypeOrder {
			resources, ok := byType[t]
			if !ok || len(resources) == 0 {
				continue
			}
			if !s.resourceTypeMatchesTarget(t, targetPath) {
				continue
			}
			ids := make([]string, 0, len(resources))
			for _, r := range resources {
				ids = append(ids, r.ID)
			}
			deployReq := &model.DeployRequest{
				ResourceIDs: ids,
				TargetPath:  targetPath,
				Force:       true,
				Track:       info.track,
				PresetID:    presetIDPtr,
			}
			dep, dErr := s.Deploy(deployReq)
			if dErr != nil {
				continue
			}
			created = append(created, *dep)
		}
	}

	s.broadcastEvent("preset:deployed", map[string]interface{}{"preset_id": presetID})
	return created, nil
}

// UndeployPresetDeployment 撤销某次 preset 部署（按 deployment ID）
func (s *DeployService) UndeployPresetDeployment(presetID, deploymentID string) error {
	dep, err := s.deployRepo.GetDeploymentByID(deploymentID)
	if err != nil {
		return model.NewBizError(model.ErrSystemDB, err.Error())
	}
	if dep == nil {
		return model.NewBizError(model.ErrDeployNotFound, "部署不存在")
	}
	if dep.PresetID == nil || *dep.PresetID != presetID {
		return model.NewBizError(model.ErrDeployNotFound, "该部署不属于此 preset")
	}
	if err := s.Undeploy(deploymentID); err != nil {
		return err
	}
	s.broadcastEvent("preset:undeployed", map[string]interface{}{
		"preset_id":     presetID,
		"deployment_id": deploymentID,
	})
	return nil
}

// ListPresetDeployments 查询某 preset 下所有部署记录
func (s *DeployService) ListPresetDeployments(presetID string) ([]model.Deployment, error) {
	deployments, err := s.deployRepo.ListDeploymentsByPreset(presetID)
	if err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}
	return deployments, nil
}

// groupOfTarget 返回 target_path 命中的路径组（任一子路径相等即命中），未命中返回 nil
func (s *DeployService) groupOfTarget(groups []model.PathGroup, targetPath string) *model.PathGroup {
	for i := range groups {
		g := &groups[i]
		if g.SkillPath == targetPath || g.AgentPath == targetPath ||
			g.PromptPath == targetPath {
			return g
		}
		for _, cp := range g.ConfigPaths {
			if cp == targetPath {
				return g
			}
		}
	}
	return nil
}

// typesWithSubPath 返回该路径组配置了子路径的资源类型集合
func typesWithSubPath(g *model.PathGroup) map[string]bool {
	set := map[string]bool{}
	if g.SkillPath != "" {
		set["skill"] = true
	}
	if g.AgentPath != "" {
		set["agent"] = true
	}
	if len(g.ConfigPaths) > 0 {
		set["config"] = true
	}
	if g.PromptPath != "" {
		set["prompt"] = true
	}
	return set
}

// ListPresetGroupDrifts 计算 preset 在每个"已部署路径组"下的漂移汇总。
//
// 关键点：漂移单位是 (preset, 路径组)，而非单条 deployment。
// preset 新增了某类型资源（如 prompt），即使该路径组此前没有 prompt 部署记录，
// 只要路径组配了 prompt 子路径，就算 pending —— 这正是逐 deployment 检测漏掉的场景。
func (s *DeployService) ListPresetGroupDrifts(presetID string) (map[string]model.PresetGroupDrift, error) {
	deployments, err := s.deployRepo.ListDeploymentsByPreset(presetID)
	if err != nil {
		return nil, err
	}
	result := map[string]model.PresetGroupDrift{}
	if len(deployments) == 0 {
		return result, nil
	}
	groups, err := s.pathGroupRepo.ListPathGroups()
	if err != nil {
		return nil, err
	}
	byType, err := s.loadPresetResources(presetID)
	if err != nil {
		return nil, err
	}

	// 找出该 preset 已部署到的路径组（去重）
	involved := map[string]*model.PathGroup{}
	for i := range deployments {
		if g := s.groupOfTarget(groups, deployments[i].TargetPath); g != nil {
			involved[g.ID] = g
		}
	}

	for gid, g := range involved {
		// 该路径组下、按类型聚合的已部署资源集
		deployedByType := map[string]map[string]bool{}
		for i := range deployments {
			dep := &deployments[i]
			tg := s.groupOfTarget(groups, dep.TargetPath)
			if tg == nil || tg.ID != gid {
				continue
			}
			items, _ := s.deployRepo.GetDeploymentItems(dep.ID)
			for _, it := range items {
				r, rErr := s.resourceRepo.GetResourceByID(it.ResourceID)
				rtype := ""
				if rErr == nil && r != nil {
					rtype = r.Type
				}
				if deployedByType[rtype] == nil {
					deployedByType[rtype] = map[string]bool{}
				}
				deployedByType[rtype][it.ResourceID] = true
			}
		}

		subTypes := typesWithSubPath(g)
		pending, stale := 0, 0
		var missingTypes []string
		// 遍历 preset 实际拥有资源的每种类型：
		//   - 路径组配了该类型子路径 → 正常比对 pending/stale
		//   - 路径组【未配】该类型子路径 → 该类型资源无处部署，记为 missing，
		//     并把这些资源计入 pending（驱动侧栏标红：「未配置」需先补路径）
		for _, rtype := range presetTypeOrder {
			expected := map[string]bool{}
			for _, r := range byType[rtype] {
				expected[r.ID] = true
			}
			if len(expected) == 0 {
				continue
			}
			if !subTypes[rtype] {
				missingTypes = append(missingTypes, rtype)
				pending += len(expected)
				continue
			}
			deployed := deployedByType[rtype]
			for rid := range expected {
				if !deployed[rid] {
					pending++
				}
			}
		}
		// 残留：该路径组已部署但已不在 preset 的资源（仅在配了子路径的类型上统计）
		for rtype := range subTypes {
			deployed := deployedByType[rtype]
			expected := map[string]bool{}
			for _, r := range byType[rtype] {
				expected[r.ID] = true
			}
			for rid := range deployed {
				if !expected[rid] {
					stale++
				}
			}
		}
		result[gid] = model.PresetGroupDrift{
			GroupID:      gid,
			GroupName:    g.Name,
			Pending:      pending,
			Stale:        stale,
			MissingTypes: missingTypes,
		}
	}
	return result, nil
}

// GetPresetGroupStatus 返回 preset 在指定路径组下的完整部署状态（部署管理弹窗用）：
// 每个"该路径组配了子路径的类型"一行 target，列出应部署资源 + 已部署/新增/残留状态。
func (s *DeployService) GetPresetGroupStatus(presetID, groupID string) (*model.PresetGroupStatus, error) {
	groups, err := s.pathGroupRepo.ListPathGroups()
	if err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}
	var g *model.PathGroup
	for i := range groups {
		if groups[i].ID == groupID {
			g = &groups[i]
			break
		}
	}
	if g == nil {
		return nil, model.NewBizError(model.ErrPathGroupNotFound, "路径组不存在")
	}

	deployments, err := s.deployRepo.ListDeploymentsByPreset(presetID)
	if err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}
	byType, err := s.loadPresetResources(presetID)
	if err != nil {
		return nil, err
	}

	// 该路径组下、每种类型对应的 deployment（若已部署）。
	// config 可有多条路径，按 target_path 索引；其余类型按类型索引（单路径）。
	depBySingleType := map[string]*model.Deployment{}
	depByConfigPath := map[string]*model.Deployment{}
	for i := range deployments {
		dep := &deployments[i]
		tg := s.groupOfTarget(groups, dep.TargetPath)
		if tg == nil || tg.ID != groupID {
			continue
		}
		switch dep.TargetPath {
		case g.SkillPath:
			depBySingleType["skill"] = dep
		case g.AgentPath:
			depBySingleType["agent"] = dep
		case g.PromptPath:
			depBySingleType["prompt"] = dep
		default:
			// 命中该组的某条 config 路径
			for _, cp := range g.ConfigPaths {
				if cp == dep.TargetPath {
					depByConfigPath[cp] = dep
				}
			}
		}
	}

	status := &model.PresetGroupStatus{GroupID: g.ID, GroupName: g.Name}
	subTypes := typesWithSubPath(g)

	// buildSingleTargetStatus 处理 skill/agent/prompt 单路径类型
	buildSingleTargetStatus := func(rtype, targetPath string, dep *model.Deployment) model.PresetTargetStatus {
		ts := model.PresetTargetStatus{Type: rtype, TargetPath: targetPath}
		deployedSet := map[string]bool{}
		if dep != nil {
			ts.DeploymentID = dep.ID
			ts.Track = dep.Track
			ts.DeployType = dep.DeployType
			ts.HasDeployment = true
			items, _ := s.deployRepo.GetDeploymentItems(dep.ID)
			for _, it := range items {
				deployedSet[it.ResourceID] = true
			}
		}
		expected := map[string]model.Resource{}
		for _, r := range byType[rtype] {
			expected[r.ID] = r
		}
		for _, r := range byType[rtype] {
			deployed := deployedSet[r.ID]
			rs := model.DeployResourceStatus{
				ResourceID: r.ID, ResourceName: r.Name, Type: r.Type, Deployed: deployed,
			}
			if deployed {
				rs.CurrentPath = targetPath
			}
			ts.Resources = append(ts.Resources, rs)
			if !deployed {
				status.Pending++
			}
		}
		for rid := range deployedSet {
			if _, ok := expected[rid]; ok {
				continue
			}
			name, rtp := rid, rtype
			if r, rErr := s.resourceRepo.GetResourceByID(rid); rErr == nil && r != nil {
				name, rtp = r.Name, r.Type
			}
			ts.Resources = append(ts.Resources, model.DeployResourceStatus{
				ResourceID: rid, ResourceName: name, Type: rtp, Deployed: true, Stale: true, CurrentPath: targetPath,
			})
			status.Stale++
		}
		if ts.Resources == nil {
			ts.Resources = []model.DeployResourceStatus{}
		}
		return ts
	}

	for _, rtype := range []string{"skill", "agent", "prompt"} {
		if !subTypes[rtype] {
			continue
		}
		tp := typeToPathSpec(rtype, &model.PathSpec{
			SkillPath: g.SkillPath, AgentPath: g.AgentPath, PromptPath: g.PromptPath,
		})
		status.Targets = append(status.Targets, buildSingleTargetStatus(rtype, tp, depBySingleType[rtype]))
	}

	// config 多路径：每条路径一行；另把 preset 中尚未部署到本组任一路径的 config 归入"未分配"行
	if subTypes["config"] {
		assignedConfigIDs := map[string]bool{}
		for _, cp := range g.ConfigPaths {
			dep := depByConfigPath[cp]
			ts := model.PresetTargetStatus{Type: "config", TargetPath: cp}
			deployedSet := map[string]bool{}
			if dep != nil {
				ts.DeploymentID = dep.ID
				ts.Track = dep.Track
				ts.DeployType = dep.DeployType
				ts.HasDeployment = true
				items, _ := s.deployRepo.GetDeploymentItems(dep.ID)
				for _, it := range items {
					deployedSet[it.ResourceID] = true
				}
			}
			presetConfigIDs := map[string]bool{}
			for _, r := range byType["config"] {
				presetConfigIDs[r.ID] = true
			}
			// 已部署在该路径、且仍属 preset 的 config
			for _, r := range byType["config"] {
				if !deployedSet[r.ID] {
					continue
				}
				assignedConfigIDs[r.ID] = true
				ts.Resources = append(ts.Resources, model.DeployResourceStatus{
					ResourceID: r.ID, ResourceName: r.Name, Type: "config", Deployed: true, CurrentPath: cp,
				})
			}
			// 残留：部署在该路径但已不在 preset
			for rid := range deployedSet {
				if presetConfigIDs[rid] {
					continue
				}
				name := rid
				if r, rErr := s.resourceRepo.GetResourceByID(rid); rErr == nil && r != nil {
					name = r.Name
				}
				ts.Resources = append(ts.Resources, model.DeployResourceStatus{
					ResourceID: rid, ResourceName: name, Type: "config", Deployed: true, Stale: true, CurrentPath: cp,
				})
				status.Stale++
			}
			if ts.Resources == nil {
				ts.Resources = []model.DeployResourceStatus{}
			}
			status.Targets = append(status.Targets, ts)
		}
		// 未分配：preset 有但尚未部署到本组任一 config 路径
		var unassigned []model.DeployResourceStatus
		for _, r := range byType["config"] {
			if assignedConfigIDs[r.ID] {
				continue
			}
			unassigned = append(unassigned, model.DeployResourceStatus{
				ResourceID: r.ID, ResourceName: r.Name, Type: "config", Deployed: false,
			})
			status.Pending++
		}
		if len(unassigned) > 0 {
			status.Targets = append(status.Targets, model.PresetTargetStatus{
				Type: "config", TargetPath: "", Resources: unassigned,
			})
		}
	}

	if status.Targets == nil {
		status.Targets = []model.PresetTargetStatus{}
	}
	return status, nil
}

// RedeployPresetGroup 将 preset 以最新全量资源重新部署到指定路径组：
// 撤销该组下旧部署，再按"路径组配了子路径 且 preset 有该类型资源"的组合全部重建。
// 这能补齐部署后新增的类型（如新加的 prompt）。
//
// configAssignments: config 资源 ID → 目标路径的分配（前端弹窗回传）。
// 撤销前先快照每个 config 当前所在路径作为默认值，assignments 覆盖之（支持重新分配 / 新增补选）。
func (s *DeployService) RedeployPresetGroup(presetID, groupID string, configAssignments map[string]string) ([]model.Deployment, error) {
	presetIDPtr := &presetID
	groups, err := s.pathGroupRepo.ListPathGroups()
	if err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}
	var g *model.PathGroup
	for i := range groups {
		if groups[i].ID == groupID {
			g = &groups[i]
			break
		}
	}
	if g == nil {
		return nil, model.NewBizError(model.ErrPathGroupNotFound, "路径组不存在")
	}

	deployments, err := s.deployRepo.ListDeploymentsByPreset(presetID)
	if err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}
	byType, err := s.loadPresetResources(presetID)
	if err != nil {
		return nil, err
	}

	// 撤销前：快照每个 config 资源当前部署到本组的哪条路径（作为重部署默认分配）
	configCurrentPath := map[string]string{}
	for i := range deployments {
		dep := &deployments[i]
		tg := s.groupOfTarget(groups, dep.TargetPath)
		if tg == nil || tg.ID != groupID {
			continue
		}
		isConfigPath := false
		for _, cp := range g.ConfigPaths {
			if cp == dep.TargetPath {
				isConfigPath = true
				break
			}
		}
		if !isConfigPath {
			continue
		}
		items, _ := s.deployRepo.GetDeploymentItems(dep.ID)
		for _, it := range items {
			configCurrentPath[it.ResourceID] = dep.TargetPath
		}
	}
	// 应用前端覆盖（重新分配 / 新增补选）
	for rid, target := range configAssignments {
		if target != "" {
			configCurrentPath[rid] = target
		}
	}

	// 记录原 track（按类型），并撤销该组下所有旧部署
	trackByType := map[string]bool{}
	configTrack := false
	for i := range deployments {
		dep := &deployments[i]
		tg := s.groupOfTarget(groups, dep.TargetPath)
		if tg == nil || tg.ID != groupID {
			continue
		}
		switch dep.TargetPath {
		case g.SkillPath:
			trackByType["skill"] = dep.Track == 1
		case g.AgentPath:
			trackByType["agent"] = dep.Track == 1
		case g.PromptPath:
			trackByType["prompt"] = dep.Track == 1
		default:
			// 任一 config 路径的 track（多条取其一即可，行为一致）
			for _, cp := range g.ConfigPaths {
				if cp == dep.TargetPath {
					configTrack = dep.Track == 1
				}
			}
		}
		_ = s.Undeploy(dep.ID)
	}

	spec := model.PathSpec{
		SkillPath: g.SkillPath, AgentPath: g.AgentPath, PromptPath: g.PromptPath,
	}
	subTypes := typesWithSubPath(g)
	var created []model.Deployment
	for _, t := range presetTypeOrder {
		if !subTypes[t] {
			continue
		}
		resources := byType[t]
		if len(resources) == 0 {
			continue
		}
		if t == "config" {
			// config 按（默认快照 + 前端覆盖）的分配重新部署，每路径一条
			deps, cErr := s.deployConfigByAssignment(resources, configCurrentPath, configTrack, presetIDPtr)
			if cErr != nil {
				// 单类型失败不阻断其余类型
				continue
			}
			created = append(created, deps...)
			continue
		}
		ids := make([]string, 0, len(resources))
		for _, r := range resources {
			ids = append(ids, r.ID)
		}
		dep, dErr := s.Deploy(&model.DeployRequest{
			ResourceIDs: ids,
			TargetPath:  typeToPathSpec(t, &spec),
			Force:       true,
			Track:       trackByType[t], // 沿用原 track；新类型默认 false(静态)
			PresetID:    presetIDPtr,
		})
		if dErr != nil {
			continue
		}
		created = append(created, *dep)
	}

	s.broadcastEvent("preset:deployed", map[string]interface{}{"preset_id": presetID})
	return created, nil
}

// UndeployAllPreset 撤销 preset 下所有部署（删除 preset 前调用）
func (s *DeployService) UndeployAllPreset(presetID string) error {
	deployments, err := s.deployRepo.ListDeploymentsByPreset(presetID)
	if err != nil {
		return model.NewBizError(model.ErrSystemDB, err.Error())
	}
	for _, dep := range deployments {
		if err := s.Undeploy(dep.ID); err != nil {
			// 继续尝试撤销其余
			continue
		}
	}
	return nil
}

// SyncPresetDeployments preset 资源变更后,对每个 track=1 的部署重新同步目标内容
//
// 实现:
//  1. 撤销每个 preset 部署的"过时"链接（资源已不在 preset 中）
//  2. 补齐"新增"资源的链接
// 仅对 track=1 的 deployment 生效；static 部署不动
func (s *DeployService) SyncPresetDeployments(presetID string) {
	deployments, err := s.deployRepo.ListDeploymentsByPreset(presetID)
	if err != nil {
		return
	}
	byType, _ := s.loadPresetResources(presetID)
	currentIDs := map[string]bool{}
	for _, rs := range byType {
		for _, r := range rs {
			currentIDs[r.ID] = true
		}
	}

	for _, dep := range deployments {
		// 静态部署不跟踪同步
		if dep.Track != 1 {
			continue
		}
		// 取该 deployment 当前所有 item
		items, err := s.deployRepo.GetDeploymentItems(dep.ID)
		if err != nil {
			continue
		}
		// 撤销已不在 preset 的 item
		for _, item := range items {
			if !currentIDs[item.ResourceID] {
				s.UndeployResourceFromTarget(item.ResourceID, dep.ID, dep.TargetPath, dep.DeployType)
			}
		}
		// 补齐新增资源
		for _, rs := range byType {
			for _, r := range rs {
				// 判断该资源类型与该 deployment 目标类型一致
				if !s.resourceTypeMatchesTarget(r.Type, dep.TargetPath) {
					continue
				}
				s.DeploySingleResourceToTarget(dep.ID, &r, dep.TargetPath)
			}
		}
	}

	s.broadcastEvent("preset:resource_changed", map[string]interface{}{"preset_id": presetID})
}

// resourceTypeMatchesTarget 粗略判断资源类型与 deployment 目标路径是否匹配
// skill/agent -> 目标是目录(无后缀)；config -> 配置文件后缀；prompt -> .md
func (s *DeployService) resourceTypeMatchesTarget(rtype, target string) bool {
	switch rtype {
	case "skill", "agent":
		return filepath.Ext(target) == ""
	case "config":
		return util.IsConfigFile(target)
	case "prompt":
		return util.IsPromptFile(target)
	}
	return false
}
