# 任务01: 项目初始化与基础骨架

## 目标

搭建前后端项目结构，完成所有基础设施：Go模块、Gin服务器、SQLite连接与迁移、Viper配置、zap日志、cobra CLI、WebSocket Hub骨架、Vue项目初始化（含Tailwind CSS）。启动后能跑通一个健康检查接口和 WebSocket 连接，前端能显示空白页面。

## 交付内容

### 后端
- `main.go` + `cmd/root.go` + `cmd/serve.go`（cobra CLI，serve子命令启动HTTP）
- `internal/config/config.go`（Viper加载配置，跨平台路径 os.UserHomeDir()）
- `internal/server/server.go`（Gin引擎 + 路由注册骨架 + 静态文件服务 + WebSocket路由）
- `internal/server/middleware.go`（CORS、请求日志、Recovery）
- `internal/repo/db.go`（SQLite连接 + 读写锁封装 + WAL模式）
- `internal/repo/migration.go` + `migrations/001_init.sql`（启动自动建表，含 track 字段）
- `internal/handler/response.go`（统一响应 {code, msg, data}，含分页响应结构）
- `internal/handler/ws.go`（WebSocket 升级 handler）
- `internal/service/hub.go`（WebSocket Hub：连接管理 + 广播 + 心跳）
- `internal/model/errors.go`（错误码定义 1000-9999）
- `internal/util/uuid.go`（crypto/rand UUID生成）
- `internal/util/file.go`（跨平台路径工具：HomeDir、路径展开、确保目录存在）
- `internal/util/symlink.go`（跨平台软链接：Unix symlink / Windows junction+mklink）
- `embed.go`（go:embed 占位）
- `go.mod` / `go.sum`
- `configs/config.yaml`（默认配置模板，含中文注释说明每个配置项）

### config.yaml 模板（首次启动自动生成到 {HOME}/.aiManager/config.yaml）

```yaml
# AiResourceManager 配置文件
# 修改后需重启服务生效

server:
  # 服务监听端口
  port: 3678
  # 启动时是否自动打开浏览器
  open_browser: true

log:
  # 日志级别: debug / info / warn / error
  level: info
  # 日志输出方式: stdout(终端) / file(文件) / both(两者)
  output: stdout
  # 日志文件路径（output包含file时生效）
  file_path: ~/.aiManager/logs/app.log

database:
  # 数据库文件路径
  path: ~/.aiManager/data/aimanager.db
```

### 前端
- `web/` 目录：Vite + Vue3 + TS + Element Plus + Pinia + Vue Router + Tailwind CSS 3.x
- `vite.config.ts`（API proxy + WebSocket proxy 到 :3678）
- `tailwind.config.js`（darkMode: 'class'，preflight: false，自定义sidebar色）
- `postcss.config.js`（tailwindcss + autoprefixer）
- `src/styles/tailwind.css`（@tailwind base/components/utilities）
- `src/App.vue`（空壳布局）
- `src/router/index.ts`（三条路由占位）
- `src/api/request.ts`（axios实例 + 拦截器）
- `src/api/ws.ts`（WebSocket 连接管理骨架：连接/断开/自动重连/心跳）

### 验证
- `go run . serve` 启动成功，监听3678端口
- 首次启动自动创建 `{HOME}/.aiManager/` 目录结构、数据库、config.yaml
- `GET /api/v1/health` 返回 `{code: 0, msg: "ok", data: null}`
- `GET /api/v1/ws` WebSocket 握手成功
- 6张表已建好（含 deployment.track 字段）
- `cd web && npm run dev` 前端启动成功，Tailwind 样式生效

## 验收标准

1. `go build .` 编译通过无error
2. 启动服务后 curl health 接口正常返回
3. WebSocket 连接能建立并收到心跳
4. SQLite 文件存在且表结构正确（含 track 字段）
5. 首次启动生成 config.yaml 含中文注释
6. 跨平台路径工具在当前 OS 下工作正常
7. 前端 dev server 启动无报错，Tailwind class 生效
8. 代码每个公开函数有中文注释
