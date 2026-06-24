// Package handler preset.go 提供 Preset 模块的 HTTP 接口处理
package handler

import (
	"io"

	"github.com/anthropic/airesourcemanager/internal/model"
	"github.com/anthropic/airesourcemanager/internal/service"
	"github.com/gin-gonic/gin"
)

// PresetHandler Preset HTTP 处理器
type PresetHandler struct {
	svc *service.PresetService
}

// NewPresetHandler 创建 PresetHandler
func NewPresetHandler(svc *service.PresetService) *PresetHandler {
	return &PresetHandler{svc: svc}
}

// RegisterPresetRoutes 注册 preset 相关路由
func RegisterPresetRoutes(group *gin.RouterGroup, h *PresetHandler) {
	r := group.Group("/presets")
	{
		r.GET("", h.handleList)
		r.POST("", h.handleCreate)
		r.GET("/:id", h.handleGet)
		r.PUT("/:id", h.handleUpdate)
		r.DELETE("/:id", h.handleDelete)
		r.GET("/:id/resources", h.handleListResources)
		r.POST("/:id/resources", h.handleLinkResources)
		r.DELETE("/:id/resources", h.handleUnlinkResources)
		r.POST("/:id/check-config-conflicts", h.handleCheckConfigConflicts)
		r.POST("/:id/private-resources", h.handleCreatePrivateResource)
		r.DELETE("/:id/private-resources/:rid", h.handleDeletePrivateResource)
		r.POST("/:id/import-private-skill", h.handleImportPrivateSkill)
		r.POST("/:id/import-private-agent", h.handleImportPrivateAgent)
		r.POST("/:id/deploy", h.handleDeploy)
		r.POST("/:id/redeploy", h.handleRedeploy)
		r.DELETE("/:id/deploy/:deploymentID", h.handleUndeploy)
		r.GET("/:id/groups/:groupID/status", h.handleGroupStatus)
		r.POST("/:id/groups/:groupID/redeploy", h.handleRedeployGroup)
	}
}

// handleList
func (h *PresetHandler) handleList(c *gin.Context) {
	list, err := h.svc.ListPresets()
	if err != nil {
		handleBizError(c, err)
		return
	}
	Success(c, list)
}

// handleCreate
func (h *PresetHandler) handleCreate(c *gin.Context) {
	var req model.CreatePresetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, model.ErrParamValidation, "参数校验失败: "+err.Error())
		return
	}
	p, err := h.svc.CreatePreset(&req)
	if err != nil {
		handleBizError(c, err)
		return
	}
	Success(c, p)
}

// handleGet
func (h *PresetHandler) handleGet(c *gin.Context) {
	id := c.Param("id")
	p, err := h.svc.GetPreset(id)
	if err != nil {
		handleBizError(c, err)
		return
	}
	// 附带资源列表
	resources, _ := h.svc.ListPresetResources(id)
	// 附带部署记录
	deployments, _ := h.svc.ListPresetDeployments(id)
	Success(c, gin.H{
		"id":             p.ID,
		"name":           p.Name,
		"description":    p.Description,
		"created_at":     p.CreatedAt,
		"updated_at":     p.UpdatedAt,
		"resource_count": p.ResourceCount,
		"private_count":  p.PrivateCount,
		"linked_count":   p.LinkedCount,
		"resources":      resources,
		"deployments":    deployments,
	})
}

// handleUpdate
func (h *PresetHandler) handleUpdate(c *gin.Context) {
	id := c.Param("id")
	var req model.UpdatePresetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, model.ErrParamValidation, "参数校验失败: "+err.Error())
		return
	}
	p, err := h.svc.UpdatePreset(id, &req)
	if err != nil {
		handleBizError(c, err)
		return
	}
	Success(c, p)
}

// handleDelete
func (h *PresetHandler) handleDelete(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.DeletePreset(id); err != nil {
		handleBizError(c, err)
		return
	}
	Success(c, nil)
}

// handleListResources 获取 preset 下的资源列表（私有 + 关联）
func (h *PresetHandler) handleListResources(c *gin.Context) {
	id := c.Param("id")
	resources, err := h.svc.ListPresetResources(id)
	if err != nil {
		handleBizError(c, err)
		return
	}
	Success(c, resources)
}

// handleLinkResources
func (h *PresetHandler) handleLinkResources(c *gin.Context) {
	id := c.Param("id")
	var req model.LinkResourcesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, model.ErrParamValidation, "参数校验失败: "+err.Error())
		return
	}
	if err := h.svc.LinkResources(id, req.ResourceIDs); err != nil {
		handleBizError(c, err)
		return
	}
	Success(c, nil)
}

// handleUnlinkResources
func (h *PresetHandler) handleUnlinkResources(c *gin.Context) {
	id := c.Param("id")
	var req model.UnlinkResourcesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, model.ErrParamValidation, "参数校验失败: "+err.Error())
		return
	}
	if err := h.svc.UnlinkResources(id, req.ResourceIDs); err != nil {
		handleBizError(c, err)
		return
	}
	Success(c, nil)
}

// handleCheckConfigConflicts 检测候选 config 与 preset 已有 config 的冲突
func (h *PresetHandler) handleCheckConfigConflicts(c *gin.Context) {
	id := c.Param("id")
	var req model.CheckPresetConfigConflictsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, model.ErrParamValidation, "参数校验失败: "+err.Error())
		return
	}
	resp, err := h.svc.CheckPresetConfigConflicts(id, req.CandidateIDs)
	if err != nil {
		handleBizError(c, err)
		return
	}
	Success(c, resp)
}

