// Package cmd serve.go 定义 serve 子命令，负责启动 HTTP 服务
// 串联整个启动流程：配置 → 数据库 → 迁移 → Hub → 服务 → 优雅关闭
package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/anthropic/airesourcemanager/internal/banner"
	"github.com/anthropic/airesourcemanager/internal/config"
	"github.com/anthropic/airesourcemanager/internal/handler"
	"github.com/anthropic/airesourcemanager/internal/repo"
	"github.com/anthropic/airesourcemanager/internal/server"
	"github.com/anthropic/airesourcemanager/internal/service"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// EmbeddedFS 前端静态资源文件系统，由 main 包在启动前设置
var EmbeddedFS fs.FS

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "启动 HTTP 服务",
	RunE:  runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().Bool("no-open", false, "跳过自动打开浏览器")
}

// runServe 启动服务的核心逻辑
// 按顺序初始化各组件，并处理优雅关闭
func runServe(cmd *cobra.Command, args []string) error {
	// 初始化日志
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	sugar := logger.Sugar()

	// 加载配置
	port, _ := cmd.Flags().GetInt("port")
	cfgPath, _ := cmd.Flags().GetString("config")
	noOpen, _ := cmd.Flags().GetBool("no-open")

	cfg, err := config.Load(cfgPath)
	if err != nil {
		sugar.Fatalf("加载配置失败: %v", err)
	}

	// 命令行端口优先级高于配置文件
	if cmd.Flags().Changed("port") {
		cfg.Port = port
	}

	// 打印启动横幅
	banner.Print(cfg.Port, cfg.DBPath, "info")

	sugar.Infof("使用配置: port=%d, db=%s", cfg.Port, cfg.DBPath)

	// 初始化数据库
	database, err := repo.NewDB(cfg.DBPath)
	if err != nil {
		sugar.Fatalf("初始化数据库失败: %v", err)
	}

	// 执行迁移
	if err := repo.RunMigrations(database); err != nil {
		sugar.Fatalf("数据库迁移失败: %v", err)
	}
	sugar.Info("数据库迁移完成")

	// 启动 WebSocket Hub
	hub := service.NewHub(logger)
	go hub.Run()

	// 获取资源根目录
	homeDir, err := os.UserHomeDir()
	if err != nil {
		sugar.Fatalf("获取主目录失败: %v", err)
	}
	baseDir := filepath.Join(homeDir, ".aiManager")

	// 创建资源服务
	resourceRepo := repo.NewResourceRepo(database)
	resourceSvc := service.NewResourceService(resourceRepo, baseDir)
	resourceHandler := handler.NewResourceHandler(resourceSvc)

	// 创建路径别名服务
	aliasRepo := repo.NewAliasRepo(database)
	aliasSvc := service.NewAliasService(aliasRepo)

	// 创建分组服务
	groupRepo := repo.NewGroupRepo(database)
	groupSvc := service.NewGroupService(groupRepo, hub, logger)
	groupHandler := handler.NewGroupHandler(groupSvc)

	// 创建部署服务
	deployRepo := repo.NewDeployRepo(database)
	deploySvc := service.NewDeployService(deployRepo, resourceRepo, aliasRepo, groupRepo, baseDir)
	deployHandler := handler.NewDeployHandler(deploySvc)

	// 别名处理器需要部署服务来还原 _meta.json
	aliasHandler := handler.NewAliasHandler(aliasSvc, deploySvc)

	// 注入部署服务到分组服务（追踪联动）
	groupSvc.SetDeployService(deploySvc)
	// 注入部署服务到资源服务（删除资源时级联撤销部署）
	resourceSvc.SetDeployService(deploySvc)

	// 创建数据导入导出服务
	dataSvc := service.NewDataService(resourceRepo, groupRepo, aliasRepo, baseDir, cfg.DBPath)
	dataHandler := handler.NewDataHandler(dataSvc)

	// === Preset / PathGroup 模块 ===
	presetRepo := repo.NewPresetRepo(database)
	pathGroupRepo := repo.NewPathGroupRepo(database)

	// 注入 presetRepo 到 deploy/resource 服务（用于 preset 部署同步、删除拦截 1005）
	deploySvc.SetPresetRepo(presetRepo)
	deploySvc.SetPathGroupRepo(pathGroupRepo)
	deploySvc.SetHub(hub)
	resourceSvc.SetPresetRepo(presetRepo)

	pathGroupSvc := service.NewPathGroupService(pathGroupRepo, hub)
	presetSvc := service.NewPresetService(presetRepo, resourceRepo, resourceSvc, deploySvc, hub, logger)

	pathGroupHandler := handler.NewPathGroupHandler(pathGroupSvc)
	presetHandler := handler.NewPresetHandler(presetSvc)

	// 创建处理器
	wsHandler := handler.NewWSHandler(hub, logger)
	healthHandler := handler.NewHealthHandler()

	// 创建并启动服务（传入嵌入的静态资源）
	srv := server.New(cfg, healthHandler, wsHandler, resourceHandler, groupHandler, aliasHandler, deployHandler, dataHandler, presetHandler, pathGroupHandler, logger, EmbeddedFS)
	addr := ":" + strconv.Itoa(cfg.Port)

	httpServer := &http.Server{
		Addr:    addr,
		Handler: srv.Router(),
	}

	// 启动 HTTP 服务（非阻塞）
	go func() {
		sugar.Infof("服务启动于 http://localhost%s", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			sugar.Fatalf("服务异常退出: %v", err)
		}
	}()

	// 自动打开浏览器
	if !noOpen {
		go func() {
			time.Sleep(500 * time.Millisecond)
			openBrowser(fmt.Sprintf("http://localhost:%d", cfg.Port))
		}()
	}

	// 启动文件监听服务
	watcherSvc := service.NewWatcherService(hub, resourceRepo, baseDir, logger)
	// 注入到 ResourceService,使主动删除时抑制 fsnotify 误报
	resourceSvc.SetWatcherService(watcherSvc)
	// 注入 presetRepo,使命中 preset 的资源变更时同时广播 preset:resource_changed
	watcherSvc.SetPresetRepo(presetRepo)
	if err := watcherSvc.Start(); err != nil {
		sugar.Warnf("文件监听服务启动失败: %v", err)
	}

	// 优雅关闭：等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	sugar.Info("收到关闭信号，开始优雅关闭...")

	// 1. 停止接受新 HTTP 请求
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 2. 关闭所有 WebSocket 连接
	hub.CloseAll()
	sugar.Info("WebSocket 连接已关闭")

	// 3. 等待进行中的请求完成（最多5秒）
	if err := httpServer.Shutdown(ctx); err != nil {
		sugar.Errorf("HTTP 服务关闭异常: %v", err)
	}
	sugar.Info("HTTP 服务已停止")

	// 4. 停止文件监听
	watcherSvc.Stop()
	sugar.Info("文件监听服务已关闭")

	// 5. 关闭 SQLite 连接
	database.Close()
	sugar.Info("数据库连接已关闭")

	// 6. 刷新日志缓冲
	logger.Sync()

	sugar.Info("服务已安全退出")
	return nil
}

// openBrowser 在默认浏览器中打开指定 URL
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	}
	if cmd != nil {
		cmd.Start()
	}
}
