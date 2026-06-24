// Package service resource.go 实现资源的业务逻辑
// 包括创建/删除时的文件系统操作、校验、级联删除等
package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropic/airesourcemanager/internal/model"
	"github.com/anthropic/airesourcemanager/internal/repo"
	"github.com/anthropic/airesourcemanager/internal/util"
)

// ResourceService 资源业务服务
type ResourceService struct {
	repo       *repo.ResourceRepo
	baseDir    string // ~/.aiManager 根目录
	deploySvc  *DeployService
	watcherSvc *WatcherService
	presetRepo *repo.PresetRepo // 用于资源删除时检查 preset 关联
}

// NewResourceService 创建资源服务实例
// 参数 repo: 资源数据仓库
// 参数 baseDir: 资源文件存储根目录（~/.aiManager）
// 返回: ResourceService 指针
func NewResourceService(repo *repo.ResourceRepo, baseDir string) *ResourceService {
	return &ResourceService{repo: repo, baseDir: baseDir}
}

// SetDeployService 注入部署服务（用于删除资源时级联撤销部署）
func (s *ResourceService) SetDeployService(deploySvc *DeployService) {
	s.deploySvc = deploySvc
}

// SetWatcherService 注入文件监听服务(用于程序主动删文件时抑制误报的 deleted 广播)
func (s *ResourceService) SetWatcherService(w *WatcherService) {
	s.watcherSvc = w
}

// SetPresetRepo 注入 preset 数据仓库（用于资源删除拦截 1005）
func (s *ResourceService) SetPresetRepo(r *repo.PresetRepo) {
	s.presetRepo = r
}

// CreateResource 创建资源
// 参数 req: 创建请求
// 返回: 创建的资源实体、错误信息
// 逻辑: 校验类型和名称 → 生成UUID → 创建文件 → 写入数据库 → 关联分组
func (s *ResourceService) CreateResource(req *model.CreateResourceReq) (*model.Resource, error) {
	return s.createResourceInternal(req, nil)
}

// CreatePrivateResource 创建 preset 私有资源（owner_preset_id 非空）
// 校验流程与 CreateResource 一致，仅将 owner_preset_id 写入 DB
func (s *ResourceService) CreatePrivateResource(presetID string, req *model.CreateResourceReq) (*model.Resource, error) {
	if strings.TrimSpace(presetID) == "" {
		return nil, model.NewBizError(model.ErrParamValidation, "presetID 不能为空")
	}
	return s.createResourceInternal(req, &presetID)
}

// createResourceInternal 创建资源的内部实现，ownerPresetID 非 nil 表示私有资源
func (s *ResourceService) createResourceInternal(req *model.CreateResourceReq, ownerPresetID *string) (*model.Resource, error) {
	// 校验类型
	if !isValidType(req.Type) {
		return nil, model.NewBizError(model.ErrParamValidation, "type 必须为 skill/agent/config/prompt")
	}
	// 校验名称
	if strings.TrimSpace(req.Name) == "" {
		return nil, model.NewBizError(model.ErrParamValidation, "name 不能为空")
	}

	// 检查名称重复
	exists, err := s.repo.CheckNameExists(req.Type, req.Name, "")
	if err != nil {
		return nil, model.NewBizError(model.ErrResourceFileIO, err.Error())
	}
	if exists {
		return nil, model.NewBizError(model.ErrResourceExists, "同类型下资源名称已存在")
	}

	// 生成 UUID 和文件路径
	uuid := util.NewUUID()
	filePath, err := s.createFiles(req.Type, req.Name, uuid, req.Description, ownerPresetID)
	if err != nil {
		return nil, model.NewBizError(model.ErrResourceFileIO, err.Error())
	}

	// 构造资源实体
	now := timeNow()
	resource := &model.Resource{
		ID:            uuid,
		Name:          req.Name,
		Type:          req.Type,
		Path:          filePath,
		Description:   req.Description,
		Metadata:      "{}",
		CreatedAt:     now,
		UpdatedAt:     now,
		OwnerPresetID: ownerPresetID,
	}

	// 写入数据库
	if err := s.repo.InsertResource(resource); err != nil {
		return nil, model.NewBizError(model.ErrResourceFileIO, err.Error())
	}

	// 关联分组 (私有资源不进分组)
	if ownerPresetID == nil && req.GroupID != "" && req.GroupID != "0" {
		if err := s.repo.InsertGroupResource(req.GroupID, resource.ID); err != nil {
			// 分组关联失败不影响创建结果，只记录
			_ = err
		}
	}

	return resource, nil
}

