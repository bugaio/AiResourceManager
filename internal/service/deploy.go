// Package service deploy.go 实现部署管理的业务逻辑
// 包括执行部署、撤销部署、健康检查、修复、清理等操作
package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/anthropic/airesourcemanager/internal/model"
	"github.com/anthropic/airesourcemanager/internal/repo"
	"github.com/anthropic/airesourcemanager/internal/util"
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
	//    - mcp: 目标路径必须是一个已存在的 .json 文件（合并写入），不存在则报错
	//    - skill/agent: 目标路径是目录，不存在则创建
	isMCP := resources[0].Type == "mcp"
	if isMCP {
		if filepath.Ext(targetPath) != ".json" {
			return nil, model.NewBizError(model.ErrDeployInvalid, fmt.Sprintf("MCP 目标路径必须是 .json 文件: %s", targetPath))
		}
		if !util.FileExists(targetPath) {
			return nil, model.NewBizError(model.ErrDeployFailed, fmt.Sprintf("目标文件不存在，请先创建: %s", targetPath))
		}
		if util.IsDir(targetPath) {
			return nil, model.NewBizError(model.ErrDeployInvalid, fmt.Sprintf("MCP 目标路径必须是文件而非目录: %s", targetPath))
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
		case "mcp":
			linkPath, err = s.deployMCP(&res, targetPath, req.Force)
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
			s.removeMCPResourceKeys(deployment.TargetPath, item.ResourceID)
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
// 参数 resourceType: 资源类型过滤（skill/agent/mcp）；为空则不过滤返回全部
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
	case "mcp":
		_, err = s.deployMCP(resource, deployment.TargetPath, true)
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
				s.removeMCPResourceKeys(dep.TargetPath, item.ResourceID)
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
	case "mcp":
		linkPath, err = s.deployMCP(resource, targetPath, true)
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
			s.removeMCPResourceKeys(targetPath, item.ResourceID)
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

// deployMCP 部署 mcp 类型资源（深度合并到目标 .json 文件）
// targetPath 是一个已存在的 .json 文件（如 claude_desktop_config.json）
// 采用 lodash 式深度合并：从第一层开始递归合并嵌套对象，标量/数组由源覆盖目标
// 返回 mcpServers 下新增/覆盖的 key 名（用于 link_path / 健康检查）
func (s *DeployService) deployMCP(resource *model.Resource, targetPath string, force bool) (string, error) {
	// 读取资源自身的 MCP 配置（完整 JSON）
	mcpFile := filepath.Join(s.baseDir, "mcps", resource.ID+".jsonc")
	data, err := os.ReadFile(mcpFile)
	if err != nil {
		return "", model.NewBizError(model.ErrDeployFailed, fmt.Sprintf("读取 MCP 文件失败: %v", err))
	}
	jsonData, err := util.ParseJSONC(data)
	if err != nil {
		return "", model.NewBizError(model.ErrDeployFailed, fmt.Sprintf("解析 JSONC 失败: %v", err))
	}
	var mcpConfig map[string]interface{}
	if err := json.Unmarshal(jsonData, &mcpConfig); err != nil {
		return "", model.NewBizError(model.ErrDeployFailed, fmt.Sprintf("解析 MCP JSON 失败: %v", err))
	}
	if len(mcpConfig) == 0 {
		return "", model.NewBizError(model.ErrDeployFailed, "MCP 配置文件为空")
	}

	// 收集新 MCP 的所有顶层 key（合并时会写入目标的 key）
	newKeys := make(map[string]bool)
	for k := range mcpConfig {
		newKeys[k] = true
	}
	// 若有 mcpServers，也收集其子 key
	if servers, ok := mcpConfig["mcpServers"].(map[string]interface{}); ok {
		for k := range servers {
			newKeys[k] = true
		}
	}

	// 提取 serverName 用作 link_path（优先 mcpServers 下，其次顶层第一个 key）
	var serverName string
	if servers, ok := mcpConfig["mcpServers"].(map[string]interface{}); ok {
		for k := range servers {
			serverName = k
			break
		}
	}
	if serverName == "" {
		for k := range mcpConfig {
			serverName = k
			break
		}
	}

	// 读取目标 .json 文件
	targetData, err := os.ReadFile(targetPath)
	if err != nil {
		return "", model.NewBizError(model.ErrDeployFailed, fmt.Sprintf("读取目标文件失败: %v", err))
	}
	var targetJSON map[string]interface{}
	if len(targetData) > 0 {
		stdJSON, parseErr := util.ParseJSONC(targetData)
		if parseErr != nil {
			stdJSON = targetData
		}
		if err := json.Unmarshal(stdJSON, &targetJSON); err != nil {
			return "", model.NewBizError(model.ErrDeployFailed, fmt.Sprintf("解析目标 JSON 失败: %v", err))
		}
	}
	if targetJSON == nil {
		targetJSON = map[string]interface{}{}
	}

	// 查找该 targetPath 下已有的 MCP 部署，检测 key 冲突
	conflictResources := s.findMCPConflicts(targetPath, resource.ID, newKeys)

	// 检测目标文件中"原始内容"（非部署管理的 key）与新 MCP 的 key 冲突
	managedKeys := s.getManagedKeys(targetPath, resource.ID)
	var originalConflictKeys []string
	for k := range newKeys {
		if managedKeys[k] {
			continue // 该 key 由某个已部署 MCP 管理，冲突已在 conflictResources 中处理
		}
		if _, exists := targetJSON[k]; exists {
			originalConflictKeys = append(originalConflictKeys, k)
		}
	}

	hasConflict := len(conflictResources) > 0 || len(originalConflictKeys) > 0

	if hasConflict && !force {
		// 构造冲突名列表：已部署 MCP 名 + "原始内容"
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

	// force 覆盖：先撤销冲突 MCP 的部署
	if len(conflictResources) > 0 && force {
		for _, cr := range conflictResources {
			s.undeployMCPResource(cr.resourceID, cr.deploymentID, targetPath)
		}
		// 重新读取目标文件
		targetData, err = os.ReadFile(targetPath)
		if err != nil {
			return "", model.NewBizError(model.ErrDeployFailed, fmt.Sprintf("读取目标文件失败: %v", err))
		}
		targetJSON = map[string]interface{}{}
		if len(targetData) > 0 {
			stdJSON, parseErr := util.ParseJSONC(targetData)
			if parseErr != nil {
				stdJSON = targetData
			}
			json.Unmarshal(stdJSON, &targetJSON)
		}
	}

	// force 覆盖：移除目标中冲突的原始 key
	if len(originalConflictKeys) > 0 && force {
		for _, k := range originalConflictKeys {
			delete(targetJSON, k)
		}
	}

	// 直接从第一层深度合并
	merged := util.DeepMerge(targetJSON, mcpConfig)

	output, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return "", model.NewBizError(model.ErrDeployFailed, fmt.Sprintf("序列化目标 JSON 失败: %v", err))
	}
	if err := os.WriteFile(targetPath, output, 0644); err != nil {
		return "", model.NewBizError(model.ErrDeployFailed, fmt.Sprintf("写入目标文件失败: %v", err))
	}

	return serverName, nil
}

// mcpConflictInfo 冲突的 MCP 资源信息
type mcpConflictInfo struct {
	resourceID   string
	deploymentID string
	name         string
}

// findMCPConflicts 查找目标路径下已部署的 MCP 中与新 key 集合有冲突的资源
// 不依赖 link_path（它只存第一个 key），而是读每个已部署 MCP 资源的实际配置，取全部 key 做交集
func (s *DeployService) findMCPConflicts(targetPath, selfResourceID string, newKeys map[string]bool) []mcpConflictInfo {
	allDeps, err := s.deployRepo.GetDeploymentsByTarget()
	if err != nil {
		return nil
	}
	deployments, ok := allDeps[targetPath]
	if !ok {
		return nil
	}

	var conflicts []mcpConflictInfo
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
			// 读该已部署 MCP 资源的实际配置，获取其所有 key
			existingKeys := s.getMCPResourceKeys(item.ResourceID)
			// 检查与新 MCP 的 key 是否有交集
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
				conflicts = append(conflicts, mcpConflictInfo{
					resourceID:   item.ResourceID,
					deploymentID: dep.ID,
					name:         resName,
				})
			}
		}
	}
	return conflicts
}

