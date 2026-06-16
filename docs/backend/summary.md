# 总结

## 架构: handler - service - repo 三层

```
HTTP Request
    |
handler (参数解析 + 校验 + 统一响应)
    |
service (业务逻辑 + 文件系统操作)
    |
repo (SQL 读写 + 锁)
    |
SQLite (WAL + 外键)
```

handler 不含业务逻辑，service 不感知 HTTP，repo 不感知业务规则。

## 依赖注入: serve.go 初始化顺序

```go
// 1. 基础设施
logger -> config -> database -> RunMigrations -> Hub

// 2. Repo 层
resourceRepo -> aliasRepo -> groupRepo -> deployRepo

// 3. Service 层
resourceSvc -> aliasSvc -> groupSvc -> deploySvc

// 4. 后注入（解决循环依赖）
groupSvc.SetDeployService(deploySvc)
resourceSvc.SetDeployService(deploySvc)

// 5. Handler 层
resourceHandler -> aliasHandler(+deploySvc) -> groupHandler -> deployHandler -> dataHandler -> wsHandler -> healthHandler

// 6. Server
server.New(...所有handler...) -> httpServer.ListenAndServe

// 7. 辅助服务
watcherSvc.Start()
```

## 关键设计

### switch type 共用逻辑

skill/agent/mcp 三种类型共用同一套 CRUD 接口，内部通过 `switch resource.Type` 分流:
- 创建: `createSkillFiles` / `createAgentFile` / `createMCPFile`
- 部署: `deploySkill` / `deployAgent` / `deployMCP`
- 删除: skill = `RemoveAll`，agent/mcp = `Remove`
- 健康检查: symlink = `Lstat`，merge = 检查 JSON key

### BizError + Data

```go
type BizError struct {
    Code int
    Msg  string
    Data interface{}  // 可选附加数据
}
```

- 所有业务异常通过 `BizError` 传递，handler 统一处理
- `NewBizErrorWithData`: 用于需要返回附加信息的场景（如冲突详情）
- 错误码分段: 1000资源 / 2000分组 / 3000部署 / 4000别名 / 5000数据 / 9000系统

### 读写锁 SQLite

- `repo.DB` 内嵌 `sync.RWMutex`
- 所有写操作持写锁，读操作持读锁
- 配合 WAL 模式实现安全并发

### 追踪部署 (track)

- 部署时 `track=1` 表示该分组部署是「追踪」模式
- 分组添加资源 -> 自动部署到所有追踪目标
- 分组移除资源 -> 自动从所有追踪目标撤销
- 删除分组 -> 级联撤销所有追踪部署

### _meta.json

每次部署在目标路径的 `.aiResource/_meta.json` 写入元数据，用于:
- 创建别名时自动还原部署关联（RestoreFromMeta）
- 审计追踪

## 开发约定

- 端口: `3678`（后端），`5173`（前端 vite dev）
- 重启后端: `pkill -f "go-build.*serve" 2>/dev/null`（禁止 `lsof -ti:3678 | xargs kill -9`，会误杀 vite）
- 重启后验证双端口: `curl -s -o /dev/null -w "%{http_code}" http://localhost:3678/api/v1/health` + `curl ... http://localhost:5173`
- 前端挂了立即重启: `cd web && npm run dev &`
