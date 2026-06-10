# AiResourceManager 产品需求文档 (PRD)

## 1. 产品概述

### 1.1 定位

本地运行的单二进制工具，集中管理 Claude Code 的 skill / MCP / subAgent 资源。通过分组和软链接/合并机制，将资源分发到各项目目录。

### 1.2 技术栈

| 层级 | 选型 |
|------|------|
| 前端 | Vue3 + Vite + TypeScript + Element Plus + Pinia |
| 后端 | Go (Gin) + SQLite (go-sqlite3) |
| 编辑器 | Monaco Editor |
| 部署 | Go embed 打包为单二进制 |
| 默认端口 | 3678 |

### 1.3 中央仓库

所有资源统一存储在用户主目录下的 `.aiManager/` 目录中。跨平台路径通过 `os.UserHomeDir()` 获取：
- macOS/Linux: `~/.aiManager/`
- Windows: `%USERPROFILE%\.aiManager\`

SQLite 存储元数据和关系，文件系统存储实际内容。

---

## 2. 中央仓库结构

```
{HOME}/.aiManager/
├── data/
│   └── aimanager.db          # SQLite 数据库
├── skills/
│   └── {uuid}/
│       ├── SKILL.md          # skill 主文件
│       ├── meta.json         # {name, description, tags, createdAt, updatedAt}
│       └── ...               # 其他文件/子目录（用户自定义）
├── agents/
│   └── {uuid}.md             # subAgent 文件（纯文件管理，不解析内容）
├── mcps/
│   └── {uuid}.jsonc          # 单个 MCP server 配置（支持注释）
└── config.yaml               # 全局配置（端口、主题等，含中文注释）
```

### 2.1 Skill 目录结构说明

每个 skill 是一个完整目录，可包含任意子文件/子目录：

```
skills/{uuid}/
├── SKILL.md              # 主文件（必须存在）
├── meta.json             # 元数据（工具管理用）
├── templates/            # 用户自定义子目录
│   └── ...
└── helpers.md            # 用户自定义辅助文件
```

部署时 symlink 链接整个目录，目标路径下能看到完整的目录内容。

### 2.2 MCP .jsonc 文件格式

存储为完整的 mcpServers 片段（一个 server 对应一个 key-value）：

```jsonc
// Context7 MCP Server 配置
{
  "context7": {
    "command": "npx",
    "args": ["-y", "@anthropic/context7-mcp"],
    "env": {
      "CONTEXT7_API_KEY": "xxx"
    }
  }
}
```

合并到目标 settings.json 时，直接将此对象的键值对合并到 `mcpServers` 字段下。

### 2.3 Agent .md 文件

纯文件管理，不解析文件内容。工具仅负责存储、分组、软链接操作。

### 2.4 config.yaml 首次生成

首次启动时自动生成到 `{HOME}/.aiManager/config.yaml`，内容包含全部配置项及中文注释说明。加载到内存后不再热更新，用户修改后需重启生效。

---

## 3. 数据模型

### 3.1 resource（资源表）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增主键 |
| uuid | TEXT UNIQUE | 文件系统存储标识 |
| type | TEXT | skill / mcp / agent |
| name | TEXT | 显示名称 |
| description | TEXT | 描述 |
| file_path | TEXT | 相对于 ~/.aiManager/ 的存储路径 |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |

### 3.2 group（分组表）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增主键 |
| name | TEXT | 分组名称 |
| type | TEXT | skill / mcp / agent |
| sort_order | INTEGER | 排序权重 |
| created_at | DATETIME | 创建时间 |

### 3.3 group_resource（分组-资源关联表）

| 字段 | 类型 | 说明 |
|------|------|------|
| group_id | INTEGER FK | 分组 ID |
| resource_id | INTEGER FK | 资源 ID |

联合主键: (group_id, resource_id)

### 3.4 path_alias（路径别名表）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增主键 |
| name | TEXT UNIQUE | 别名名称（如 "claude全局"） |
| path | TEXT | 实际路径（如 ~/.claude） |
| created_at | DATETIME | 创建时间 |

### 3.5 deployment（部署记录表）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增主键 |
| group_id | INTEGER FK (nullable) | 整组部署时的分组 ID |
| resource_id | INTEGER FK (nullable) | 单资源部署时的资源 ID |
| target_path | TEXT | 目标路径 |
| alias_id | INTEGER FK (nullable) | 使用的路径别名 |
| deploy_type | TEXT | symlink / merge |
| track | BOOLEAN | 是否跟踪分组变化（仅group_id非空时有意义） |
| created_at | DATETIME | 部署时间 |

### 3.6 deployment_item（部署明细表）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增主键 |
| deployment_id | INTEGER FK | 部署记录 ID |
| resource_id | INTEGER FK | 资源 ID |
| link_path | TEXT | 实际创建的链接/合并路径 |

---

## 4. 页面结构

### 4.1 顶部栏 (TopBar)

- **资源类型切换**: skill / mcp / subAgent 三个 Tab
- **设置入口**:
  - 目录别名管理
  - 数据导入导出

### 4.2 左侧栏 (Sidebar)

分为上下两部分：

#### 上部分 — 分组列表

- "全部" 项（展示当前类型所有资源）
- 自定义分组列表（可排序）
- 右侧齿轮按钮，弹窗可操作：
  - 新增分组（填写名称）
  - 配置是否默认显示全部分组
  - 添加指定资源到分组

每个分组右侧有操作按钮：
- 删除分组
- 重命名分组
- 将此分组导入到指定目录（可选路径别名或手动填写路径）

#### 下部分 — 目标路径列表

显示已注册的路径别名 + 操作历史中的路径，展示该路径下已应用的资源：
- 整组引用：只能整组移除
- 单个引用：可单个移除

### 4.3 右侧内容区 (Content)

- 顶部搜索栏：按名称筛选
- 资源卡片网格展示
- 每张卡片显示：
  - 资源名称
  - 描述
  - 所属分组标签
  - 操作按钮：
    - 编辑内容（打开 Monaco 编辑器）
    - 移动/添加到其他分组
    - 部署到指定路径（可选路径别名）
    - 删除

### 4.4 目录别名管理页

- 别名列表：名称 + 路径
- 支持：查看、筛选、编辑（重命名或重新设置路径）、删除
- 新增别名

### 4.5 数据导入导出页

- **导出**: 选择导出目标目录，一键导出全部数据（db + 资源文件）
- **导入**: 选择导入源目录，一键导入数据

---

## 5. 部署机制

### 5.1 Skill 部署

- 操作：创建 symlink（链接整个目录）
- 源：`{HOME}/.aiManager/skills/{uuid}/`
- 目标：`{target_path}/.claude/commands/{skill-name}/`
- Windows：使用 junction（目录软链接）

### 5.2 SubAgent 部署

- 操作：创建 symlink
- 源：`{HOME}/.aiManager/agents/{uuid}.md`
- 目标：`{target_path}/.claude/agents/{agent-name}.md`
- Windows：使用 mklink（文件软链接）

### 5.3 MCP 部署

- 操作：JSON 合并
- 流程：
  1. 读取 `{target_path}/.claude/settings.json`（不存在则创建 `{"mcpServers":{}}`）
  2. 解析 .jsonc 资源文件（hujson strip注释）
  3. 将整个 key-value 对象合并到 settings.json 的 `mcpServers` 字段
  4. 在 `{target_path}/.claude/_meta.json` 记录合并来源

### 5.4 跟踪模式 (Track)

部署分组时，用户可选择是否"跟踪"该分组：

| 模式 | 行为 |
|------|------|
| 跟踪 (track=true) | 双向同步：分组新增资源 → 自动部署到目标；分组移除资源 → 自动从目标撤销 |
| 静态 (track=false) | 部署时的快照，后续分组变化不影响已部署内容 |

跟踪模式的实现：
- 分组添加资源时，检查该分组是否有 track=true 的部署记录
- 若有，自动对新资源执行部署到对应目标路径
- 分组移除资源时，同理自动撤销

### 5.5 撤销部署

| 类型 | 撤销操作 |
|------|---------|
| skill | 删除 symlink（整个目录链接） |
| subAgent | 删除 symlink |
| MCP | 从 settings.json 移除对应 mcpServers 字段，写回文件，更新 _meta.json |

### 5.6 资源删除的级联处理

删除资源时：
1. 检查该资源是否有关联的部署记录
2. 若有 → 返回关联信息（部署到了哪些路径），要求前端确认
3. 用户确认删除后：
   - 撤销所有关联部署（删symlink / 从settings.json移除）
   - 更新所有相关 _meta.json
   - 清理 deployment + deployment_item 记录
   - 清理 group_resource 关联
   - 删除文件系统中的资源文件/目录
   - 删除 resource 记录

### 5.7 _meta.json 结构

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

## 6. API 设计

### 6.1 通用分页参数

列表接口统一支持分页：

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| page | int | 1 | 页码 |
| page_size | int | 20 | 每页条数（最大100） |

响应中包含分页信息：
```json
{
  "code": 0,
  "msg": "ok",
  "data": {
    "list": [],
    "total": 100,
    "page": 1,
    "page_size": 20
  }
}
```

### 6.2 资源管理

```
GET    /api/v1/resources              # 列表（?type=skill&search=xxx&group_id=0&page=1&page_size=20）
POST   /api/v1/resources              # 创建
GET    /api/v1/resources/:id          # 详情
PUT    /api/v1/resources/:id          # 更新元数据
DELETE /api/v1/resources/:id          # 删除（返回关联部署信息，需confirm=true确认）
DELETE /api/v1/resources/batch        # 批量删除（body: {ids: [], confirm: true}）

