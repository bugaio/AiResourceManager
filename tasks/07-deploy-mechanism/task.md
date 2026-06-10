# 任务07: 部署机制

## 目标

实现后端核心部署逻辑：跨平台 symlink 创建/删除（skill整个目录、agent单文件）、MCP JSON 合并/撤销、_meta.json 追踪、冲突检测与处理、track 跟踪模式。

## 交付内容

### 后端
- `internal/handler/deploy.go`（部署/撤销/查询 handler）
- `internal/service/deploy.go`（部署核心逻辑）
- `internal/repo/deploy.go`（deployment + deployment_item 数据库操作）
- `internal/model/deploy.go`（Deployment/DeploymentItem 结构体 + DTO）

### API

| 方法 | 路径 | 功能 |
|------|------|------|
| POST | /api/v1/deployments | 执行部署 |
| GET | /api/v1/deployments | 查看所有部署记录（分页） |
| DELETE | /api/v1/deployments/:id | 撤销部署 |
| GET | /api/v1/targets | 查看目标路径应用状态 |

### 部署请求体

```json
{
    "group_id": 1,           // 整组部署（与 resource_id 二选一）
    "resource_id": null,     // 单资源部署
    "target_path": "/path",  // 目标路径（直接填写）
    "alias_id": 1,           // 或使用别名（与 target_path 二选一）
    "force": false,          // 冲突时是否强制覆盖
    "track": false           // 是否跟踪分组变化（仅 group_id 非空时有意义）
}
```

### Skill 部署逻辑（链接整个目录）
1. 解析目标路径（别名展开 / ~ 展开，跨平台）
2. 确保 `{target}/.claude/commands/` 目录存在
3. 检查 `{target}/.claude/commands/{resource.name}/` 是否已存在：
   - 已存在 + force=false → 返回 code=3002 冲突信息
   - 已存在 + force=true → 删除旧链接后创建
   - 不存在 → 直接创建
4. 创建软链接（Unix: symlink, Windows: junction）
   - 源：`{HOME}/.aiManager/skills/{uuid}/`
   - 目标：`{target}/.claude/commands/{resource.name}/`
5. 写入 deployment + deployment_item
6. 更新 `{target}/.claude/_meta.json`

### Agent 部署逻辑（链接单文件）
1. 确保 `{target}/.claude/agents/` 目录存在
2. 创建软链接（Unix: symlink, Windows: mklink）
   - 源：`{HOME}/.aiManager/agents/{uuid}.md`
   - 目标：`{target}/.claude/agents/{resource.name}.md`
3. 其余同 Skill

### MCP 部署逻辑（JSON 合并）
1. 解析目标路径
2. 读取 .jsonc 资源文件 → hujson 标准化为 JSON
3. 解析为 `{"serverName": {command, args, env}}` 结构
4. 读取/创建 `{target}/.claude/settings.json`（不存在则创建 `{"mcpServers":{}}`）
5. 检查 mcpServers 中是否有同名 key：
   - 有 + force=false → 返回冲突
   - 有 + force=true → 覆盖
6. 将整个 key-value 对象合并到 mcpServers
7. 写回 settings.json（缩进2空格）
8. 写入 deployment 记录 + 更新 _meta.json

### Track 模式
- 部署分组时 track=true → deployment 记录 track=1
- 后续分组资源变化时，由 group service 触发自动同步（见任务03）
- track=false → 静态快照，分组变化不影响

### 撤销逻辑
1. 查询 deployment 记录
2. symlink 类型 → os.Remove(link_path)
3. merge 类型 → 从 settings.json 删除对应 key，写回
4. 更新 _meta.json（移除记录）
5. 删除数据库记录

### targets 接口
- 聚合所有 deployment 记录，按 target_path 分组
- 每个路径下展示：部署的资源列表、是整组还是单个、是否 track、部署时间
- **含健康检查**：每个 item 检查 link_path 是否存在，返回 status: "ok" | "broken"

### 健康检查与修复 API
- `POST /api/v1/deployments/check` — 手动触发全量检查，返回所有 broken 项
- `POST /api/v1/deployments/:id/repair` — 修复 broken 项（重新创建 symlink / 重新合并 MCP）
- `DELETE /api/v1/deployments/:id/items/:item_id` — 清理单条 broken 记录（承认删除事实）

检查逻辑：
- symlink 类型：`os.Lstat(link_path)` 检查链接是否存在
- merge 类型：读取 settings.json 检查 mcpServers 中对应 key 是否存在

## 验收标准

1. 部署 skill 后，目标路径下出现正确的 symlink 指向整个目录
2. 部署 agent 后，symlink 指向正确的 .md 文件
3. 部署 MCP 后，settings.json 的 mcpServers 中出现对应的完整配置
4. _meta.json 正确记录所有部署信息
5. 冲突时 force=false 返回3002，force=true 覆盖成功
6. 撤销部署后 symlink 消失 / settings.json 对应字段消失
7. targets 接口返回正确的聚合数据（含 track 状态）
8. settings.json 不存在时自动创建
9. track=true 的部署在分组变化时自动同步（配合任务03）
10. 跨平台：当前 OS 下 symlink/junction 正确创建
11. targets 接口返回每个 item 的 status（ok/broken）
12. 手动删除 symlink 后，targets 返回 broken 状态
13. repair 接口能重建 broken 的链接
14. 清理记录接口能删除单条 deployment_item
