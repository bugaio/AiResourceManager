// Package service watcher.go 实现文件系统监听服务
// 监听 ~/.aiManager/skills/、agents/、configs/、prompts/ 目录变化
// 通过 WebSocket Hub 推送事件到前端
package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/anthropic/airesourcemanager/internal/repo"
	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

// WatcherService 文件监听服务
// 监听资源目录变化，通过 WebSocket 推送事件
type WatcherService struct {
	watcher    *fsnotify.Watcher
	hub        *Hub
	repo       *repo.ResourceRepo
	baseDir    string
	lastEvents map[string]time.Time // 去重: path → 最后处理时间
	suppressed map[string]time.Time // 抑制: uuid → 抑制截止时间(程序自己发起的删除)
	mu         sync.Mutex
	stopCh     chan struct{}
	logger     *zap.Logger
}

// WatcherEvent WebSocket 推送的文件变更事件
type WatcherEvent struct {
	Type string      `json:"type"` // resource:updated / resource:deleted
	Data interface{} `json:"data"`
}

// NewWatcherService 创建文件监听服务实例
// 参数 hub: WebSocket 广播中心
// 参数 resourceRepo: 资源仓库（更新 updated_at）
// 参数 baseDir: 资源根目录 (~/.aiManager)
// 参数 logger: 日志实例
// 返回: WatcherService 指针
func NewWatcherService(hub *Hub, resourceRepo *repo.ResourceRepo, baseDir string, logger *zap.Logger) *WatcherService {
	return &WatcherService{
		hub:        hub,
		repo:       resourceRepo,
		baseDir:    baseDir,
		lastEvents: make(map[string]time.Time),
		suppressed: make(map[string]time.Time),
		stopCh:     make(chan struct{}),
		logger:     logger,
	}
}

// SuppressUUID 抑制指定 UUID 在接下来 ttl 时间内产生的 deleted 广播
// 用于 ResourceService.DeleteResource 程序主动删文件时,避免被 fsnotify 误报为"外部删除"
func (w *WatcherService) SuppressUUID(uuid string, ttl time.Duration) {
	if uuid == "" {
		return
	}
	w.mu.Lock()
	w.suppressed[uuid] = time.Now().Add(ttl)
	w.mu.Unlock()
}

// isSuppressed 检查 UUID 是否仍在抑制窗口内,顺便清理过期项
func (w *WatcherService) isSuppressed(uuid string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	deadline, ok := w.suppressed[uuid]
	if !ok {
		return false
	}
	if time.Now().After(deadline) {
		delete(w.suppressed, uuid)
		return false
	}
	return true
}

// Start 启动文件监听
// 创建 fsnotify.Watcher 并添加监听目录，启动事件处理协程
// 返回: 启动过程中的错误
func (w *WatcherService) Start() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	w.watcher = watcher

	// 需要监听的目录
	watchDirs := []string{
		filepath.Join(w.baseDir, "skills"),
		filepath.Join(w.baseDir, "agents"),
		filepath.Join(w.baseDir, "configs"),
		filepath.Join(w.baseDir, "prompts"),
	}

	for _, dir := range watchDirs {
		// 确保目录存在
		if err := os.MkdirAll(dir, 0755); err != nil {
			w.logger.Error("创建监听目录失败", zap.String("dir", dir), zap.Error(err))
			continue
		}

		// 添加目录本身
		if err := watcher.Add(dir); err != nil {
			w.logger.Error("添加监听目录失败", zap.String("dir", dir), zap.Error(err))
			continue
		}

		// 递归添加子目录（主要用于 skills/ 下的 UUID 目录）
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() && path != dir {
				if addErr := watcher.Add(path); addErr != nil {
					w.logger.Warn("添加子目录监听失败", zap.String("path", path), zap.Error(addErr))
				}
			}
			return nil
		})
	}

	// 启动事件处理协程
	go w.processEvents()

	w.logger.Info("文件监听服务已启动", zap.String("baseDir", w.baseDir))
	return nil
}

// Stop 停止文件监听
func (w *WatcherService) Stop() {
	close(w.stopCh)
	if w.watcher != nil {
		w.watcher.Close()
	}
	w.logger.Info("文件监听服务已停止")
}

// processEvents 事件处理主循环
func (w *WatcherService) processEvents() {
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			w.logger.Error("文件监听错误", zap.Error(err))
		case <-w.stopCh:
			return
		}
	}
}

// handleEvent 处理单个文件事件
func (w *WatcherService) handleEvent(event fsnotify.Event) {
	path := event.Name

	// 新建目录时始终加入监听（无论是否能提取 UUID）
	if event.Has(fsnotify.Create) {
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			w.watcher.Add(path)
			w.logger.Info("新目录已加入监听", zap.String("path", path))
		}
	}

	// 去重检查：500ms 内同路径事件只处理一次
	w.mu.Lock()
	if last, exists := w.lastEvents[path]; exists {
		if time.Since(last) < 500*time.Millisecond {
			w.mu.Unlock()
			return
		}
	}
	w.lastEvents[path] = time.Now()
	w.mu.Unlock()

	// 从路径提取 UUID
	uuid := extractUUIDFromPath(path, w.baseDir)
	if uuid == "" {
		return
	}

	// 查找数据库中对应的资源
	resource, err := w.repo.GetResourceByID(uuid)
	if err != nil || resource == nil {
		return
	}

	// 根据事件类型处理
	if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
		// 文件被修改或创建
		now := time.Now()
		nowStr := now.Format(time.RFC3339)
		w.repo.TouchResource(uuid) // 触发 updated_at 更新

		msg := WatcherEvent{
			Type: "resource:updated",
			Data: map[string]interface{}{
				"id":         resource.ID,
				"uuid":       uuid,
				"updated_at": nowStr,
			},
		}
		w.broadcast(msg)

	} else if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
		// 文件被删除或重命名 — 若是程序自己发起的删除,跳过广播(防止误报)
		if w.isSuppressed(uuid) {
			w.logger.Debug("跳过被抑制的 deleted 广播", zap.String("uuid", uuid))
			return
		}
		w.logger.Warn("资源文件被删除", zap.String("uuid", uuid), zap.String("path", path))

		msg := WatcherEvent{
			Type: "resource:deleted",
			Data: map[string]interface{}{
				"id":   resource.ID,
				"uuid": uuid,
			},
		}
		w.broadcast(msg)
	}
}

// broadcast 向所有 WebSocket 客户端广播事件
func (w *WatcherService) broadcast(event WatcherEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		w.logger.Error("序列化监听事件失败", zap.Error(err))
		return
	}
	w.hub.Broadcast(data)
}
