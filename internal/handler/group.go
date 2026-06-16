// Package handler group.go 提供分组 CRUD 的 HTTP 接口处理
// 包括列表、创建、更新、删除、资源关联操作
package handler

import (
	"strconv"

	"github.com/anthropic/airesourcemanager/internal/model"
	"github.com/anthropic/airesourcemanager/internal/service"
	"github.com/gin-gonic/gin"
)

// GroupHandler 分组 HTTP 处理器
type GroupHandler struct {
	svc *service.GroupService
}

// NewGroupHandler 创建分组处理器实例
// 参数 svc: 分组业务服务
// 返回: GroupHandler 指针
func NewGroupHandler(svc *service.GroupService) *GroupHandler {
	return &GroupHandler{svc: svc}
}

// RegisterGroupRoutes 注册分组相关路由
// 参数 group: gin 路由组（/api/v1）
// 参数 h: 分组处理器
func RegisterGroupRoutes(group *gin.RouterGroup, h *GroupHandler) {
	r := group.Group("/groups")
	{
		r.GET("", h.handleList)
		r.POST("", h.handleCreate)
		r.PUT("/:id", h.handleUpdate)
		r.DELETE("/:id", h.handleDelete)
		r.POST("/:id/resources", h.handleAddResources)
		r.DELETE("/:id/resources/:rid", h.handleRemoveResource)
	}
}

// handleList 处理分组列表查询
// 查询参数: type, page, page_size
func (h *GroupHandler) handleList(c *gin.Context) {
	groupType := c.Query("type")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	resp, err := h.svc.ListGroups(groupType, page, pageSize)
	if err != nil {
		handleBizError(c, err)
		return
	}

	Success(c, resp)
}

// handleCreate 处理分组创建
// 请求体: CreateGroupReq
func (h *GroupHandler) handleCreate(c *gin.Context) {
	var req model.CreateGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, model.ErrParamValidation, "参数校验失败: "+err.Error())
		return
	}

	group, err := h.svc.CreateGroup(&req)
	if err != nil {
		handleBizError(c, err)
		return
	}

	Success(c, group)
}

// handleUpdate 处理分组更新
// 路径参数: id
// 请求体: UpdateGroupReq
func (h *GroupHandler) handleUpdate(c *gin.Context) {
	id := c.Param("id")
	var req model.UpdateGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, model.ErrParamValidation, "参数校验失败: "+err.Error())
		return
	}

	if err := h.svc.UpdateGroup(id, &req); err != nil {
		handleBizError(c, err)
		return
	}

	Success(c, nil)
}

// handleDelete 处理分组删除
// 路径参数: id
func (h *GroupHandler) handleDelete(c *gin.Context) {
	id := c.Param("id")

	if err := h.svc.DeleteGroup(id); err != nil {
		handleBizError(c, err)
		return
	}

	Success(c, nil)
}

// handleAddResources 处理向分组添加资源
// 路径参数: id
// 请求体: AddResourcesReq
func (h *GroupHandler) handleAddResources(c *gin.Context) {
	groupID := c.Param("id")
	var req model.AddResourcesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, model.ErrParamValidation, "参数校验失败: "+err.Error())
		return
	}

	if len(req.ResourceIDs) == 0 {
		Error(c, model.ErrParamValidation, "resource_ids 不能为空")
		return
	}

	if err := h.svc.AddResources(groupID, req.ResourceIDs); err != nil {
		handleBizError(c, err)
		return
	}

	Success(c, nil)
}

// handleRemoveResource 处理从分组移除资源
// 路径参数: id（分组 ID）, rid（资源 ID）
func (h *GroupHandler) handleRemoveResource(c *gin.Context) {
	groupID := c.Param("id")
	resourceID := c.Param("rid")

	if err := h.svc.RemoveResource(groupID, resourceID); err != nil {
		handleBizError(c, err)
		return
	}

	Success(c, nil)
}