GET    /api/v1/resources/:id/content  # 获取文件内容（Monaco 编辑）
PUT    /api/v1/resources/:id/content  # 保存文件内容
```

特殊 group_id 约定：
- `group_id=0` → "全部"，返回当前 type 所有资源（不按分组筛选）
- `group_id=N` (N>0) → 按指定分组筛选

删除接口行为：
- `DELETE /api/v1/resources/:id` → 检查关联部署，返回 `{code: 1004, data: {deployments: [...]}}`
- `DELETE /api/v1/resources/:id?confirm=true` → 强制删除：撤销所有部署 + 清理文件 + 删记录
- `DELETE /api/v1/resources/batch` → 批量删除，目标文件/目录不存在时跳过（不报错）

### 6.3 分组管理

```
GET    /api/v1/groups                 # 列表（?type=skill&page=1&page_size=20）
POST   /api/v1/groups                 # 创建
PUT    /api/v1/groups/:id             # 更新（重命名、排序）
DELETE /api/v1/groups/:id             # 删除

POST   /api/v1/groups/:id/resources   # 添加资源到分组（若有track部署则自动同步）
DELETE /api/v1/groups/:id/resources/:rid  # 从分组移除资源（若有track部署则自动撤销）
```

### 6.4 路径别名

```
GET    /api/v1/aliases                # 列表
POST   /api/v1/aliases                # 创建
PUT    /api/v1/aliases/:id            # 更新
DELETE /api/v1/aliases/:id            # 删除
```

### 6.5 部署

```
POST   /api/v1/deployments            # 执行部署（分组或单资源，含 track 字段）
GET    /api/v1/deployments            # 查看所有部署记录（?page=1&page_size=20，含健康状态）
DELETE /api/v1/deployments/:id        # 撤销部署