// getMCPResourceKeys 读取 MCP 资源文件，返回其所有顶层 key + mcpServers 子 key
func (s *DeployService) getMCPResourceKeys(resourceID string) map[string]bool {
	keys := map[string]bool{}
	r, err := s.resourceRepo.GetResourceByID(resourceID)
	if err != nil || r == nil {
		return keys
	}
	mcpFile := filepath.Join(s.baseDir, "mcps", r.ID+".jsonc")
	data, err := os.ReadFile(mcpFile)
	if err != nil {
		return keys
	}
	jsonData, _ := util.ParseJSONC(data)
	var cfg map[string]interface{}
	if json.Unmarshal(jsonData, &cfg) != nil {
		return keys
	}
	for k := range cfg {
		keys[k] = true
	}
	if servers, ok := cfg["mcpServers"].(map[string]interface{}); ok {
		for k := range servers {
			keys[k] = true
		}
	}
	return keys
}

// getManagedKeys 获取 targetPath 下所有已部署 MCP 管理的 key 集合（排除 selfResourceID）
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
			for k := range s.getMCPResourceKeys(item.ResourceID) {
				managed[k] = true
			}
		}
	}
	return managed
}

// undeployMCPResource 撤销某 MCP 资源在指定目标的部署（删其写入的所有 key + 清 DB）
func (s *DeployService) undeployMCPResource(resourceID, deploymentID, targetPath string) {
	// 读取该资源的 MCP 配置，获取其写入的所有顶层 key
	r, err := s.resourceRepo.GetResourceByID(resourceID)
	if err != nil || r == nil {
		// 资源已不存在，只清 DB
		s.UndeployResourceFromTarget(resourceID, deploymentID, targetPath, "merge")
		return
	}

	mcpFile := filepath.Join(s.baseDir, "mcps", r.ID+".jsonc")
	data, err := os.ReadFile(mcpFile)
	if err != nil {
		s.UndeployResourceFromTarget(resourceID, deploymentID, targetPath, "merge")
		return
	}
	jsonData, _ := util.ParseJSONC(data)
	var mcpConfig map[string]interface{}
	if json.Unmarshal(jsonData, &mcpConfig) != nil {
		s.UndeployResourceFromTarget(resourceID, deploymentID, targetPath, "merge")
		return
	}

	// 从目标文件中移除该 MCP 的所有顶层 key
	if util.FileExists(targetPath) {
		tData, err := os.ReadFile(targetPath)
		if err == nil {
			stdJSON, pErr := util.ParseJSONC(tData)
			if pErr != nil {
				stdJSON = tData
			}
			var targetObj map[string]interface{}
			if json.Unmarshal(stdJSON, &targetObj) == nil {
				for k := range mcpConfig {
					delete(targetObj, k)
					// 也从 mcpServers 下删
					if servers, ok := targetObj["mcpServers"].(map[string]interface{}); ok {
						delete(servers, k)
					}
				}
				// mcpServers 下的子 key 也要删
				if servers, ok := mcpConfig["mcpServers"].(map[string]interface{}); ok {
					if tServers, ok2 := targetObj["mcpServers"].(map[string]interface{}); ok2 {
						for k := range servers {
							delete(tServers, k)
						}
					}
				}
				output, _ := json.MarshalIndent(targetObj, "", "  ")
				os.WriteFile(targetPath, output, 0644)
			}
		}
	}

	// 清 DB 记录
	s.UndeployResourceFromTarget(resourceID, deploymentID, targetPath, "merge")
}

