# 任务10: 集成构建与收尾

## 目标

完成 Go embed 打包、CLI 完善、优雅退出（含WebSocket关闭）、路径别名管理页面、批量操作完善、整体联调和修复。生产可用的单二进制。

## 交付内容

### 后端
- `embed.go` — 完善 go:embed 嵌入 web/dist
- `cmd/serve.go` — 完善 serve 命令：--port、--no-open flag，自动打开浏览器逻辑
- `cmd/root.go` — version 子命令
- 优雅退出：SIGINT/SIGTERM → 停止接收请求 → 关闭所有 WebSocket 连接 → 等待完成(5s超时) → 关watcher → 关DB → 刷日志
- 静态文件服务：非 /api 路由 fallback 到 index.html（SPA支持）

### 前端
- `src/views/AliasView.vue` — 路径别名管理页面完整实现：
  - 表格展示（别名、路径、操作）
  - 搜索筛选
  - 新增弹窗（别名+路径）
  - 编辑弹窗（修改别名名称或路径）
  - 删除确认
- 批量操作完善：
  - 批量部署到目录（含 track 选项）
  - 批量移动到分组
  - 批量删除（检查关联 → 展示关联信息 → 二次确认 → 级联删除）
- WebSocket 连接完善：
  - App.vue onMounted 建立连接
  - 连接状态指示器（可选，TopBar角落小圆点）
  - 自动重连逻辑稳定（指数退避 1s→2s→4s→8s→30s max）
- 整体UI打磨：
  - 加载状态 loading
  - 操作反馈 toast/message
  - 表单校验提示
  - 确认删除弹窗统一风格
  - 暗色主题下所有组件样式检查

### 构建脚本
- `Makefile`：
  - `make dev` — 同时启动前后端开发模式
  - `make build` — 前端构建 + Go编译 → ./aimanager
  - `make clean` — 清理产物

### 联调检查清单
- [ ] 从零启动 → 创建资源 → 分组 → 部署 → 撤销，完整流程通
- [ ] Track 模式：分组部署(track=true) → 新增资源到分组 → 自动部署 → 移除资源 → 自动撤销
- [ ] 删除有关联部署的资源 → 展示关联信息 → 确认后级联清理
- [ ] 暗色主题下所有页面正常
- [ ] 别名管理页 CRUD 正常
- [ ] 数据导出后删库，导入恢复正常
- [ ] 外部修改文件 → WebSocket 推送 → 前端自动刷新
- [ ] 单二进制启动，访问前端页面正常
- [ ] WebSocket 断开后自动重连
- [ ] 分页在各列表页正常工作

## 验收标准

1. `make build` 生成单个 `aimanager` 二进制文件
2. `./aimanager serve` 启动后浏览器自动打开，页面正常加载
3. `./aimanager serve --no-open --port 9999` 参数生效
4. Ctrl+C 优雅退出，WebSocket 连接正常关闭，无报错
5. 完整业务流程通（创建→分组→部署(track)→编辑→自动同步→撤销→删除）
6. 路径别名管理页 CRUD 正常
7. 暗色主题下无样式问题
8. 生产二进制大小合理（<30MB）
9. WebSocket 实时推送功能正常
10. 分页功能全链路正常