POST   /api/v1/deployments/check      # 手动触发全量健康检查
POST   /api/v1/deployments/:id/repair # 修复 broken 部署项（重建链接/重新合并）
DELETE /api/v1/deployments/:id/items/:item_id  # 清理单条 broken 记录

GET    /api/v1/targets                # 查看所有目标路径及其应用状态（含健康状态）
```

### 6.6 数据导入导出

```
POST   /api/v1/data/export            # 导出到指定目录
POST   /api/v1/data/import            # 从指定目录导入
```

### 6.7 WebSocket

```
WS     /api/v1/ws                     # WebSocket 连接
```

推送事件类型：

| 事件 | 触发时机 | payload |
|------|---------|---------|
| `resource:updated` | 文件监听检测到资源文件变化 | `{resource_id, uuid, updated_at}` |
| `resource:deleted` | 文件被外部删除 | `{resource_id, uuid}` |
| `deploy:synced` | track模式自动同步完成 | `{deployment_id, action, resource_name}` |

前端收到事件后自动刷新对应数据。

---

## 7. 构建与运行

### 7.1 开发模式

```bash
# 前端（热更新）
cd web && npm run dev

# 后端
go run . serve --port 3678
```

Vite 开发模式通过 proxy 将 `/api` 请求转发到后端。

### 7.2 生产构建

```bash
cd web && npm run build              # 输出到 web/dist/
go build -o aimanager .              # embed web/dist → 单二进制
```

### 7.3 运行

```bash
./aimanager serve                    # 默认端口 3678
./aimanager serve --port 8080        # 指定端口
```

---

## 8. 非功能性要求

### 8.1 代码注释规范

- 前后端代码均使用中文注释
- 所有函数/方法必须有注释，包含：功能说明、入参说明、返回值说明
- 每个文件顶部必须有模块级注释，说明：
  - 模块功能概述
  - 数据流转方向（从哪来、到哪去）
  - 依赖的其他模块
- 关键业务逻辑处需有行内注释解释"为什么"
- 复杂数据结构需注释每个字段含义

Go 示例：
```go
// Package service 实现核心业务逻辑层
// 数据流转：handler → service → repo（数据库）+ 文件系统
// 本模块负责资源的创建、删除、内容读写等业务操作

