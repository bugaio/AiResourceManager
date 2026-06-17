// Package handler resource.go 提供资源 CRUD 的 HTTP 接口处理
// 包括列表、创建、详情、更新、删除、批量删除、内容读写
package handler

import (
	"io"
	"strconv"

	"github.com/anthropic/airesourcemanager/internal/model"
	"github.com/anthropic/airesourcemanager/internal/service"
	"github.com/gin-gonic/gin"
)

// ResourceHandler 资源 HTTP 处理器
type ResourceHandler struct {
	svc *service.ResourceService
}

// NewResourceHandler 创建资源处理器实例
// 参数 svc: 资源业务服务
// 返回: ResourceHandler 指针
func NewResourceHandler(svc *service.ResourceService) *ResourceHandler {
	return &ResourceHandler{svc: svc}
}

// RegisterResourceRoutes 注册资源相关路由
// 参数 group: gin 路由组（/api/v1）
// 参数 h: 资源处理器
func RegisterResourceRoutes(group *gin.RouterGroup, h *ResourceHandler) {
	r := group.Group("/resources")
	{
		r.GET("", h.handleList)
		r.POST("", h.handleCreate)
		// import-skill / import-agent 必须在 :id 之前注册
		r.POST("/import-skill", h.handleImportSkill)
		r.POST("/import-agent", h.handleImportAgent)
		// batch 路由必须在 :id 之前注册，避免路由冲突
		r.DELETE("/batch", h.handleBatchDelete)
		r.GET("/:id", h.handleGet)
		r.PUT("/:id", h.handleUpdate)
		r.DELETE("/:id", h.handleDelete)
		r.GET("/:id/content", h.handleGetContent)
		r.PUT("/:id/content", h.handleUpdateContent)
	}
}

// handleList 处理资源列表查询
// 查询参数: type, search, group_id, page, page_size
func (h *ResourceHandler) handleList(c *gin.Context) {
	resourceType := c.Query("type")
	search := c.Query("search")
	groupID := c.DefaultQuery("group_id", "0")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	resp, err := h.svc.ListResources(resourceType, search, groupID, page, pageSize)
	if err != nil {
		handleBizError(c, err)
		return
	}

	Success(c, resp)
}

// handleCreate 处理资源创建
// 请求体: CreateResourceReq
func (h *ResourceHandler) handleCreate(c *gin.Context) {
	var req model.CreateResourceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, model.ErrParamValidation, "参数校验失败: "+err.Error())
		return
	}

	resource, err := h.svc.CreateResource(&req)
	if err != nil {
		handleBizError(c, err)
		return
	}

	Success(c, resource)
}

// handleGet 处理资源详情查询
// 路径参数: id
func (h *ResourceHandler) handleGet(c *gin.Context) {
	id := c.Param("id")
	resource, err := h.svc.GetResource(id)
	if err != nil {
		handleBizError(c, err)
		return
	}

	Success(c, resource)
}

// handleUpdate 处理资源元数据更新
// 路径参数: id
// 请求体: UpdateResourceReq
func (h *ResourceHandler) handleUpdate(c *gin.Context) {
	id := c.Param("id")
	var req model.UpdateResourceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, model.ErrParamValidation, "参数校验失败: "+err.Error())
		return
	}

	resource, err := h.svc.UpdateResource(id, &req)
	if err != nil {
		handleBizError(c, err)
		return
	}

	Success(c, resource)
}

// handleDelete 处理资源删除
// 路径参数: id
// 查询参数: confirm (true 时级联删除)
func (h *ResourceHandler) handleDelete(c *gin.Context) {
	id := c.Param("id")
	confirm := c.Query("confirm") == "true"

	data, err := h.svc.DeleteResource(id, confirm)
	if err != nil {
		if bizErr, ok := err.(*model.BizError); ok {
			ErrorWithData(c, bizErr.Code, bizErr.Msg, data)
			return
		}
		Error(c, model.ErrResourceFileIO, err.Error())
		return
	}

	Success(c, nil)
}

// handleBatchDelete 处理批量删除
// 请求体: BatchDeleteReq {ids: [], confirm: bool}
func (h *ResourceHandler) handleBatchDelete(c *gin.Context) {
	var req model.BatchDeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, model.ErrParamValidation, "参数校验失败: "+err.Error())
		return
	}

	if len(req.IDs) == 0 {
		Error(c, model.ErrParamValidation, "ids 不能为空")
		return
	}

	results, err := h.svc.BatchDelete(&req)
	if err != nil {
		handleBizError(c, err)
		return
	}

	Success(c, results)
}

// handleGetContent 处理资源文件内容读取
// 路径参数: id
func (h *ResourceHandler) handleGetContent(c *gin.Context) {
	id := c.Param("id")
	content, err := h.svc.GetContent(id)
	if err != nil {
		handleBizError(c, err)
		return
	}

	Success(c, map[string]string{"content": content})
}

// handleUpdateContent 处理资源文件内容更新
// 路径参数: id
// 请求体: UpdateContentReq
func (h *ResourceHandler) handleUpdateContent(c *gin.Context) {
	id := c.Param("id")
	var req model.UpdateContentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, model.ErrParamValidation, "参数校验失败: "+err.Error())
		return
	}

	if err := h.svc.UpdateContent(id, req.Content); err != nil {
		handleBizError(c, err)
		return
	}

	Success(c, nil)
}

// handleImportSkill 处理 skill 目录整体导入
// multipart/form-data 字段:
//   name        — 从源 SKILL.md frontmatter 解析的名称(必填)
//   description — 从源 SKILL.md frontmatter 解析的描述(可选)
//   group_id    — 关联分组 ID(可选)
//   paths       — 文件相对路径数组(与 files 同序,逐个对应)
//   files       — 文件数组
func (h *ResourceHandler) handleImportSkill(c *gin.Context) {
	name := c.PostForm("name")
	description := c.PostForm("description")
	groupID := c.PostForm("group_id")
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

	resource, err := h.svc.ImportSkill(name, description, groupID, imported)
	if err != nil {
		handleBizError(c, err)
		return
	}
	Success(c, resource)
}

// handleImportAgent 处理 agent 单文件导入
// multipart/form-data 字段:
//   name        — 从源 .md frontmatter 解析的名称(必填)
//   description — 从源 .md frontmatter 解析的描述(可选)
//   group_id    — 关联分组 ID(可选)
//   file        — 源 .md 文件(单文件)
func (h *ResourceHandler) handleImportAgent(c *gin.Context) {
	name := c.PostForm("name")
	description := c.PostForm("description")
	groupID := c.PostForm("group_id")

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

	resource, err := h.svc.ImportAgent(name, description, groupID, data)
	if err != nil {
		handleBizError(c, err)
		return
	}
	Success(c, resource)
}

// handleBizError 统一处理业务错误响应
// 参数 c: gin 上下文
// 参数 err: 错误
func handleBizError(c *gin.Context, err error) {
	if bizErr, ok := err.(*model.BizError); ok {
		if bizErr.Data != nil {
			ErrorWithData(c, bizErr.Code, bizErr.Msg, bizErr.Data)
			return
		}
		Error(c, bizErr.Code, bizErr.Msg)
		return
	}
	Error(c, model.ErrResourceFileIO, err.Error())
}
