# 项目目录功能划分

## 入口

```
main.go → cmd/root.go → cmd/serve.go
```

- `main.go`: 程序入口，将 `go:embed` 嵌入的前端静态资源传递给 cmd 包，调用 `cmd.Execute()`
- `cmd/root.go`: Cobra 根命令定义，注册全局 flag（`--port`, `--config`）
- `cmd/serve.go`: `serve` 子命令，串联整个启动流程：配置 → 数据库 → 迁移 → Hub → 服务层初始化 → 依赖注入 → HTTP 服务 → 文件监听 → 优雅关闭

## internal/ 目录结构

```
internal/
├── banner/         启动横幅（终端彩色输出）
├── config/         Viper 配置管理
├── handler/        Gin HTTP 处理器
│   ├── resource.go     资源 CRUD + 内容读写 + 批量删除
│   ├── deploy.go       部署/撤销/targets/health/repair/cleanItem/check-path/check-conflicts/open-folder/by-resource
│   ├── alias.go        路径别名 CRUD
│   ├── group.go        分组 CRUD + 资源关联
│   ├── data.go         数据导入导出
│   ├── ws.go           WebSocket 升级
│   └── response.go     统一响应工具（Success/Error/ErrorWithData）
├── model/          数据模型 + DTO + 错误码
│   ├── resource.go     Resource 实体 + CreateResourceReq/UpdateResourceReq/BatchDeleteReq 等
│   ├── deploy.go       Deployment/DeploymentItem/DeployRequest/MetaJSON/CheckConflictsReq 等
│   ├── alias.go        PathAlias + CreateAliasReq/UpdateAliasReq
│   ├── group.go        Group + CreateGroupReq/UpdateGroupReq/AddResourcesReq
│   ├── data.go         导入导出请求/响应结构
│   └── errors.go       BizError 类型 + 错误码常量（分段：1000资源/2000分组/3000部署/4000别名/5000数据/9000系统）
├── repo/           SQLite 数据仓库 + 迁移
│   ├── db.go           DB 结构体（sql.DB + sync.RWMutex）+ NewDB + WAL + 外键
│   ├── migration.go    go:embed 迁移 + _migrations 表 + RunMigrations
│   ├── resource.go     资源 CRUD SQL
│   ├── deploy.go       部署记录 + 明细 SQL
│   ├── alias.go        别名 CRUD SQL
│   ├── group.go        分组 CRUD + group_resource 关联 SQL
│   └── migrations/     迁移 SQL 文件
│       ├── 001_init.sql
│       ├── 002_group_enhancements.sql
│       ├── 003_deploy.sql
│       ├── 004_group_color.sql
│       └── 005_alias_type.sql
├── server/         HTTP 服务器 + 中间件
│   ├── server.go       Server 结构体 + New() + 路由注册 + SPA 静态文件
│   └── middleware.go   CORSMiddleware + LoggerMiddleware
├── service/        业务逻辑层
│   ├── resource.go     资源 CRUD + 文件创建/删除 + 级联
│   ├── deploy.go       部署核心（Deploy/Undeploy/Repair/CleanItem/CheckConflicts/RestoreFromMeta）
│   ├── alias.go        别名 CRUD + 路径展开
│   ├── group.go        分组 CRUD + 追踪部署联动
│   ├── data.go         数据导入导出
│   ├── hub.go          WebSocket Hub（广播/注册/注销）
│   └── watcher.go      fsnotify 文件监听（资源文件变化通知前端）
└── util/           工具函数
    ├── file.go         FileExists / IsDir / EnsureDir / ExpandPath
    ├── jsonc.go        ParseJSONC（去注释，依赖 tailscale/hujson）
    ├── merge.go        DeepMerge（lodash 式递归合并）
    ├── symlink.go      CreateSymlink
    └── uuid.go         NewUUID
```

## 技术栈

| 层      | 技术                         |
|---------|------------------------------|
| CLI     | Cobra + Viper                |
| HTTP    | Gin                          |
| 数据库  | SQLite (mattn/go-sqlite3)    |
| 迁移    | go:embed SQL 文件 + _migrations 表 |
| 并发    | sync.RWMutex 保护 SQLite 写入 |
| 日志    | zap                          |
| WebSocket | gorilla/websocket          |
| 文件监听 | fsnotify                    |
| JSONC   | tailscale/hujson             |
| 依赖注入 | 手动构造（非框架），serve.go 中按序 new |

## 关键约定

- Go 1.21+
- 端口: 3678（可通过 `--port` 或配置文件覆盖）
- 资源文件存储根: `~/.aiManager/`（skills/ agents/ mcps/ 三子目录）
- 数据库默认路径: `~/.aiManager/airesource.db`
- 前端静态资源通过 `go:embed` 嵌入二进制