// CreateResource 创建一个新资源（skill/mcp/agent）
// 流程：校验参数 → 生成UUID → 创建文件 → 写入DB → 关联分组
// 参数：
//   - ctx: 请求上下文
//   - req: 创建请求，包含 type/name/description/group_ids
// 返回：
//   - *model.Resource: 创建成功的资源详情
//   - error: 失败时的错误信息（含错误码）
func (s *ResourceService) CreateResource(ctx context.Context, req *model.CreateResourceReq) (*model.Resource, error) {
```

Vue/TS 示例：
```typescript
/**
 * 资源状态管理模块
 * 管理资源列表的加载、筛选、分页状态
 * 数据流转：组件 → store action → API → 后端
 * 依赖：api/resource.ts, types/resource.ts
 */

/**
 * 获取资源列表
 * @param type - 资源类型 (skill/mcp/agent)
 * @param groupId - 分组ID，0表示全部
 * @param search - 搜索关键词
 * @returns 分页资源列表
 */
async function fetchResources(type: string, groupId: number, search: string): Promise<PaginatedList<Resource>> {
```

### 8.2 其他要求

- 错误处理：API 统一返回 `{code, msg, data}` 格式
- 文件操作需处理权限错误、路径不存在等异常
- symlink 创建前检查目标是否已存在，避免覆盖用户文件
- 删除操作中目标文件/目录已不存在时跳过（不报错）
- 跨平台支持：macOS / Linux / Windows
  - 路径：`os.UserHomeDir()` + `filepath.Join()`
  - 目录软链接：Unix 用 symlink，Windows 用 junction
  - 文件软链接：Unix 用 symlink，Windows 用 mklink
- WebSocket 保持长连接，支持自动重连（前端实现）
- 列表接口统一支持分页

---

## 9. 实现优先级

| Phase | 内容 | 产出 |
|-------|------|------|
| 1 | 后端骨架 + SQLite + 资源/分组/别名 CRUD | 可用 API |
| 2 | 前端三栏布局 + 资源列表 + 分组管理 | 基本可视化 |
| 3 | 部署机制（symlink + MCP merge）| 核心功能闭环 |
| 4 | Monaco 编辑器 + 数据导入导出 | 完整编辑能力 |
| 5 | UX 完善（搜索筛选、排序、批量操作）| 体验优化 |
