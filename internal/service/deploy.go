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
	//    - skill/agent: 目标路径是目录，不存在则创建
	isConfig := resources[0].Type == "config"
	if isConfig {
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
	} else {
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
			s.removeConfigResourceKeys(deployment.TargetPath, item.ResourceID)
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
				s.removeConfigResourceKeys(dep.TargetPath, item.ResourceID)
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
			s.removeConfigResourceKeys(targetPath, item.ResourceID)
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

	source := filepath.Join(s.baseDir, "skills", resource.ID)
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

	source := filepath.Join(s.baseDir, "agents", resource.ID+".md")
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
	if len(cfgFragment) == 0 {
		return "", model.NewBizError(model.ErrDeployFailed, "Config 片段为空")
	}

	// 3. 收集本次会写入目标的所有顶层 key(用于冲突检测)
	newKeys := flatKeys(cfgFragment)

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

	// 6. 检测冲突
	conflictResources := s.findConfigConflicts(targetPath, resource.ID, newKeys)
	managedKeys := s.getManagedKeys(targetPath, resource.ID)
	var originalConflictKeys []string
	for k := range newKeys {
		if managedKeys[k] {
			continue
		}
		if _, exists := targetKeys[k]; exists {
			originalConflictKeys = append(originalConflictKeys, k)
		}
	}
	hasConflict := len(conflictResources) > 0 || len(originalConflictKeys) > 0

	if hasConflict && !force {
		names := make([]string, 0)
		for _, cr := range conflictResources {
			names = append(names, cr.name)
		}
		if len(originalConflictKeys) > 0 {
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

	// 8. 实际合并写入(由 format 决定 JSON/YAML/TOML 分支)
	if err := util.MergeConfigToFile(targetPath, format, cfgFragment); err != nil {
		return "", model.NewBizError(model.ErrDeployFailed, fmt.Sprintf("合并写入失败: %v", err))
	}

	return linkPath, nil
}

// flatKeys 把嵌套 map 的所有 key 拍平(顶层 + 一层子键,如 mcpServers 的子键)
// 用于冲突检测: 一次部署实际"占用了"哪些 key,撤销时要精准删除
func flatKeys(m map[string]interface{}) map[string]bool {
	out := map[string]bool{}
	for k, v := range m {
		out[k] = true
		if sub, ok := v.(map[string]interface{}); ok {
			for sk := range sub {
				out[sk] = true
			}
		}
	}
	return out
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

// findConfigConflicts 查找目标路径下已部署的 Config 中与新 key 集合有冲突的资源
func (s *DeployService) findConfigConflicts(targetPath, selfResourceID string, newKeys map[string]bool) []configConflictInfo {
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
			if seen[item.ResourceID] {
				continue
			}
			existingKeys := s.getConfigResourceKeys(item.ResourceID)
			hasOverlap := false
			for k := range existingKeys {
				if newKeys[k] {
					hasOverlap = true
					break
				}
			}
			if hasOverlap {
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
	keysToRemove := flatKeys(cfg)
	if len(keysToRemove) == 0 {
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
		deleteNestedKeys(obj, keysToRemove)
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
			deleteNestedKeysFromYAML(rootMap, keysToRemove)
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
		deleteNestedKeys(obj, keysToRemove)
		out, err := toml.Marshal(obj)
		if err != nil {
			return err
		}
		return os.WriteFile(targetPath, out, 0644)
	}
	return nil
}

// deleteNestedKeys 从 obj 中删除 keysToRemove 中的 key(顶层 + 一层子键)
// 顶层命中直接删除;否则尝试从 mcpServers 等子映射中删除(兼容历史 MCP 数据)
func deleteNestedKeys(obj map[string]interface{}, keysToRemove map[string]bool) {
	if obj == nil {
		return
	}
	for k := range keysToRemove {
		if _, ok := obj[k]; ok {
			delete(obj, k)
			continue
		}
		// 兼容历史:从所有子映射中删除
		for _, v := range obj {
			if sub, ok := v.(map[string]interface{}); ok {
				delete(sub, k)
			}
		}
	}
}

// deleteNestedKeysFromYAML 从 YAML mapping 节点中删除 key(顶层 + 一层子键)
func deleteNestedKeysFromYAML(m *yamlv3.Node, keysToRemove map[string]bool) {
	if m.Kind != yamlv3.MappingNode {
		return
	}
	// 先收集要删除的 key 在 Content 中的下标(从大到小删除,避免下标位移)
	type kv struct{ keyIdx, valIdx int }
	var toDelete []kv
	for i := 0; i < len(m.Content); i += 2 {
		k := m.Content[i].Value
		if keysToRemove[k] {
			toDelete = append([]kv{{i, i + 1}}, toDelete...) // 倒序
		}
	}
	for _, d := range toDelete {
		m.Content = append(m.Content[:d.keyIdx], m.Content[d.valIdx+1:]...)
	}
	// 兼容历史:扫一遍剩余的子 mapping
	for i := 0; i < len(m.Content); i += 2 {
		val := m.Content[i+1]
		if val.Kind != yamlv3.MappingNode {
			continue
		}
		// 子键删除同样倒序
		var delIdx []int
		for j := 0; j < len(val.Content); j += 2 {
			if keysToRemove[val.Content[j].Value] {
				delIdx = append([]int{j}, delIdx...)
			}
		}
		for _, j := range delIdx {
			val.Content = append(val.Content[:j], val.Content[j+2:]...)
		}
	}
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

// checkItemHealth 检查单个部署明细的健康状态
func (s *DeployService) checkItemHealth(deployment *model.Deployment, item *model.DeploymentItem) string {
	if deployment.DeployType == "merge" {
		// Config 合并部署: targetPath 是配置文件,检查 link_path 对应的 key 是否仍在
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
		if _, exists := settings[item.LinkPath]; exists {
			return "ok"
		}
		// 兼容历史:扫所有子映射
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
		// 确定 deploy_type
		deployType := "symlink"
		for _, e := range entries {
			if e.Type == "mcp" {
				deployType = "merge"
				break
			}
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

// CheckConflicts Config 批量部署预检冲突（不写入任何文件）
// 检测：1. 待部署资源之间的 key 冲突  2. 与目标文件已有内容（原始+已部署Config）的冲突
// 返回 has_conflict 仅在存在真正需要用户决策的冲突时为 true
// (ignored=被牺牲的资源, existing=与已有内容的冲突)；纯 applied 项不算冲突
func (s *DeployService) CheckConflicts(req *model.CheckConflictsReq) (*model.CheckConflictsResp, error) {
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

	// 收集每个待部署资源的 key 集合
	type resKeys struct {
		id   string
		name string
		keys map[string]bool
	}
	var pending []resKeys
	for _, rid := range req.ResourceIDs {
		r, err := s.resourceRepo.GetResourceByID(rid)
		if err != nil || r == nil {
			continue
		}
		keys := s.getConfigResourceKeys(rid)
		pending = append(pending, resKeys{id: rid, name: r.Name, keys: keys})
	}

	// 已有内容中被已部署 MCP 管理的 key
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

	// 合并有 key 交集的资源
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			for k := range pending[i].keys {
				if pending[j].keys[k] {
					union(i, j)
					break
				}
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

	// 2. 检测与已部署 MCP 的冲突
	allNewKeys := map[string]bool{}
	for _, p := range pending {
		for k := range p.keys {
			allNewKeys[k] = true
		}
	}
	existingConflicts := s.findConfigConflicts(targetPath, "", allNewKeys)
	for _, ec := range existingConflicts {
		isSelf := false
		for _, p := range pending {
			if p.id == ec.resourceID {
				isSelf = true
				break
			}
		}
		if !isSelf && !seen[ec.name] {
			seen[ec.name] = true
			conflicts = append(conflicts, model.ConflictItem{ResourceName: ec.name, Status: "existing"})
		}
	}

	// 3. 检测与原始内容的冲突（目标中非已部署 MCP 管理的 key）
	for k := range allNewKeys {
		if managedKeys[k] {
			continue
		}
		if _, exists := targetJSON[k]; exists {
			if !seen["原始内容"] {
				seen["原始内容"] = true
				conflicts = append(conflicts, model.ConflictItem{ResourceName: "原始内容", Status: "existing"})
			}
			break
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

// GetResourceDeployTargets 获取某资源已部署到的所有目标路径（MCP 保存后同步用）
// 返回每个目标路径的 deployment_id、target_path 及是否有冲突
func (s *DeployService) GetResourceDeployTargets(resourceID string) ([]model.ResourceDeployTarget, error) {
	items, err := s.deployRepo.GetDeploymentItemsByResourceID(resourceID)
	if err != nil {
		return nil, model.NewBizError(model.ErrSystemDB, err.Error())
	}

	// 按 deployment 去重（同 deployment 下多 item 只出现一次）
	seen := map[string]bool{}
	var result []model.ResourceDeployTarget

	for _, item := range items {
		if seen[item.DeploymentID] {
			continue
		}
		seen[item.DeploymentID] = true

		dep, err := s.deployRepo.GetDeploymentByID(item.DeploymentID)
		if err != nil || dep == nil {
			continue
		}

		// 检测冲突
		hasConflict := false
		if dep.DeployType == "merge" && item.LinkPath != "" {
			hasConflict = s.checkConfigKeyConflict(dep.TargetPath, item.LinkPath, resourceID)
		}

		// 查别名名称
		aliasName := ""
		if dep.AliasID != nil && *dep.AliasID != "" {
			alias, aErr := s.aliasRepo.GetAliasByID(*dep.AliasID)
			if aErr == nil && alias != nil {
				aliasName = alias.Name
			}
		}

		result = append(result, model.ResourceDeployTarget{
			DeploymentID: dep.ID,
			TargetPath:   dep.TargetPath,
			AliasName:    aliasName,
			HasConflict:  hasConflict,
		})
	}

	if result == nil {
		result = []model.ResourceDeployTarget{}
	}
	return result, nil
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