// ImportedSkillFile 导入 skill 时的单个文件条目
// RelPath 是相对于 skill 子目录的路径(如 "SKILL.md"、"assets/x.png")
// Data 是文件原始字节
type ImportedSkillFile struct {
	RelPath string
	Data    []byte
}

// ImportSkill 导入一个 skill: 不走模板创建,而是把外部目录的全部文件原样写入 {baseDir}/skills/{uuid}/
// 参数 name: 从 SKILL.md frontmatter 解析得到的名称
// 参数 description: 从 SKILL.md frontmatter 解析得到的描述
// 参数 groupID: 可选关联分组
// 参数 files: 该 skill 子目录下的所有文件(含相对路径)
// 返回: 创建的资源、错误
func (s *ResourceService) ImportSkill(name, description, groupID string, files []ImportedSkillFile) (*model.Resource, error) {
	return s.importSkillInternal(name, description, groupID, files, nil)
}

// ImportPrivateSkill 导入私有 skill（owner_preset_id 非空）
func (s *ResourceService) ImportPrivateSkill(presetID, name, description string, files []ImportedSkillFile) (*model.Resource, error) {
	if strings.TrimSpace(presetID) == "" {
		return nil, model.NewBizError(model.ErrParamValidation, "presetID 不能为空")
	}
	return s.importSkillInternal(name, description, "", files, &presetID)
}

func (s *ResourceService) importSkillInternal(name, description, groupID string, files []ImportedSkillFile, ownerPresetID *string) (*model.Resource, error) {
	if strings.TrimSpace(name) == "" {
		return nil, model.NewBizError(model.ErrParamValidation, "name 不能为空")
	}
	if len(files) == 0 {
		return nil, model.NewBizError(model.ErrParamValidation, "files 不能为空")
	}

	// 同类型重名校验(与 CreateResource 一致)
	exists, err := s.repo.CheckNameExists("skill", name, "")
	if err != nil {
		return nil, model.NewBizError(model.ErrResourceFileIO, err.Error())
	}
	if exists {
		return nil, model.NewBizError(model.ErrResourceExists, "同类型下资源名称已存在")
	}

	uuid := util.NewUUID()
	dir := filepath.Join(s.typeDir("skill", ownerPresetID), uuid)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, model.NewBizError(model.ErrResourceFileIO, "创建 skill 目录失败: "+err.Error())
	}

	// 原样写入所有文件,任一失败则清理并报错
	for _, f := range files {
		clean, errSan := sanitizeRelPath(f.RelPath)
		if errSan != nil {
			_ = os.RemoveAll(dir)
			return nil, model.NewBizError(model.ErrParamValidation, errSan.Error())
		}
		dst := filepath.Join(dir, clean)
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			_ = os.RemoveAll(dir)
			return nil, model.NewBizError(model.ErrResourceFileIO, "创建子目录失败: "+err.Error())
		}
		if err := os.WriteFile(dst, f.Data, 0644); err != nil {
			_ = os.RemoveAll(dir)
			return nil, model.NewBizError(model.ErrResourceFileIO, "写入文件失败: "+err.Error())
		}
	}

	// 写 DB(与 CreateResource 完全相同的字段)
	now := timeNow()
	resource := &model.Resource{
		ID:            uuid,
		Name:          name,
		Type:          "skill",
		Path:          dir,
		Description:   description,
		Metadata:      "{}",
		CreatedAt:     now,
		UpdatedAt:     now,
		OwnerPresetID: ownerPresetID,
	}
	if err := s.repo.InsertResource(resource); err != nil {
		_ = os.RemoveAll(dir)
		return nil, model.NewBizError(model.ErrResourceFileIO, err.Error())
	}

	// 关联分组(与 CreateResource 一致,失败不阻断；私有资源不进分组)
	if ownerPresetID == nil && groupID != "" && groupID != "0" {
		_ = s.repo.InsertGroupResource(groupID, resource.ID)
	}

	return resource, nil
}