// removeMCPKey 从目标 .json 文件中移除指定 key（顶层优先，其次 mcpServers 下）
// targetPath 是 MCP 部署的目标 .json 文件本身
func (s *DeployService) removeMCPKey(targetPath, key string) {
	if !util.FileExists(targetPath) {
		return
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		return
	}

	stdJSON, parseErr := util.ParseJSONC(data)
	if parseErr != nil {
		stdJSON = data
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(stdJSON, &settings); err != nil {
		return
	}

	removed := false
	// 先尝试从顶层删除
	if _, exists := settings[key]; exists {
		delete(settings, key)
		removed = true
	}
	// 再尝试从 mcpServers 下删除
	if mcpServers, ok := settings["mcpServers"].(map[string]interface{}); ok {
		if _, exists := mcpServers[key]; exists {
			delete(mcpServers, key)
			settings["mcpServers"] = mcpServers
			removed = true
		}
	}

	if !removed {
		return
	}

	output, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(targetPath, output, 0644)
}

// removeMCPResourceKeys 根据资源 ID 读取其 MCP 配置，从目标文件中移除该资源写入的所有 key
func (s *DeployService) removeMCPResourceKeys(targetPath, resourceID string) {
	// 获取该 MCP 资源的所有 key
	allKeys := s.getMCPResourceKeys(resourceID)
	if len(allKeys) == 0 {
		return
	}
	if !util.FileExists(targetPath) {
		return
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		return
	}
	stdJSON, parseErr := util.ParseJSONC(data)
	if parseErr != nil {
		stdJSON = data
	}
	var obj map[string]interface{}
	if json.Unmarshal(stdJSON, &obj) != nil {
		return
	}

	// 读资源配置原文拿到精确结构
	r, rErr := s.resourceRepo.GetResourceByID(resourceID)
	if rErr != nil || r == nil {
		// 资源已删，用 allKeys fallback
		for k := range allKeys {
			delete(obj, k)
			if servers, ok := obj["mcpServers"].(map[string]interface{}); ok {
				delete(servers, k)
			}
		}
	} else {
		mcpFile := filepath.Join(s.baseDir, "mcps", r.ID+".jsonc")
		cfgData, err := os.ReadFile(mcpFile)
		if err == nil {
			jsonData, _ := util.ParseJSONC(cfgData)
			var cfg map[string]interface{}
			if json.Unmarshal(jsonData, &cfg) == nil {
				// 删除该 MCP 写入的所有顶层 key
				for k := range cfg {
					delete(obj, k)
				}
				// 若 MCP 有 mcpServers，从目标的 mcpServers 中也删
				if servers, ok := cfg["mcpServers"].(map[string]interface{}); ok {
					if tServers, ok2 := obj["mcpServers"].(map[string]interface{}); ok2 {
						for k := range servers {
							delete(tServers, k)
						}
					}
				}
			}
		}
	}

	output, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(targetPath, output, 0644)
}

// checkItemHealth 检查单个部署明细的健康状态
func (s *DeployService) checkItemHealth(deployment *model.Deployment, item *model.DeploymentItem) string {
	if deployment.DeployType == "merge" {
		// MCP: targetPath 即目标 .json 文件，检查文件中是否仍含 link_path 对应的 key
		data, err := os.ReadFile(deployment.TargetPath)
		if err != nil {
			return "broken"
		}
		stdJSON, parseErr := util.ParseJSONC(data)
		if parseErr != nil {
			stdJSON = data
		}
		var settings map[string]interface{}
		if err := json.Unmarshal(stdJSON, &settings); err != nil {
			return "broken"
		}
		// 在顶层和 mcpServers 下查找该 key
		if _, exists := settings[item.LinkPath]; exists {
			return "ok"
		}
		if mcpServers, ok := settings["mcpServers"].(map[string]interface{}); ok {
			if _, exists := mcpServers[item.LinkPath]; exists {
				return "ok"
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

// CheckConflicts MCP 批量部署预检冲突（不写入任何文件）
// 检测：1. 待部署资源之间的 key 冲突  2. 与目标文件已有内容（原始+已部署MCP）的冲突
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

	// 读目标文件当前内容
	var targetJSON map[string]interface{}
	if util.FileExists(targetPath) {
		data, _ := os.ReadFile(targetPath)
		if len(data) > 0 {
			stdJSON, pErr := util.ParseJSONC(data)
			if pErr != nil {
				stdJSON = data
			}
			json.Unmarshal(stdJSON, &targetJSON)
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
		keys := s.getMCPResourceKeys(rid)
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
	existingConflicts := s.findMCPConflicts(targetPath, "", allNewKeys)
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
		HasConflict: len(conflicts) > 0,
		Conflicts:   conflicts,
	}, nil
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
			hasConflict = s.checkMCPKeyConflict(dep.TargetPath, item.LinkPath, resourceID)
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

// checkMCPKeyConflict 检查目标 json 文件中是否已存在该 key（判定为冲突）
func (s *DeployService) checkMCPKeyConflict(targetPath, key, _ string) bool {
	if !util.FileExists(targetPath) {
		return false
	}
	data, err := os.ReadFile(targetPath)
	if err != nil {
		return false
	}
	stdJSON, parseErr := util.ParseJSONC(data)
	if parseErr != nil {
		stdJSON = data
	}
	var obj map[string]interface{}
	if json.Unmarshal(stdJSON, &obj) != nil {
		return false
	}
	// 顶层或 mcpServers 下存在该 key
	if _, exists := obj[key]; exists {
		return true
	}
	if servers, ok := obj["mcpServers"].(map[string]interface{}); ok {
		if _, exists := servers[key]; exists {
			return true
		}
	}
	return false
}