// handleCreatePrivateResource 新增私有资源（普通表单字段）
func (h *PresetHandler) handleCreatePrivateResource(c *gin.Context) {
	id := c.Param("id")
	var req model.CreateResourceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, model.ErrParamValidation, "参数校验失败: "+err.Error())
		return
	}
	r, err := h.svc.CreatePrivateResource(id, &req)
	if err != nil {
		handleBizError(c, err)
		return
	}
	Success(c, r)
}

// handleDeletePrivateResource 删除 preset 下的单个私有资源
func (h *PresetHandler) handleDeletePrivateResource(c *gin.Context) {
	id := c.Param("id")
	rid := c.Param("rid")
	if err := h.svc.DeletePrivateResource(id, rid); err != nil {
		handleBizError(c, err)
		return
	}
	Success(c, nil)
}

// handleImportPrivateSkill 导入私有 skill (multipart)
func (h *PresetHandler) handleImportPrivateSkill(c *gin.Context) {
	id := c.Param("id")
	name := c.PostForm("name")
	description := c.PostForm("description")
	paths := c.PostFormArray("paths")

	form, err := c.MultipartForm()
	if err != nil {
		Error(c, model.ErrParamValidation, "解析 multipart 失败: "+err.Error())
		return
	}
	files := form.File["files"]
	if len(paths) != len(files) {
		Error(c, model.ErrParamValidation, "paths 与 files 长度不一致")
		return
	}
	if len(files) == 0 {
		Error(c, model.ErrParamValidation, "至少需要一个文件")
		return
	}

	imported := make([]service.ImportedSkillFile, 0, len(files))
	for i, fh := range files {
		f, err := fh.Open()
		if err != nil {
			Error(c, model.ErrResourceFileIO, "读取上传文件失败: "+err.Error())
			return
		}
		data, err := io.ReadAll(f)
		_ = f.Close()
		if err != nil {
			Error(c, model.ErrResourceFileIO, "读取上传内容失败: "+err.Error())
			return
		}
		imported = append(imported, service.ImportedSkillFile{
			RelPath: paths[i],
			Data:    data,
		})
	}

	r, err := h.svc.ImportPrivateSkill(id, name, description, imported)
	if err != nil {
		handleBizError(c, err)
		return
	}
	Success(c, r)
}

// handleImportPrivateAgent 导入私有 agent (multipart)
func (h *PresetHandler) handleImportPrivateAgent(c *gin.Context) {
	id := c.Param("id")
	name := c.PostForm("name")
	description := c.PostForm("description")

	fh, err := c.FormFile("file")
	if err != nil {
		Error(c, model.ErrParamValidation, "缺少 file 字段: "+err.Error())
		return
	}
	f, err := fh.Open()
	if err != nil {
		Error(c, model.ErrResourceFileIO, "读取上传文件失败: "+err.Error())
		return
	}
	data, err := io.ReadAll(f)
	_ = f.Close()
	if err != nil {
		Error(c, model.ErrResourceFileIO, "读取上传内容失败: "+err.Error())
		return
	}

	r, err := h.svc.ImportPrivateAgent(id, name, description, data)
	if err != nil {
		handleBizError(c, err)
		return
	}
	Success(c, r)
}

// handleDeploy 部署 preset
func (h *PresetHandler) handleDeploy(c *gin.Context) {
	id := c.Param("id")
	var req model.DeployPresetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, model.ErrParamValidation, "参数校验失败: "+err.Error())
		return
	}
	deployments, err := h.svc.DeployPreset(id, &req)
	if err != nil {
		handleBizError(c, err)
		return
	}
	Success(c, deployments)
}

// handleUndeploy 撤销某次 preset 部署
func (h *PresetHandler) handleUndeploy(c *gin.Context) {
	id := c.Param("id")
	deploymentID := c.Param("deploymentID")
	if err := h.svc.UndeployPresetDeployment(id, deploymentID); err != nil {
		handleBizError(c, err)
		return
	}
	Success(c, nil)
}

// handleRedeploy 重新部署整个 preset（复用已有 target_path）
func (h *PresetHandler) handleRedeploy(c *gin.Context) {
	id := c.Param("id")
	deployments, err := h.svc.RedeployPreset(id)
	if err != nil {
		handleBizError(c, err)
		return
	}
	Success(c, deployments)
}

// handleDeployResourceStatus 某部署目标下全量资源及部署状态（重新部署预览）
func (h *PresetHandler) handleGroupStatus(c *gin.Context) {
	id := c.Param("id")
	groupID := c.Param("groupID")
	status, err := h.svc.GetPresetGroupStatus(id, groupID)
	if err != nil {
		handleBizError(c, err)
		return
	}
	Success(c, status)
}

// handleRedeployGroup 将 preset 以最新全量资源重新部署到指定路径组（补齐新增类型）
func (h *PresetHandler) handleRedeployGroup(c *gin.Context) {
	id := c.Param("id")
	groupID := c.Param("groupID")
	deployments, err := h.svc.RedeployPresetGroup(id, groupID)
	if err != nil {
		handleBizError(c, err)
		return
	}
	Success(c, deployments)
}
