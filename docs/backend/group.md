# 分组服务 (service/group.go)

## 概述

分组允许将多个同类型资源打包管理。核心特性是「追踪部署」——分组部署时若 `track=1`，后续增删资源会自动同步到已部署目标。

## 结构体

```go
type GroupService struct {
    repo      *repo.GroupRepo
    hub       *Hub             // WebSocket Hub，广播事件
    logger    *zap.Logger
    deploySvc *DeployService   // 后注入
}
```

## SetDeployService

```go
func (s *GroupService) SetDeployService(svc *DeployService)
```

在 `serve.go` 中后注入，解决循环依赖。

## CreateGroup

流程:
1. 校验 type（skill/agent/mcp）
2. 校验 name 不为空
3. 同类型下名称重复检查（`CheckGroupNameExists`）
4. `pickColor(type)` 从 20 色池中选取未使用的颜色
5. 生成 UUID → `InsertGroup`

### pickColor

```go
func (s *GroupService) pickColor(groupType string) string
```

颜色池（20色）:
```go
var groupColors = []string{
    "#3B82F6", "#10B981", "#F59E0B", "#EF4444", "#8B5CF6",
    "#EC4899", "#06B6D4", "#84CC16", "#F97316", "#6366F1",
    "#14B8A6", "#D946EF", "#0EA5E9", "#22C55E", "#E11D48",
    "#7C3AED", "#2DD4BF", "#FBBF24", "#FB7185", "#A78BFA",
}
```

策略:
1. 查询同类型所有分组已用颜色
2. 从池中找未使用的，随机选一个
3. 全部用完时随机选任意一个

## ListGroups

- 分页查询
- 填充每个分组的 `ResourceCount`（`GetGroupResources` 计数）

## UpdateGroup

- 检查分组是否存在
- 名称变更时做同类型重名检查（排除自身）
- 支持更新 `name` 和 `sort_order`

## DeleteGroup

流程:
1. 检查分组是否存在
2. 查追踪部署（`GetTrackDeployments`）
3. 有追踪部署 → 逐个调用 `deploySvc.Undeploy` 级联撤销
4. `repo.DeleteGroup(id)` → 级联删 `group_resource` 关联

## AddResources

流程:
1. 校验分组存在
2. `AddResourcesToGroup(groupID, resourceIDs)` 批量写关联表
3. 查追踪部署 (`GetTrackDeployments`)
4. 有追踪部署 → 对每个 deployment × 每个新资源调用 `DeploySingleResourceToTarget`
5. 广播 `deploy:synced` WebSocket 事件

## RemoveResource

流程:
1. 校验分组存在
2. 查追踪部署
3. 有追踪部署 → 对每个 deployment 调用 `UndeployResourceFromTarget`（仅 track=1 才撤销实际文件）
4. `RemoveResourceFromGroup(groupID, resourceID)` 删关联
5. 广播 `deploy:synced` 事件

## GetGroupDeployments（GroupRepo 中）

查该分组的所有 deployment 记录（不限 track 值），用于展示分组已部署到哪些目标。

## 事件广播

```go
func (s *GroupService) broadcastSynced(groupID string)
```

通过 WebSocket Hub 广播 JSON:
```json
{"type": "deploy:synced", "group_id": "xxx"}
```

前端收到后刷新部署状态。
