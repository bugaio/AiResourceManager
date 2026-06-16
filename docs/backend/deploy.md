# 部署服务 (service/deploy.go)

## 概述

部署服务是系统核心，负责将资源（skill/agent/mcp）部署到用户指定的目标路径。skill/agent 用 symlink，mcp 用 JSON 深度合并。

## 结构体

```go
type DeployService struct {
    deployRepo   *repo.DeployRepo
    resourceRepo *repo.ResourceRepo
    aliasRepo    *repo.AliasRepo
    groupRepo    *repo.GroupRepo
    baseDir      string // ~/.aiManager
}
```

## Deploy 主流程

```
Deploy(req) → resolveTargetPath → resolveResources → 按type处理路径 → 逐资源部署 → 记录DB → writeMetaJSON
```

### 1. resolveTargetPath

优先级：`alias_id` > `target_path`

- 有 `alias_id`: 查 alias 表获取 path → `util.ExpandPath` → `filepath.Clean`
- 无 alias: 直接用 `target_path` → 展开 → Clean

### 2. resolveResources

优先级：`ResourceIDs` > `ResourceID` > `GroupID`

- `ResourceIDs`: 批量查资源，任一不存在即报错
- `ResourceID`: 查单个资源
- `GroupID`: 查 `group_resource` 获取所有资源 ID，逐个查资源

### 3. 按 type 处理路径

- **MCP**: 目标路径必须是 `.json` 文件且已存在（不自动创建），否则报错
- **skill/agent**: 目标路径是目录，不存在则 `util.EnsureDir` 创建

### 4. 逐资源部署

force=true 时先调用 `cleanOldDeploymentItems` 清理旧记录。

### 5. 写 DB

创建 `deployment` 记录 + 逐个 `deployment_item` 记录。

### 6. writeMetaJSON

在目标路径的 `.aiResource/_meta.json` 中追加部署元数据。

---

## deploySkill

```go
func (s *DeployService) deploySkill(resource, targetPath, force) (linkPath, error)
```

- `EnsureDir(targetPath)`
- 链接目标: `targetPath/{resource.Name}`（目录级 symlink）
- 链接源: `~/.aiManager/skills/{resource.ID}`
- force=true 时先 `os.RemoveAll` 已有目标
- 调用 `util.CreateSymlink(source, linkDst)`

## deployAgent

```go
func (s *DeployService) deployAgent(resource, targetPath, force) (linkPath, error)
```

- `EnsureDir(targetPath)`
- 链接目标: `targetPath/{resource.Name}.md`（单文件 symlink）
- 链接源: `~/.aiManager/agents/{resource.ID}.md`
- force=true 时先 `os.Remove` 已有目标

## deployMCP（核心复杂逻辑）

```go
func (s *DeployService) deployMCP(resource, targetPath, force) (serverName, error)
```

**完整流程**:

1. 读资源 JSONC 文件: `~/.aiManager/mcps/{resource.ID}.jsonc`
2. `util.ParseJSONC(data)` 去注释 → 解析为 `map[string]interface{}`
3. 收集新 MCP 的所有 key（顶层 + `mcpServers` 子 key）
4. 提取 `serverName` 用作返回值（优先 `mcpServers` 下第一个 key，其次顶层第一个 key）
5. 读取目标 `.json` 文件内容（支持 JSONC）
6. **冲突检测**:
   - `findMCPConflicts`: 查所有已部署到该 target 的 MCP 资源，读取各自实际配置取 key 集合，与新 MCP key 做交集
   - 检测「原始内容」冲突: 目标文件中非已部署 MCP 管理的 key 与新 key 重叠
7. 有冲突且 `force=false` → 返回 `BizErrorWithData`（含冲突名称列表）
8. `force=true` 时:
   - 调用 `undeployMCPResource` 撤销冲突 MCP 的部署
   - 从目标中删除冲突的原始 key
   - 重新读取目标文件
9. `util.DeepMerge(targetJSON, mcpConfig)` 深度合并
10. `json.MarshalIndent` → `os.WriteFile` 写回目标

### DeepMerge (util/merge.go)

lodash 式递归合并规则:
- 两边同 key 且都是 `map[string]interface{}` → 递归深度合并
- 其余情况（标量、数组、类型不一致）→ src 覆盖 dst
- dst 中有而 src 中没有的 key → 保留

```go
func DeepMerge(dst, src map[string]interface{}) map[string]interface{}
```

### findMCPConflicts

```go
func (s *DeployService) findMCPConflicts(targetPath, selfResourceID string, newKeys map[string]bool) []mcpConflictInfo
```

