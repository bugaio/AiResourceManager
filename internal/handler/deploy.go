// Package handler deploy.go 提供部署管理的 HTTP 接口处理
// 包括执行部署、查看记录、撤销部署、健康检查、修复、清理
package handler

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/anthropic/airesourcemanager/internal/model"
	"github.com/anthropic/airesourcemanager/internal/service"
	"github.com/gin-gonic/gin"
)

// DeployHandler 部署管理 HTTP 处理器
type DeployHandler struct {
	svc *service.DeployService
}

// NewDeployHandler 创建部署处理器实例
// 参数 svc: 部署业务服务
// 返回: DeployHandler 指针
func NewDeployHandler(svc *service.DeployService) *DeployHandler {
	return &DeployHandler{svc: svc}
}

// RegisterDeployRoutes 注册部署相关路由
// 参数 group: gin 路由组（/api/v1）
// 参数 h: 部署处理器
func RegisterDeployRoutes(group *gin.RouterGroup, h *DeployHandler) {
	deployments := group.Group("/deployments")
	{
		deployments.POST("", h.handleDeploy)
		deployments.GET("", h.handleList)
		deployments.DELETE("/:id", h.handleUndeploy)
		deployments.GET("/targets", h.handleTargets)
		deployments.GET("/health", h.handleCheck)
		deployments.POST("/check-path", h.handleCheckPath)
		deployments.POST("/check-conflicts", h.handleCheckConflicts)
		deployments.POST("/summarize-paths", h.handleSummarizePaths)
		deployments.POST("/undeploy-paths", h.handleUndeployPaths)
		deployments.POST("/open-folder", h.handleOpenFolder)
		deployments.POST("/:id/repair", h.handleRepair)
		deployments.DELETE("/:id/items/:item_id", h.handleCleanItem)
		deployments.GET("/by-resource/:resourceId", h.handleResourceDeployTargets)
	}
}

// handleDeploy 处理执行部署请求
// 请求体: DeployRequest（group_id/resource_id 二选一，target_path/alias_id 二选一）
func (h *DeployHandler) handleDeploy(c *gin.Context) {
	var req model.DeployRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, model.ErrParamValidation, "参数校验失败: "+err.Error())
		return
	}

	// 校验: group_id、resource_id、resource_ids 不能全为空
	hasGroup := req.GroupID != nil && *req.GroupID != ""
	hasResource := req.ResourceID != nil && *req.ResourceID != ""
	hasResources := len(req.ResourceIDs) > 0
	if !hasGroup && !hasResource && !hasResources {
		Error(c, model.ErrDeployInvalid, "group_id、resource_id、resource_ids 不能全为空")
		return
	}

	// 校验: target_path 和 alias_id 不能同时为空
	if req.TargetPath == "" && (req.AliasID == nil || *req.AliasID == "") {
		Error(c, model.ErrDeployInvalid, "target_path 和 alias_id 不能同时为空")
		return
	}

	deployment, err := h.svc.Deploy(&req)
	if err != nil {
		handleBizError(c, err)
		return
	}

	Success(c, deployment)
}

// handleList 处理部署记录列表查询（分页）
// 查询参数: page, page_size
func (h *DeployHandler) handleList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	resp, err := h.svc.ListDeployments(page, pageSize)
	if err != nil {
		handleBizError(c, err)
		return
	}

	Success(c, resp)
}

// handleUndeploy 处理撤销部署请求
// 路径参数: id（部署记录 ID）
func (h *DeployHandler) handleUndeploy(c *gin.Context) {
	id := c.Param("id")

	if err := h.svc.Undeploy(id); err != nil {
		handleBizError(c, err)
		return
	}

	Success(c, nil)
}

// handleTargets 处理目标路径聚合查询
// 查询参数: type（可选，按资源类型 skill/agent/config/prompt 过滤）
func (h *DeployHandler) handleTargets(c *gin.Context) {
	resourceType := c.Query("type")
	targets, err := h.svc.GetTargets(resourceType)
	if err != nil {
		handleBizError(c, err)
		return
	}

	Success(c, targets)
}

// handleCheck 处理手动触发健康检查请求
func (h *DeployHandler) handleCheck(c *gin.Context) {
	brokenItems, err := h.svc.CheckHealth()
	if err != nil {
		handleBizError(c, err)
		return
	}

	Success(c, brokenItems)
}