// sanitizeRelPath 校验相对路径,防止穿越攻击
// - 不能以 / 开头
// - 不能含 ".." 段
// - filepath.Clean 后不能跑出当前目录
func sanitizeRelPath(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("文件相对路径为空")
	}
	// 统一分隔符(前端可能传 / )
	p = strings.ReplaceAll(p, "\\", "/")
	if strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("非法路径(绝对路径): %s", p)
	}
	clean := filepath.Clean(p)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return "", fmt.Errorf("非法路径(穿越): %s", p)
	}
	return clean, nil
}

// ImportAgent 导入一个 agent: 把外部 .md 文件原样写入 {baseDir}/agents/{uuid}.md
// 参数 name: 从 frontmatter 解析得到的名称(必填)
// 参数 description: 从 frontmatter 解析得到的描述(可空)
// 参数 groupID: 可选关联分组
// 参数 data: 源 .md 文件原始字节
// 返回: 创建的资源、错误
func (s *ResourceService) ImportAgent(name, description, groupID string, data []byte) (*model.Resource, error) {
	return s.importAgentInternal(name, description, groupID, data, nil)
}

// ImportPrivateAgent 导入私有 agent（owner_preset_id 非空）
func (s *ResourceService) ImportPrivateAgent(presetID, name, description string, data []byte) (*model.Resource, error) {
	if strings.TrimSpace(presetID) == "" {
		return nil, model.NewBizError(model.ErrParamValidation, "presetID 不能为空")
	}
	return s.importAgentInternal(name, description, "", data, &presetID)
}

func (s *ResourceService) importAgentInternal(name, description, groupID string, data []byte, ownerPresetID *string) (*model.Resource, error) {
	if strings.TrimSpace(name) == "" {
		return nil, model.NewBizError(model.ErrParamValidation, "name 不能为空")
	}
	if len(data) == 0 {
		return nil, model.NewBizError(model.ErrParamValidation, "文件内容为空")
	}

	// 同类型重名校验(与 CreateResource 一致)
	exists, err := s.repo.CheckNameExists("agent", name, "")
	if err != nil {
		return nil, model.NewBizError(model.ErrResourceFileIO, err.Error())
	}
	if exists {
		return nil, model.NewBizError(model.ErrResourceExists, "同类型下资源名称已存在")
	}

	uuid := util.NewUUID()
	dir := s.typeDir("agent", ownerPresetID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, model.NewBizError(model.ErrResourceFileIO, "创建 agents 目录失败: "+err.Error())
	}
	filePath := filepath.Join(dir, uuid+".md")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return nil, model.NewBizError(model.ErrResourceFileIO, "写入 agent 文件失败: "+err.Error())
	}

	now := timeNow()
	resource := &model.Resource{
		ID:            uuid,
		Name:          name,
		Type:          "agent",
		Path:          filePath,
		Description:   description,
		Metadata:      "{}",
		CreatedAt:     now,
		UpdatedAt:     now,
		OwnerPresetID: ownerPresetID,
	}
	if err := s.repo.InsertResource(resource); err != nil {
		_ = os.Remove(filePath)
		return nil, model.NewBizError(model.ErrResourceFileIO, err.Error())
	}

	if ownerPresetID == nil && groupID != "" && groupID != "0" {
		_ = s.repo.InsertGroupResource(groupID, resource.ID)
	}
	return resource, nil
}
// 参数 id: 资源 ID
// 返回: 资源实体、错误信息
func (s *ResourceService) GetResource(id string) (*model.Resource, error) {
	r, err := s.repo.GetResourceByID(id)
	if err != nil {
		return nil, model.NewBizError(model.ErrResourceFileIO, err.Error())
	}
	if r == nil {
		return nil, model.NewBizError(model.ErrResourceNotFound, "资源不存在")
	}
	return r, nil
}

