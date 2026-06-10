# AiResourceManager 后端技术文档

## 1. 技术栈

| 项目 | 选型 |
|------|------|
| 语言 | Go 1.26.1 |
| Web框架 | Gin |
| WebSocket | gorilla/websocket |
| 数据库 | SQLite (mattn/go-sqlite3, CGO) |
| 配置管理 | Viper (yaml + env + flag) |
| 日志 | zap (可配置输出到终端/文件) |
| CLI | cobra |
| UUID | 标准库 crypto/rand 自行生成 |
| JSONC解析 | tailscale/hujson |
| 文件监听 | fsnotify/fsnotify |
| 前端嵌入 | go:embed |

---

## 2. 项目目录结构

```
├── main.go                       # 入口
├── go.mod
├── go.sum
├── embed.go                      # //go:embed web/dist
├── cmd/
│   ├── root.go                   # cobra 根命令
│   └── serve.go                  # serve 子命令（启动HTTP服务）
├── internal/
│   ├── config/
│   │   └── config.go             # Viper 配置加载
│   ├── server/
│   │   ├── server.go             # Gin 引擎初始化 + 路由注册
│   │   └── middleware.go         # 中间件（CORS、日志、Recovery）
│   ├── handler/
│   │   ├── resource.go           # 资源 CRUD handler
│   │   ├── group.go              # 分组 CRUD handler
│   │   ├── alias.go              # 路径别名 handler
│   │   ├── deploy.go             # 部署/撤销 handler
│   │   ├── data.go               # 导入导出 handler
│   │   ├── ws.go                 # WebSocket handler
│   │   └── response.go           # 统一响应封装
│   ├── service/
│   │   ├── resource.go           # 资源业务逻辑（含文件系统操作）
│   │   ├── group.go              # 分组业务逻辑
│   │   ├── alias.go              # 别名业务逻辑
│   │   ├── deploy.go             # 部署核心逻辑（symlink/merge）
│   │   ├── data.go               # 导入导出逻辑
│   │   ├── watcher.go            # 文件监听服务
│   │   └── hub.go                # WebSocket 连接管理 + 事件广播
│   ├── repo/
│   │   ├── db.go                 # SQLite 连接管理 + 读写锁
│   │   ├── migration.go          # 数据库迁移（建表/升级）
│   │   ├── resource.go           # 资源数据访问
│   │   ├── group.go              # 分组数据访问
│   │   ├── alias.go              # 别名数据访问
│   │   └── deploy.go             # 部署记录数据访问
│   ├── model/
│   │   ├── resource.go           # 资源模型
│   │   ├── group.go              # 分组模型
│   │   ├── alias.go              # 别名模型
│   │   ├── deploy.go             # 部署模型
│   │   └── errors.go             # 错误码定义
│   └── util/
│       ├── uuid.go               # UUID 生成（crypto/rand）
│       ├── file.go               # 文件操作工具
│       └── jsonc.go              # JSONC 解析封装（hujson）
├── configs/
│   └── config.yaml               # 默认配置模板
└── web/                          # 前端代码（构建后 dist 被 embed）
```

---

## 3. 配置管理

### 3.1 配置文件位置

`~/.aiManager/config.yaml`

### 3.2 配置结构

```yaml
server:
  port: 3678              # 监听端口
  open_browser: true      # 启动时是否自动打开浏览器

log:
  level: info             # debug / info / warn / error
  output: stdout          # stdout / file / both
  file_path: ~/.aiManager/logs/app.log

database:
  path: ~/.aiManager/data/aimanager.db
```

### 3.3 优先级

flag参数 > 环境变量 (AIM_*) > config.yaml > 默认值

---

## 4. 数据库设计

### 4.1 连接管理

- SQLite WAL 模式
- 读写锁（sync.RWMutex）：并发读，写串行
- 单个 *sql.DB 实例，连接池 max_open=1（写），读可并发

### 4.2 迁移策略

启动时自动执行迁移，内嵌 SQL 文件，按版本号递增执行：