// handleRepair 处理修复异常部署子项请求
// 路径参数: id（部署记录 ID）
// 请求体: RepairRequest（可选 item_id）
func (h *DeployHandler) handleRepair(c *gin.Context) {
	deploymentID := c.Param("id")

	var req model.RepairRequest
	// 允许空 body，item_id 可选
	_ = c.ShouldBindJSON(&req)

	if err := h.svc.RepairItem(deploymentID, req.ItemID); err != nil {
		handleBizError(c, err)
		return
	}

	Success(c, nil)
}

// handleCleanItem 处理清理/撤销部署子项请求
// 路径参数: id（部署记录 ID）, item_id（子项 ID）
// 查询参数: undeploy=true 时同时删除实际文件（symlink/json key）
func (h *DeployHandler) handleCleanItem(c *gin.Context) {
	itemID := c.Param("item_id")
	undeploy := c.Query("undeploy") == "true"

	if err := h.svc.CleanItem(itemID, undeploy); err != nil {
		handleBizError(c, err)
		return
	}

	Success(c, nil)
}

// handleCheckPath 检查给定路径是否存在于文件系统
// 请求体: {"path": "/some/path"}
// 返回: {"exists": true/false}
func (h *DeployHandler) handleCheckPath(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, model.ErrParamValidation, "参数校验失败: "+err.Error())
		return
	}

	_, err := os.Stat(req.Path)
	Success(c, gin.H{"exists": err == nil})
}

// handleSummarizePaths 统计给定目标路径下的部署内容（编辑路径组删 config 路径前用）
func (h *DeployHandler) handleSummarizePaths(c *gin.Context) {
	var req struct {
		Paths []string `json:"paths"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, model.ErrParamValidation, "参数校验失败: "+err.Error())
		return
	}
	summary, err := h.svc.SummarizeDeploymentsAtPaths(req.Paths)
	if err != nil {
		handleBizError(c, err)
		return
	}
	Success(c, summary)
}

// handleUndeployPaths 撤销给定目标路径下的所有部署（确认移除时用）
func (h *DeployHandler) handleUndeployPaths(c *gin.Context) {
	var req struct {
		Paths []string `json:"paths"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, model.ErrParamValidation, "参数校验失败: "+err.Error())
		return
	}
	count, err := h.svc.UndeployAtPaths(req.Paths)
	if err != nil {
		handleBizError(c, err)
		return
	}
	Success(c, gin.H{"undeployed": count})
}

// handleOpenFolder 在系统文件管理器中打开指定路径
// 若 path 是文件而非目录，打开其父目录并选中该文件（macOS -R / Windows /select）
// 跨平台：macOS=open, Linux=xdg-open, Windows=explorer
func (h *DeployHandler) handleOpenFolder(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, model.ErrParamValidation, "参数校验失败: "+err.Error())
		return
	}

	info, err := os.Stat(req.Path)
	if err != nil {
		Error(c, model.ErrDeployFailed, "路径不存在: "+req.Path)
		return
	}

	isFile := !info.IsDir()

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		if isFile {
			// -R: 在 Finder 中定位并选中该文件
			cmd = exec.Command("open", "-R", req.Path)
		} else {
			cmd = exec.Command("open", req.Path)
		}
	case "linux":
		if isFile {
			cmd = exec.Command("xdg-open", filepath.Dir(req.Path))
		} else {
			cmd = exec.Command("xdg-open", req.Path)
		}
	case "windows":
		if isFile {
			cmd = exec.Command("explorer", "/select,", req.Path)
		} else {
			cmd = exec.Command("explorer", req.Path)
		}
	default:
		Error(c, model.ErrDeployFailed, "不支持的操作系统: "+runtime.GOOS)
		return
	}

	if err := cmd.Start(); err != nil {
		Error(c, model.ErrDeployFailed, "打开文件夹失败: "+err.Error())
		return
	}

	Success(c, nil)
}

// handleResourceDeployTargets 获取某资源已部署到的所有目标路径（Config 保存后同步用）
// 路径参数: resourceId
func (h *DeployHandler) handleResourceDeployTargets(c *gin.Context) {
	resourceID := c.Param("resourceId")
	targets, err := h.svc.GetResourceDeployTargets(resourceID)
	if err != nil {
		handleBizError(c, err)
		return
	}
	Success(c, targets)
}

// handleCheckConflicts Config 批量部署预检冲突（不写入文件）
func (h *DeployHandler) handleCheckConflicts(c *gin.Context) {
	var req model.CheckConflictsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, model.ErrParamValidation, "参数校验失败: "+err.Error())
		return
	}
	resp, err := h.svc.CheckConflicts(&req)
	if err != nil {
		handleBizError(c, err)
		return
	}
	Success(c, resp)
}