// ListResources 分页查询资源列表
// 参数 resourceType: 类型筛选
// 参数 search: 名称搜索
// 参数 groupID: 分组 ID
// 参数 ownerPresetID: 私有资源过滤（详见 repo.ListResources 注释）
// 参数 page: 页码
// 参数 pageSize: 每页数量
// 返回: 分页响应、错误信息
func (s *ResourceService) ListResources(resourceType, search, groupID, ownerPresetID string, page, pageSize int) (*model.ResourceListResp, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	list, total, err := s.repo.ListResources(resourceType, search, groupID, ownerPresetID, page, pageSize)
	if err != nil {
		return nil, model.NewBizError(model.ErrResourceFileIO, err.Error())
	}

	// 填充 preset_links：批量查询本页资源各自被哪些 preset 关联（避免 N+1）
	if s.presetRepo != nil && len(list) > 0 {
		ids := make([]string, len(list))
		for i := range list {
			ids[i] = list[i].ID
		}
		if linkMap, lerr := s.presetRepo.ListPresetsByResourceIDs(ids); lerr == nil {
			for i := range list {
				list[i].PresetLinks = linkMap[list[i].ID]
			}
		}
	}

	return &model.ResourceListResp{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// UpdateResource 更新资源元数据（名称、描述）
// 参数 id: 资源 ID
// 参数 req: 更新请求
// 返回: 更新后的资源实体、错误信息
func (s *ResourceService) UpdateResource(id string, req *model.UpdateResourceReq) (*model.Resource, error) {
	// 先查询资源是否存在
	r, err := s.repo.GetResourceByID(id)
	if err != nil {
		return nil, model.NewBizError(model.ErrResourceFileIO, err.Error())
	}
	if r == nil {
		return nil, model.NewBizError(model.ErrResourceNotFound, "资源不存在")
	}

	// 名称重复检查
	if req.Name != nil && *req.Name != r.Name {
		exists, err := s.repo.CheckNameExists(r.Type, *req.Name, id)
		if err != nil {
			return nil, model.NewBizError(model.ErrResourceFileIO, err.Error())
		}
		if exists {
			return nil, model.NewBizError(model.ErrResourceExists, "同类型下资源名称已存在")
		}
	}

	if err := s.repo.UpdateResource(id, req.Name, req.Description); err != nil {
		return nil, model.NewBizError(model.ErrResourceFileIO, err.Error())
	}

	// 返回最新数据
	return s.repo.GetResourceByID(id)
}

// DeleteResource 删除资源
// 参数 id: 资源 ID
// 参数 confirm: 是否确认级联部署删除
// 参数 unlink: 资源被 preset 关联时,是否级联解除关联并撤销受影响的 preset 部署
// 返回:
//   - data:  附加信息（关联部署 / 关联 preset）
//   - error: 业务错误
//
// 优先级: preset 关联拦截 (1005) > 部署关联拦截 (1004)
func (s *ResourceService) DeleteResource(id string, confirm bool, unlink bool) (interface{}, error) {
	r, err := s.repo.GetResourceByID(id)
	if err != nil {
		return nil, model.NewBizError(model.ErrResourceFileIO, err.Error())
	}
	if r == nil {
		return nil, model.NewBizError(model.ErrResourceNotFound, "资源不存在")
	}

	// 私有资源不允许通过普通接口删除,要走 preset 的级联/私有资源接口
	if r.OwnerPresetID != nil && *r.OwnerPresetID != "" {
		return nil, model.NewBizError(model.ErrResourceLockedByPreset,
			"该资源是 preset 的私有资源,请通过 preset 接口删除")
	}

	// 1. preset 关联拦截（优先于部署拦截）
	if s.presetRepo != nil {
		links, lErr := s.presetRepo.ListPresetsByResourceID(id)
		if lErr != nil {
			return nil, model.NewBizError(model.ErrSystemDB, lErr.Error())
		}
		if len(links) > 0 {
			if !unlink {
				return model.ResourceDeleteBlockedData{Presets: links},
					model.NewBizError(model.ErrResourceLockedByPreset, "资源被 preset 管理,需先解除关联")
			}
			// unlink=true: 解除全部关联 → 撤销受影响 preset 的部署(让上层 PresetService 处理)
			affectedPresets := make([]string, 0, len(links))
			for _, p := range links {
				affectedPresets = append(affectedPresets, p.ID)
			}
			// 解除关联
			for _, pid := range affectedPresets {
				if uErr := s.presetRepo.UnlinkResources(pid, []string{id}); uErr != nil {
					return nil, model.NewBizError(model.ErrSystemDB, uErr.Error())
				}
			}
			// 重新部署受影响的 preset（移除该资源的链接）
			if s.deploySvc != nil {
				for _, pid := range affectedPresets {
					s.deploySvc.SyncPresetDeployments(pid)
				}
			}
		}
	}

	// 2. 检查部署关联
	deployments, err := s.repo.GetResourceDeployments(id)
	if err != nil {
		return nil, model.NewBizError(model.ErrResourceFileIO, err.Error())
	}

	if len(deployments) > 0 && !confirm {
		return map[string]interface{}{
			"deployments": deployments,
		}, model.NewBizError(model.ErrResourceHasDeploy, "资源存在关联部署，需确认删除")
	}

	// 级联撤销该资源在所有部署中的链接
	if s.deploySvc != nil && len(deployments) > 0 {
		for _, dep := range deployments {
			// 从该 deployment 中撤销此资源的部署项
			s.deploySvc.UndeployResourceFromTarget(id, dep.ID, dep.TargetPath, "")
		}
	}

	// 删除文件系统 — 先告诉 watcher 这是程序主动删除,避免被推送为"外部删除"
	if s.watcherSvc != nil {
		s.watcherSvc.SuppressUUID(id, 5*time.Second)
	}
	s.deleteFiles(r.Type, r.Path)

	// 删除数据库记录
	if err := s.repo.DeleteResource(id); err != nil {
		return nil, model.NewBizError(model.ErrResourceFileIO, err.Error())
	}

	return nil, nil
}

// DeletePrivateResource 物理删除私有资源（preset 级联调用）
// 不做 preset 关联拦截 / 私有锁定校验，仅撤销部署、删文件、删 DB
func (s *ResourceService) DeletePrivateResource(id string) error {
	r, err := s.repo.GetResourceByID(id)
	if err != nil {
		return model.NewBizError(model.ErrResourceFileIO, err.Error())
	}
	if r == nil {
		return nil
	}

	deployments, _ := s.repo.GetResourceDeployments(id)
	if s.deploySvc != nil && len(deployments) > 0 {
		for _, dep := range deployments {
			s.deploySvc.UndeployResourceFromTarget(id, dep.ID, dep.TargetPath, "")
		}
	}

	if s.watcherSvc != nil {
		s.watcherSvc.SuppressUUID(id, 5*time.Second)
	}
	s.deleteFiles(r.Type, r.Path)

	if err := s.repo.DeleteResource(id); err != nil {
		return model.NewBizError(model.ErrResourceFileIO, err.Error())
	}
	return nil
}

// BatchDelete 批量删除资源
// 参数 req: 批量删除请求
// 参数 unlink: 若资源被 preset 关联,是否级联解除关联（透传给 DeleteResource）
// 返回: 各项删除结果列表、错误信息
func (s *ResourceService) BatchDelete(req *model.BatchDeleteReq, unlink bool) ([]model.BatchDeleteResult, error) {
	results := make([]model.BatchDeleteResult, 0, len(req.IDs))

	for _, id := range req.IDs {
		result := model.BatchDeleteResult{ID: id}

		data, err := s.DeleteResource(id, req.Confirm, unlink)
		if err != nil {
			if bizErr, ok := err.(*model.BizError); ok {
				result.Success = false
				result.Code = bizErr.Code
				result.Msg = bizErr.Msg
				if bizErr.Code == model.ErrResourceHasDeploy {
					if m, ok := data.(map[string]interface{}); ok {
						if deps, ok := m["deployments"].([]model.DeploymentInfo); ok {
							result.Deployments = deps
						}
					}
				}
			} else {
				result.Success = false
				result.Code = model.ErrResourceFileIO
				result.Msg = err.Error()
			}
		} else {
			result.Success = true
		}

		results = append(results, result)
	}

	return results, nil
}

// GetContent 读取资源文件内容
// 参数 id: 资源 ID
// 返回: 文件内容字符串、错误信息
func (s *ResourceService) GetContent(id string) (string, error) {
	r, err := s.repo.GetResourceByID(id)
	if err != nil {
		return "", model.NewBizError(model.ErrResourceFileIO, err.Error())
	}
	if r == nil {
		return "", model.NewBizError(model.ErrResourceNotFound, "资源不存在")
	}

	contentPath := s.getContentPath(r.Type, r.Path)
	data, err := os.ReadFile(contentPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", model.NewBizError(model.ErrResourceNotFound, "资源文件不存在")
		}
		return "", model.NewBizError(model.ErrResourceFileIO, fmt.Sprintf("读取文件失败: %v", err))
	}

	return string(data), nil
}

// UpdateContent 更新资源文件内容
// 参数 id: 资源 ID
// 参数 content: 新文件内容
// 返回: 错误信息
func (s *ResourceService) UpdateContent(id string, content string) error {
	r, err := s.repo.GetResourceByID(id)
	if err != nil {
		return model.NewBizError(model.ErrResourceFileIO, err.Error())
	}
	if r == nil {
		return model.NewBizError(model.ErrResourceNotFound, "资源不存在")
	}

	contentPath := s.getContentPath(r.Type, r.Path)
	if err := os.WriteFile(contentPath, []byte(content), 0644); err != nil {
		return model.NewBizError(model.ErrResourceFileIO, fmt.Sprintf("写入文件失败: %v", err))
	}

	return nil
}

// getContentPath 根据资源类型获取内容文件路径
// 参数 resourceType: 资源类型
// 参数 basePath: 资源基础路径
// 返回: 内容文件的绝对路径
func (s *ResourceService) getContentPath(resourceType, basePath string) string {
	switch resourceType {
	case "skill":
		// skill 的 path 是目录，内容文件为 SKILL.md
		return filepath.Join(basePath, "SKILL.md")
	default:
		// agent 和 config 的 path 就是文件本身
		return basePath
	}
}

// createFiles 根据类型创建资源文件
// 参数 resourceType: 资源类型
// 参数 name: 资源名称
// 参数 uuid: 生成的 UUID
// 参数 description: 资源描述
// 参数 ownerPresetID: 非 nil 表示私有资源，存到 presets/{presetID}/ 下
// 返回: 文件路径、错误信息
func (s *ResourceService) createFiles(resourceType, name, uuid, description string, ownerPresetID *string) (string, error) {
	switch resourceType {
	case "skill":
		return s.createSkillFiles(name, uuid, description, ownerPresetID)
	case "agent":
		return s.createAgentFile(name, uuid, ownerPresetID)
	case "config":
		return s.createConfigFile(name, uuid, ownerPresetID)
	case "prompt":
		return s.createPromptFile(name, uuid, ownerPresetID)
	default:
		return "", fmt.Errorf("不支持的资源类型: %s", resourceType)
	}
}

// typeDir 返回某类型资源的存储根目录
// 全局资源按类型分目录：{baseDir}/{skills|agents|configs|prompts}
// 私有资源（ownerPresetID 非空）存到 {baseDir}/presets/{presetID}/{skills|agents|configs|prompts}
// 私有资源的类型子目录与全局资源保持一致，仅多一层 presets/{presetID} 隔离
func (s *ResourceService) typeDir(resourceType string, ownerPresetID *string) string {
	sub := typeSubdir(resourceType)
	if ownerPresetID != nil && *ownerPresetID != "" {
		return filepath.Join(s.baseDir, "presets", *ownerPresetID, sub)
	}
	return filepath.Join(s.baseDir, sub)
}

// typeSubdir 返回资源类型对应的存储子目录名（全局/私有共用）
func typeSubdir(resourceType string) string {
	switch resourceType {
	case "skill":
		return "skills"
	case "agent":
		return "agents"
	case "config":
		return "configs"
	case "prompt":
		return "prompts"
	}
	return ""
}

// createSkillFiles 创建 skill 类型的文件: {baseDir}/skills/{uuid}/SKILL.md + meta.json
// 参数 name: skill 名称
// 参数 uuid: UUID
// 参数 description: skill 描述
// 返回: skill 目录路径、错误信息
func (s *ResourceService) createSkillFiles(name, uuid, description string, ownerPresetID *string) (string, error) {
	dir := filepath.Join(s.typeDir("skill", ownerPresetID), uuid)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("创建 skill 目录失败: %w", err)
	}

	// 写入 SKILL.md（preset 私有资源创建空文件，全局资源带标题模板）
	skillContent := ""
	if ownerPresetID == nil {
		skillContent = fmt.Sprintf("# %s\n\n", name)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		return "", fmt.Errorf("写入 SKILL.md 失败: %w", err)
	}

	// 写入 meta.json
	meta := map[string]interface{}{
		"name":        name,
		"description": description,
		"version":     "1.0.0",
	}
	metaBytes, _ := json.MarshalIndent(meta, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), metaBytes, 0644); err != nil {
		return "", fmt.Errorf("写入 meta.json 失败: %w", err)
	}

	return dir, nil
}