```
internal/repo/migrations/
├── 001_init.sql          # 初始建表
├── 002_xxx.sql           # 后续升级
└── ...
```

迁移记录存在 `_migrations` 表中，已执行的不重复执行。

### 4.3 表结构 SQL

```sql
-- 资源表
CREATE TABLE resource (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid        TEXT NOT NULL UNIQUE,
    type        TEXT NOT NULL CHECK(type IN ('skill', 'mcp', 'agent')),
    name        TEXT NOT NULL,
    description TEXT DEFAULT '',
    file_path   TEXT NOT NULL,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 分组表
CREATE TABLE "group" (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL,
    type        TEXT NOT NULL CHECK(type IN ('skill', 'mcp', 'agent')),
    sort_order  INTEGER DEFAULT 0,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 分组-资源关联表
CREATE TABLE group_resource (
    group_id    INTEGER NOT NULL REFERENCES "group"(id) ON DELETE CASCADE,
    resource_id INTEGER NOT NULL REFERENCES resource(id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, resource_id)
);

-- 路径别名表
CREATE TABLE path_alias (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    path        TEXT NOT NULL,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 部署记录表
CREATE TABLE deployment (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id    INTEGER REFERENCES "group"(id) ON DELETE SET NULL,
    resource_id INTEGER REFERENCES resource(id) ON DELETE SET NULL,
    target_path TEXT NOT NULL,
    alias_id    INTEGER REFERENCES path_alias(id) ON DELETE SET NULL,
    deploy_type TEXT NOT NULL CHECK(deploy_type IN ('symlink', 'merge')),
    track       INTEGER DEFAULT 0,  -- 是否跟踪分组变化（0=静态, 1=跟踪）
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 部署明细表
CREATE TABLE deployment_item (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    deployment_id INTEGER NOT NULL REFERENCES deployment(id) ON DELETE CASCADE,
    resource_id   INTEGER NOT NULL REFERENCES resource(id) ON DELETE CASCADE,
    link_path     TEXT NOT NULL
);

-- 迁移记录表
CREATE TABLE _migrations (
    version     INTEGER PRIMARY KEY,
    applied_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

---

## 5. API 设计

### 5.1 统一响应格式

```json
{
    "code": 0,
    "msg": "ok",
    "data": {}
}
```

- `code = 0`：成功
- `code != 0`：失败，按错误码体系分类

### 5.2 错误码体系

| 范围 | 模块 | 示例 |
|------|------|------|
| 1000-1999 | 资源 | 1001=资源不存在, 1002=名称重复, 1003=文件读写失败 |
| 2000-2999 | 分组 | 2001=分组不存在, 2002=名称重复 |
| 3000-3999 | 部署 | 3001=目标路径不存在, 3002=symlink冲突, 3003=merge失败 |
| 4000-4999 | 别名 | 4001=别名不存在, 4002=名称重复 |
| 5000-5999 | 数据 | 5001=导入格式错误, 5002=导出失败 |
| 9000-9999 | 系统 | 9001=参数校验失败, 9002=内部错误 |

### 5.3 API 路由

#### 资源

```
GET    /api/v1/resources              # 列表 ?type=skill&group_id=0&search=xxx&page=1&page_size=20
POST   /api/v1/resources              # 创建
GET    /api/v1/resources/:id          # 详情
PUT    /api/v1/resources/:id          # 更新元数据
DELETE /api/v1/resources/:id          # 删除（?confirm=true 强制删除含级联清理）
DELETE /api/v1/resources/batch        # 批量删除（body: {ids: [], confirm: true}）

GET    /api/v1/resources/:id/content  # 获取文件内容
PUT    /api/v1/resources/:id/content  # 保存文件内容
```

group_id 约定：
- `group_id=0` → "全部"，不按分组筛选，返回当前 type 所有资源
- `group_id=N` (N>0) → 按指定分组筛选

删除行为：
- 无 confirm 参数时，若资源有关联部署 → 返回 code=1004 + 关联详情
- confirm=true 时 → 撤销所有部署 + 删文件 + 删DB记录
- 批量删除：逐个执行级联清理，目标文件/目录不存在时跳过（不报错）

#### 分组

```
GET    /api/v1/groups                 # 列表 ?type=skill&page=1&page_size=20
POST   /api/v1/groups                 # 创建
PUT    /api/v1/groups/:id             # 更新（重命名/排序）
DELETE /api/v1/groups/:id             # 删除

