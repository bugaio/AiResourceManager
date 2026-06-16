// Package handler data.go 提供数据导入导出的 HTTP 接口处理
package handler

import (
	"github.com/anthropic/airesourcemanager/internal/model"
	"github.com/anthropic/airesourcemanager/internal/service"
	"github.com/anthropic/airesourcemanager/internal/util"
	"github.com/gin-gonic/gin"
)

// DataHandler 数据导入导出处理器
type DataHandler struct {
	svc *service.DataService
}

// NewDataHandler 创建数据处理器实例
// 参数 svc: 数据服务
// 返回: DataHandler 指针
func NewDataHandler(svc *service.DataService) *DataHandler {
	return &DataHandler{svc: svc}
}

// RegisterDataRoutes 注册数据导入导出路由
// POST /api/v1/data/export — 导出数据到指定目录
// POST /api/v1/data/import — 从指定目录导入数据
func RegisterDataRoutes(group *gin.RouterGroup, h *DataHandler) {
	r := group.Group("/data")
	{
		r.POST("/export", h.handleExport)
		r.POST("/import", h.handleImport)
	}
}

// handleExport 处理数据导出请求
// 请求体: ExportRequest {target_path}
func (h *DataHandler) handleExport(c *gin.Context) {
	var req model.ExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, model.ErrParamValidation, "参数校验失败: "+err.Error())
		return
	}

	// 展开路径中的 ~
	targetPath, err := util.ExpandPath(req.TargetPath)
	if err != nil {
		Error(c, model.ErrParamValidation, "路径展开失败: "+err.Error())
		return
	}

	result, err := h.svc.Export(targetPath)
	if err != nil {
		handleBizError(c, err)
		return
	}

	Success(c, result)
}

// handleImport 处理数据导入请求
// 请求体: ImportRequest {source_path, strategy}
func (h *DataHandler) handleImport(c *gin.Context) {
	var req model.ImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, model.ErrParamValidation, "参数校验失败: "+err.Error())
		return
	}

	// 展开路径中的 ~
	sourcePath, err := util.ExpandPath(req.SourcePath)
	if err != nil {
		Error(c, model.ErrParamValidation, "路径展开失败: "+err.Error())
		return
	}

	result, err := h.svc.Import(sourcePath, req.Strategy)
	if err != nil {
		handleBizError(c, err)
		return
	}

	Success(c, result)
}