// createAgentFile 创建 agent 类型的文件: {baseDir}/agents/{uuid}.md
// 参数 name: agent 名称
// 参数 uuid: UUID
// 返回: 文件路径、错误信息
func (s *ResourceService) createAgentFile(name, uuid string, ownerPresetID *string) (string, error) {
	dir := s.typeDir("agent", ownerPresetID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("创建 agents 目录失败: %w", err)
	}

	filePath := filepath.Join(dir, uuid+".md")
	// preset 私有资源创建空文件，全局资源带标题模板
	content := ""
	if ownerPresetID == nil {
		content = fmt.Sprintf("# %s\n\n", name)
	}
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("写入 agent 文件失败: %w", err)
	}

	return filePath, nil
}

// createConfigFile 创建 config 类型的文件: {baseDir}/configs/{uuid}.jsonc
// 参数 name: Config 资源名称
// 参数 uuid: UUID
// 返回: 文件路径、错误信息
// 默认后缀 .jsonc(JSONC 格式,允许注释). 后续编辑/导入时可改为 .yaml/.toml.
func (s *ResourceService) createConfigFile(name, uuid string, ownerPresetID *string) (string, error) {
	dir := s.typeDir("config", ownerPresetID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("创建 configs 目录失败: %w", err)
	}

	filePath := filepath.Join(dir, uuid+".jsonc")
	// preset 私有资源创建空文件，全局资源带 config 模板
	content := ""
	if ownerPresetID == nil {
		content = `{
  // config 配置片段
  // 部署时将与目标文件深度合并,保留目标原有注释和格式
  "mcpServers": {}
}
`
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("写入 config 文件失败: %w", err)
	}

	return filePath, nil
}

