// Package handler path_group.go 提供 PathGroup 模块的 HTTP 接口处理
package handler

import (
	"github.com/anthropic/airesourcemanager/internal/model"
	"github.com/anthropic/airesourcemanager/internal/service"
	"github.com/gin-gonic/gin"
)

// PathGroupHandler 路径组 HTTP 处理器
type PathGroupHandler struct {
	svc *service.PathGroupService
}

// NewPathGroupHandler 创建 PathGroupHandler
func NewPathGroupHandler(svc *service.PathGroupService) *PathGroupHandler {
	return &PathGroupHandler{svc: svc}
}

// RegisterPathGroupRoutes 注册 path-group 相关路由
func RegisterPathGroupRoutes(group *gin.RouterGroup, h *PathGroupHandler) {
	r := group.Group("/path-groups")
	{
		r.GET("", h.handleList)
		r.POST("", h.handleCreate)
		r.GET("/:id", h.handleGet)
		r.PUT("/:id", h.handleUpdate)
		r.DELETE("/:id", h.handleDelete)
	}
}

func (h *PathGroupHandler) handleList(c *gin.Context) {
	list, err := h.svc.ListPathGroups()
	if err != nil {
		handleBizError(c, err)
		return
	}
	Success(c, list)
}

func (h *PathGroupHandler) handleCreate(c *gin.Context) {
	var req model.CreatePathGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, model.ErrParamValidation, "参数校验失败: "+err.Error())
		return
	}
	g, err := h.svc.CreatePathGroup(&req)
	if err != nil {
		handleBizError(c, err)
		return
	}
	Success(c, g)
}

func (h *PathGroupHandler) handleGet(c *gin.Context) {
	id := c.Param("id")
	g, err := h.svc.GetPathGroup(id)
	if err != nil {
		handleBizError(c, err)
		return
	}
	Success(c, g)
}

func (h *PathGroupHandler) handleUpdate(c *gin.Context) {
	id := c.Param("id")
	var req model.UpdatePathGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, model.ErrParamValidation, "参数校验失败: "+err.Error())
		return
	}
	g, err := h.svc.UpdatePathGroup(id, &req)
	if err != nil {
		handleBizError(c, err)
		return
	}
	Success(c, g)
}

func (h *PathGroupHandler) handleDelete(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.DeletePathGroup(id); err != nil {
		handleBizError(c, err)
		return
	}
	Success(c, nil)
}