POST   /api/v1/groups/:id/resources   # 添加资源到分组（若有track部署则自动同步）
DELETE /api/v1/groups/:id/resources/:rid  # 移除资源（若有track部署则自动撤销）
```

#### 路径别名

```
GET    /api/v1/aliases                # 列表
POST   /api/v1/aliases                # 创建
PUT    /api/v1/aliases/:id            # 更新
DELETE /api/v1/aliases/:id            # 删除
```

#### 部署

```
POST   /api/v1/deployments            # 执行部署（body含 track 字段）
GET    /api/v1/deployments            # 查看所有部署记录 ?page=1&page_size=20（含健康检查状态）
DELETE /api/v1/deployments/:id        # 撤销部署

POST   /api/v1/deployments/check      # 手动触发全量健康检查
POST   /api/v1/deployments/:id/repair # 修复 broken 的部署项（重新创建链接/合并）
DELETE /api/v1/deployments/:id/items/:item_id  # 清理单条 broken 记录

GET    /api/v1/targets                # 查看所有目标路径及应用状态
```

#### 数据

```
POST   /api/v1/data/export            # 导出
POST   /api/v1/data/import            # 导入
```

#### WebSocket

```
GET    /api/v1/ws                     # WebSocket 升级连接
```

推送消息格式：
```json
{
    "event": "resource:updated",
    "data": {"resource_id": 1, "uuid": "xxx", "updated_at": "..."}
}
```

事件类型：
- `resource:updated` — 文件监听检测到变化
- `resource:deleted` — 文件被外部删除
- `deploy:synced` — track模式自动同步完成

---

## 6. 核心业务逻辑

### 6.1 资源创建流程

```
1. 校验参数（名称必填、类型合法）
2. 生成 UUID（crypto/rand）
3. 创建文件系统目录/文件：
   - skill: {HOME}/.aiManager/skills/{uuid}/SKILL.md + meta.json
   - agent: {HOME}/.aiManager/agents/{uuid}.md
   - mcp:   {HOME}/.aiManager/mcps/{uuid}.jsonc
4. 写入 SQLite resource 表
5. 若指定分组，写入 group_resource 关联
6. 返回资源详情
```

### 6.2 部署流程 — Skill/SubAgent (symlink)

```
1. 校验资源存在、目标路径存在
2. 计算链接目标路径：
   - skill: {target}/.claude/commands/{resource.name}/  （链接整个目录）
   - agent: {target}/.claude/agents/{resource.name}.md
3. 检查目标是否已存在：
   - 不存在 → 创建 symlink
   - 已存在 → 返回冲突信息（code=3002），由前端确认后带 force=true 重试
4. 创建目标目录（若不存在 .claude/commands/ 或 .claude/agents/）
5. 创建 symlink（跨平台：Unix=symlink, Windows=junction/mklink）
6. 写入 deployment + deployment_item 表
7. 更新/创建 {target}/.claude/_meta.json
```

### 6.3 部署流程 — MCP (merge)

```
1. 校验资源存在、目标路径存在
2. 读取资源 .jsonc 文件 → hujson 解析为标准 JSON
3. 读取 {target}/.claude/settings.json：
   - 不存在 → 创建 {"mcpServers": {}}
   - 存在 → 解析现有内容
4. 检查 mcpServers 中是否已有同名 key：
   - 已存在 → 返回冲突信息，由前端确认
5. 合并 MCP 配置到 mcpServers（整个 key-value 对象合入）
6. 写回 settings.json（格式化缩进2空格）
7. 写入 deployment + deployment_item 表
8. 更新 {target}/.claude/_meta.json
```

### 6.4 撤销部署流程

```
1. 查询 deployment + deployment_item 记录
2. 根据 deploy_type 执行撤销：
   - symlink: os.Remove(link_path)
   - merge: 从 settings.json 的 mcpServers 中删除对应 key，写回