// createPromptFile 创建 prompt 类型的文件: {baseDir}/prompts/{uuid}.md
// 参数 name: prompt 资源名称
// 参数 uuid: UUID
// 返回: 文件路径、错误信息
// 模板内容由 PRD §2.4 给出,作为新建资源的初始编辑器内容
func (s *ResourceService) createPromptFile(name, uuid string, ownerPresetID *string) (string, error) {
	dir := s.typeDir("prompt", ownerPresetID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("创建 prompts 目录失败: %w", err)
	}

	filePath := filepath.Join(dir, uuid+".md")
	// preset 私有资源创建空文件，全局资源带 prompt 模板
	content := ""
	if ownerPresetID == nil {
		content = `<!-- 在此编写提示词内容 -->

## 用法

> 描述这段提示词的使用场景与触发条件
`
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("写入 prompt 文件失败: %w", err)
	}

	return filePath, nil
}

// deleteFiles 删除资源对应的文件系统文件
// 参数 resourceType: 资源类型
// 参数 path: 文件/目录路径
// 说明: 目标不存在时静默跳过
func (s *ResourceService) deleteFiles(resourceType, path string) {
	if path == "" {
		return
	}
	switch resourceType {
	case "skill":
		// skill path 是目录，整个删除
		os.RemoveAll(path)
	default:
		// agent/config 是单文件
		os.Remove(path)
	}
}

// isValidType 检查资源类型是否合法
// 参数 t: 类型字符串
// 返回: 是否合法
func isValidType(t string) bool {
	return t == "skill" || t == "agent" || t == "config" || t == "prompt"
}

// timeNow 获取当前时间的辅助函数（方便测试 mock）
var timeNow = func() time.Time {
	return time.Now()
}
