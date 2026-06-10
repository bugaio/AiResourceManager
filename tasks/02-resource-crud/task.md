# 任务02: 资源 CRUD

## 目标

实现资源（skill/mcp/agent）的完整增删改查，包括 API 接口、业务逻辑、数据库操作、文件系统操作（创建目录/文件、读写内容、删除）。支持分页查询和删除时的级联处理。

## 交付内容

### 后端
- `internal/handler/resource.go`（handler：List/Create/Get/Update/Delete/GetContent/UpdateContent）
- `internal/service/resource.go`（资源业务逻辑：校验、文件创建/删除、UUID生成、删除级联）
- `internal/repo/resource.go`（数据库CRUD：插入、分页查询列表、按ID查、更新、删除）
- `internal/model/resource.go`（Resource结构体 + 请求/响应DTO）
- `internal/util/file.go`（文件读写工具函数，补充内容读写）
- `internal/util/jsonc.go`（hujson JSONC解析封装）

### API

| 方法 | 路径 | 功能 |
|------|------|------|
| GET | /api/v1/resources | 列表（?type=&search=&group_id=0&page=1&page_size=20） |
| POST | /api/v1/resources | 创建资源 |
| GET | /api/v1/resources/:id | 获取详情 |
| PUT | /api/v1/resources/:id | 更新元数据（名称、描述） |
| DELETE | /api/v1/resources/:id | 删除（检查关联，需confirm=true确认） |
| DELETE | /api/v1/resources/batch | 批量删除（body: {ids: [], confirm: true}） |
| GET | /api/v1/resources/:id/content | 获取文件内容 |
| PUT | /api/v1/resources/:id/content | 保存文件内容 |

### group_id 约定
- `group_id=0` → "全部"，不按分组筛选，返回当前 type 所有资源
- `group_id=N` (N>0) → 按指定分组筛选

### 文件系统操作
- 创建 skill：`{HOME}/.aiManager/skills/{uuid}/SKILL.md` + `meta.json`（目录可包含任意子文件）
- 创建 agent：`{HOME}/.aiManager/agents/{uuid}.md`
- 创建 mcp：`{HOME}/.aiManager/mcps/{uuid}.jsonc`
- 删除时同步清理文件/目录

### MCP .jsonc 文件格式

创建 MCP 时的初始内容模板：
```jsonc
// {name} MCP Server 配置
{
  "{server_name}": {
    "command": "",
    "args": [],
    "env": {}
  }
}
```

### 删除级联逻辑

```
DELETE /api/v1/resources/:id
  → 检查 deployment_item 中是否有关联记录
  → 有关联 → 返回 code=1004, data={deployments: [{id, target_path}]}
  → 前端展示确认

DELETE /api/v1/resources/:id?confirm=true
  → 遍历关联 deployment_item，撤销部署（删 symlink / 从 json 移除）
  → 更新相关 _meta.json
  → 删除 deployment_item 记录
  → 若 deployment 下无其他 item，删除 deployment 记录
  → 删除 group_resource 关联
  → 删除文件系统中的资源文件/目录
  → 删除 resource 记录
```

### 分页响应格式

```json
{
  "code": 0,
  "msg": "ok",
  "data": {
    "list": [...],
    "total": 100,
    "page": 1,
    "page_size": 20
  }
}
```

## 验收标准

1. 通过 curl/Postman 调通所有8个接口（含批量删除）
2. 创建 skill 后 `{HOME}/.aiManager/skills/{uuid}/SKILL.md` 和 `meta.json` 存在
3. 创建 mcp 后 .jsonc 文件内容格式正确（完整的 {"serverName": {...}} 结构）
4. 删除资源后文件被清理；目标文件已不存在时跳过不报错
5. 列表接口 group_id=0 返回全部，group_id=N 按分组筛选，分页返回 total
6. content 接口能正确读写 .md 和 .jsonc 文件内容
7. 删除有关联部署的资源时，返回 1004 + 关联信息
8. confirm=true 删除时级联清理完整（symlink删除、json字段移除、DB记录清理）
9. 批量删除接口正常工作，部分资源有关联时返回各自状态
10. 参数校验失败返回对应错误码（1001-1003）
11. 每个文件顶部有模块功能注释，每个函数有入参/返回值中文注释
