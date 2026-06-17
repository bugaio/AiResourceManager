// Package handler alias.go 提供路径别名 CRUD 的 HTTP 接口处理
// 包括列表、创建、更新、删除
package handler

import (
	"github.com/anthropic/airesourcemanager/internal/model"
	"github.com/anthropic/airesourcemanager/internal/service"
	"github.com/gin-gonic/gin"
)

// AliasHandler 路径别名 HTTP 处理器
type AliasHandler struct {
	svc       *service.AliasService
	deploySvc *service.DeployService
}

// NewAliasHandler 创建路径别名处理器实例
// 参数 svc: 路径别名业务服务
// 参数 deploySvc: 部署服务（用于还原 _meta.json 关联）
// 返回: AliasHandler 指针
func NewAliasHandler(svc *service.AliasService, deploySvc *service.DeployService) *AliasHandler {
	return &AliasHandler{svc: svc, deploySvc: deploySvc}
}

// RegisterAliasRoutes 注册路径别名相关路由
// 参数 group: gin 路由组（/api/v1）
// 参数 h: 路径别名处理器
func RegisterAliasRoutes(group *gin.RouterGroup, h *AliasHandler) {
	r := group.Group("/aliases")
	{
		r.GET("", h.handleList)
		r.POST("", h.handleCreate)
		r.PUT("/:id", h.handleUpdate)
		r.DELETE("/:id", h.handleDelete)
	}
}

// handleList 处理路径别名列表查询
// 查询参数: type（可选，按资源类型 skill/agent/config 过滤）
func (h *AliasHandler) handleList(c *gin.Context) {
	aliasType := c.Query("type")
	list, err := h.svc.ListAliases(aliasType)
	if err != nil {
		handleBizError(c, err)
		return
	}

	Success(c, list)
}

// handleCreate 处理路径别名创建
// 请求体: CreateAliasReq
// 创建别名后自动检测 .aiResource/_meta.json，还原部署关联
func (h *AliasHandler) handleCreate(c *gin.Context) {
	var req model.CreateAliasReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, model.ErrParamValidation, "参数校验失败: "+err.Error())
		return
	}

	alias, err := h.svc.CreateAlias(&req)
	if err != nil {
		handleBizError(c, err)
		return
	}

	// 创建后尝试从 _meta.json 还原部署记录
	if h.deploySvc != nil {
		h.deploySvc.RestoreFromMeta(alias.Path, alias.ID)
	}

	Success(c, alias)
}

// handleUpdate 处理路径别名更新
// 路径参数: id
// 请求体: UpdateAliasReq
func (h *AliasHandler) handleUpdate(c *gin.Context) {
	id := c.Param("id")
	var req model.UpdateAliasReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, model.ErrParamValidation, "参数校验失败: "+err.Error())
		return
	}

	if err := h.svc.UpdateAlias(id, &req); err != nil {
		handleBizError(c, err)
		return
	}

	Success(c, nil)
}

// handleDelete 处理路径别名删除
// 路径参数: id
func (h *AliasHandler) handleDelete(c *gin.Context) {
	id := c.Param("id")

	if err := h.svc.DeleteAlias(id); err != nil {
		handleBizError(c, err)
		return
	}

	Success(c, nil)
}