3. 更新 _meta.json（移除对应记录）
4. 删除 deployment + deployment_item 记录
```

### 6.5 资源删除的级联处理

```
1. 查询资源是否有关联的 deployment_item 记录
2. 若有 → 返回 code=1004, data={deployments: [{id, target_path, resource_name}]}
3. 前端展示确认弹窗后，带 ?confirm=true 再次调用
4. 确认删除后执行：
   a. 遍历所有关联 deployment_item，执行撤销（删symlink/从json移除）
   b. 更新所有相关目标路径的 _meta.json
   c. 删除 deployment_item 记录
   d. 若 deployment 下无其他 item，删除 deployment 记录
   e. 删除 group_resource 关联
   f. 删除文件系统中的资源文件/目录
   g. 删除 resource 记录
```

### 6.6 分组跟踪同步

```
当分组关联变化时（添加/移除资源），检查该分组是否有 track=1 的部署记录：

添加资源到分组：
1. 写入 group_resource
2. 查询 deployment 表：WHERE group_id=? AND track=1
3. 对每条 track 部署记录，自动为新资源执行部署到对应 target_path
4. 写入新的 deployment_item
5. 通过 WebSocket 推送 deploy:synced 事件

从分组移除资源：
1. 查询 deployment 表：WHERE group_id=? AND track=1
2. 对每条 track 部署记录，自动撤销该资源在对应 target_path 的部署
3. 删除对应 deployment_item
4. 删除 group_resource
5. 通过 WebSocket 推送 deploy:synced 事件
```

### 6.7 文件监听

```
1. 启动时注册 fsnotify watcher 监听 {HOME}/.aiManager/skills/、agents/、mcps/
2. 文件变化事件处理：
   - CREATE: 日志记录（外部添加，不自动导入）
   - MODIFY: 更新 resource.updated_at，通过 WebSocket 推送 resource:updated
   - DELETE: 日志告警，通过 WebSocket 推送 resource:deleted
   - RENAME: 忽略（UUID命名不应被重命名）
3. 去重策略：短时间窗口去重
   - 同一文件路径在 500ms 内的多次事件合并为一次处理
   - 使用 map[string]time.Time 记录最近处理时间
   - 收到事件时检查：距上次处理 < 500ms 则跳过
   - 这同时解决了自身 API 操作触发的重复事件问题
```

### 6.8 部署健康检查

```
触发时机：
  - GET /api/v1/targets 接口调用时（前端查看侧栏目标路径列表）
  - GET /api/v1/deployments 接口调用时（前端查看部署记录）
  - POST /api/v1/deployments/check 手动触发检查

检查逻辑：
  对每条 deployment_item，验证 link_path 是否实际存在于文件系统：
  - symlink 类型：os.Lstat(link_path) 检查链接是否存在
  - merge 类型：读取 settings.json 检查 mcpServers 中对应 key 是否存在

返回结果中每个 item 附带状态字段：
  - status: "ok"     → 软链接/配置正常存在
  - status: "broken" → 软链接已被外部删除 / 配置已被外部移除

前端处理：
  - broken 状态的资源在侧栏目标路径列表中显示 ⚠️ 图标
  - 用户可操作：
    a. "重新部署" → 重新创建 symlink / 重新合并 MCP 配置
    b. "清理记录" → 删除 deployment_item（承认手动删除的事实）

API:
  POST /api/v1/deployments/:id/repair    → 重新部署（修复 broken 的 item）
  DELETE /api/v1/deployments/:id/items/:item_id  → 清理单条 item 记录
