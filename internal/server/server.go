// Package server 提供 HTTP 服务的初始化和路由注册
package server

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/anthropic/airesourcemanager/internal/config"
	"github.com/anthropic/airesourcemanager/internal/handler"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Server HTTP 服务结构体，持有路由和依赖
type Server struct {
	router          *gin.Engine
	cfg             *config.Config
	healthHandler   *handler.HealthHandler
	wsHandler       *handler.WSHandler
	resourceHandler *handler.ResourceHandler
	groupHandler    *handler.GroupHandler
	aliasHandler    *handler.AliasHandler
	deployHandler   *handler.DeployHandler
	dataHandler     *handler.DataHandler
	logger          *zap.Logger
	staticFS        fs.FS // 嵌入的前端静态资源文件系统
}

// New 创建并配置 HTTP 服务实例
// 参数 cfg: 应用配置
// 参数 healthHandler: 健康检查处理器
// 参数 wsHandler: WebSocket 处理器
// 参数 resourceHandler: 资源处理器
// 参数 groupHandler: 分组处理器
// 参数 aliasHandler: 路径别名处理器
// 参数 deployHandler: 部署管理处理器
// 参数 dataHandler: 数据导入导出处理器
// 参数 logger: 日志实例
// 参数 staticFS: 前端静态资源文件系统，为 nil 时不提供静态文件服务
// 返回: 配置完成的 Server 指针
func New(cfg *config.Config, healthHandler *handler.HealthHandler, wsHandler *handler.WSHandler, resourceHandler *handler.ResourceHandler, groupHandler *handler.GroupHandler, aliasHandler *handler.AliasHandler, deployHandler *handler.DeployHandler, dataHandler *handler.DataHandler, logger *zap.Logger, staticFS fs.FS) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(CORSMiddleware())
	router.Use(LoggerMiddleware(logger))

	s := &Server{
		router:          router,
		cfg:             cfg,
		healthHandler:   healthHandler,
		wsHandler:       wsHandler,
		resourceHandler: resourceHandler,
		groupHandler:    groupHandler,
		aliasHandler:    aliasHandler,
		deployHandler:   deployHandler,
		dataHandler:     dataHandler,
		logger:          logger,
		staticFS:        staticFS,
	}

	s.registerRoutes()
	s.setupStaticFiles()
	return s
}

// Router 返回 gin.Engine 实例，用于 HTTP 服务启动
func (s *Server) Router() *gin.Engine {
	return s.router
}

// registerRoutes 注册所有 API 路由
func (s *Server) registerRoutes() {
	v1 := s.router.Group("/api/v1")
	{
		v1.GET("/health", s.healthHandler.Health)
		v1.GET("/ws", s.wsHandler.HandleWS)

		// 资源路由
		handler.RegisterResourceRoutes(v1, s.resourceHandler)

		// 分组路由
		handler.RegisterGroupRoutes(v1, s.groupHandler)

		// 路径别名路由
		handler.RegisterAliasRoutes(v1, s.aliasHandler)

		// 部署管理路由
		handler.RegisterDeployRoutes(v1, s.deployHandler)

		// 数据导入导出路由
		handler.RegisterDataRoutes(v1, s.dataHandler)
	}
}

// setupStaticFiles 配置前端静态文件服务和 SPA 回退
// 非 /api 路由优先尝试返回静态文件，匹配不到则回退到 index.html
func (s *Server) setupStaticFiles() {
	if s.staticFS == nil {
		return
	}

	fileServer := http.FS(s.staticFS)

	s.router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// /api 开头的路由不走静态文件服务
		if strings.HasPrefix(path, "/api") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		// 尝试直接服务请求的文件
		f, err := fileServer.Open(path)
		if err == nil {
			f.Close()
			c.FileFromFS(path, fileServer)
			return
		}

		// SPA 回退：返回 index.html
		c.FileFromFS("/index.html", fileServer)
	})
}