遍历该 targetPath 下所有 `deploy_type=merge` 的 deployment → 取每个 item 的 resourceID → 读其 JSONC 配置获取 key 集合 → 与 newKeys 做交集。返回有交集的资源信息。

### getManagedKeys

获取 targetPath 下所有已部署 MCP 管理的 key 集合（排除自身），用于区分「已部署 MCP 的 key」和「原始内容的 key」。

## removeMCPResourceKeys

撤销 MCP 部署时调用，从目标 JSON 文件中移除该资源写入的所有 key:
1. 读资源 JSONC 配置，获取其写入的所有顶层 key
2. 从目标文件顶层删除这些 key
3. 若资源有 `mcpServers`，从目标的 `mcpServers` 下也删除对应子 key
4. 写回目标文件

**注意**: 不是只删 `link_path` 存的那个 key，而是读资源配置删所有 key。

## checkItemHealth

```go
func (s *DeployService) checkItemHealth(deployment, item) string  // "ok" | "broken"
```

- **symlink**: `os.Lstat(item.LinkPath)` 成功即 ok
- **merge**: 读目标 JSON，检查顶层或 `mcpServers` 下是否存在 `item.LinkPath` 对应的 key

## CleanItem

```go
func (s *DeployService) CleanItem(itemID string, undeploy bool) error
```

- `undeploy=true`: 先撤销实际文件（symlink → `os.Remove`，merge → `removeMCPResourceKeys`），再删 DB 记录
- `undeploy=false`: 仅删 DB 记录
- 删除 item 后若该 deployment 下无其他 item → 删除整条 deployment

## DeploySingleResourceToTarget（追踪联动）

```go
func (s *DeployService) DeploySingleResourceToTarget(deploymentID string, resource *model.Resource, targetPath string) error
```

- **幂等**: 检查该 deployment 下是否已有此 resource_id 的 item，有则跳过
- 按 type 执行 deploySkill/deployAgent/deployMCP（force=true）
- 插入新 deployment_item

## UndeployResourceFromTarget（追踪联动）

```go
func (s *DeployService) UndeployResourceFromTarget(resourceID, deploymentID, targetPath, deployType string) error
```

- 若 `deployType` 为空，从 deployment 记录获取
- 查该资源在该 deployment 下的所有 item
- merge → `removeMCPResourceKeys`，symlink → `os.Remove`
- 删 item 记录
- 若 deployment 下无其他 item → 删 deployment + 清 _meta.json

## GetTargets

```go
func (s *DeployService) GetTargets(resourceType string) ([]model.TargetInfo, error)
```

- 获取所有 deployment 按 target_path 聚合
- 按资源 type 过滤: 遍历每个 deployment 的 items → 查资源 type 是否匹配
- 运行时填充: `ResourceName`, `Status`(checkItemHealth), `GroupName`/`GroupColor`(IsResourceInGroup 实时判断)
- 按最新部署时间倒序排列

## CheckConflicts（预检）

```go
func (s *DeployService) CheckConflicts(req *model.CheckConflictsReq) (*model.CheckConflictsResp, error)
```

不写文件，纯检测:
1. 收集每个待部署资源的 key 集合
2. **Union-Find 分组**: 有 key 交集的资源归为一组
   - 冲突组: 最后一个标 `applied`，其余标 `ignored`
   - 无冲突的单独资源: 标 `applied`
3. 检测与已部署 MCP 的冲突 → 标 `existing`
4. 检测与原始内容的冲突 → "原始内容" `existing`

## metaDirFor

```go
func metaDirFor(targetPath string) string
```

- `.json` 文件 → 父目录下 `.aiResource/`
- 目录 → 自身下 `.aiResource/`

## RestoreFromMeta

```go
func (s *DeployService) RestoreFromMeta(targetPath, aliasID string)
```

创建别名时调用。逻辑:
1. 若该路径已有 deployment 记录 → 仅更新 `alias_id` 关联，不重复创建
2. 读 `_meta.json` → 校验每条记录的资源是否仍存在
3. 无效条目从 meta 中移除
4. 有效条目按 `deployment_id` 分组 → 创建新的 deployment + items
5. 更新 meta 中的 deployment_id 为新 ID

## GetResourceDeployTargets

```go
func (s *DeployService) GetResourceDeployTargets(resourceID string) ([]model.ResourceDeployTarget, error)
```

查某资源已部署到的所有目标路径（MCP 保存后同步用），返回 deployment_id、target_path、alias_name、has_conflict。