```

### 6.9 WebSocket Hub

```
1. 维护活跃连接列表（sync.Map）
2. 新连接注册 / 断开连接注销
3. 广播接口：接收事件 → 遍历所有活跃连接发送
4. 心跳检测：定期 ping，超时无 pong 则断开
5. 前端自动重连：断开后指数退避重连
```

---

## 7. 中间件

### 7.1 CORS

开发模式下允许跨域（前端 vite dev server 端口不同）。

### 7.2 请求日志

使用 zap 记录每个请求的 method、path、status、耗时。

### 7.3 Recovery

panic 恢复，返回 code=9002 内部错误。

### 7.4 静态文件服务

生产模式下，非 `/api` 路由全部返回 embed 的前端静态文件。SPA fallback 到 index.html。

---

## 8. CLI 命令

```bash
aimanager serve                    # 启动服务（默认端口3678）
aimanager serve --port 8080        # 指定端口
aimanager serve --no-open          # 不自动打开浏览器
aimanager version                  # 版本信息
```

---

## 9. 优雅退出

```
1. 监听 SIGINT / SIGTERM
2. 收到信号后：
   a. 停止接受新请求
   b. 关闭所有 WebSocket 连接
   c. 等待已有请求完成（超时5s强制关闭）
   d. 停止文件监听 watcher
   e. 关闭 SQLite 连接
   f. 刷新日志缓冲
   g. 退出
```

---

## 10. _meta.json 规范

部署操作在目标目录生成的追踪文件：

```json
{
    "managed_by": "AiResourceManager",
    "version": 1,
    "deployments": [
        {
            "deployment_id": 1,
            "type": "symlink",
            "resource_uuid": "a1b2c3d4...",
            "resource_name": "my-skill",
            "link_path": ".claude/commands/my-skill",
            "deployed_at": "2026-06-10T10:00:00Z"
        },
        {
            "deployment_id": 2,
            "type": "merge",
            "resource_uuid": "e5f6g7h8...",
            "resource_name": "context7",
            "mcp_key": "context7",
            "deployed_at": "2026-06-10T10:00:00Z"
        }
    ]
}
```

---

## 11. 关键依赖清单

```go
require (
    github.com/gin-gonic/gin          // Web 框架
    github.com/gorilla/websocket      // WebSocket
    github.com/mattn/go-sqlite3       // SQLite 驱动 (CGO)
    github.com/spf13/cobra            // CLI 框架
    github.com/spf13/viper            // 配置管理
    go.uber.org/zap                   // 日志
    github.com/fsnotify/fsnotify      // 文件监听
    github.com/tailscale/hujson       // JSONC 解析
)
```

---

## 12. 构建

### 12.1 开发

```bash
go run . serve --port 3678
```

### 12.2 生产

```bash
# 确保前端已构建
cd web && npm run build

# 编译（CGO 需要 C 编译器）
CGO_ENABLED=1 go build -o aimanager .
```

### 12.3 交叉编译注意

mattn/go-sqlite3 依赖 CGO，交叉编译需要对应平台的 C 工具链。可选方案：
- 本机编译
- Docker 多平台构建
- GitHub Actions matrix build

---

## 13. 跨平台处理

### 13.1 路径

- 使用 `os.UserHomeDir()` 获取用户主目录
- 使用 `filepath.Join()` 拼接路径（自动处理分隔符）
- 路径展开：`~` → `os.UserHomeDir()` 替换

### 13.2 软链接

| 操作 | Unix (macOS/Linux) | Windows |
|------|-------------------|---------|
| 目录链接 (skill) | `os.Symlink(src, dst)` | `exec.Command("cmd", "/c", "mklink", "/J", dst, src)` (junction) |
| 文件链接 (agent) | `os.Symlink(src, dst)` | `exec.Command("cmd", "/c", "mklink", dst, src)` |

Windows 注意事项：
- junction 不需要管理员权限（优先使用）
- 文件 symlink 在 Windows 10 1703+ 开发者模式下不需要管理员权限
- 操作失败时给出明确错误提示（权限不足等）

### 13.3 config.yaml 首次生成

启动时检测 `{HOME}/.aiManager/config.yaml` 是否存在：
- 不存在 → 从内嵌模板生成（含中文注释）
- 存在 → 直接加载到内存，运行期间不热更新

模板示例：
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
