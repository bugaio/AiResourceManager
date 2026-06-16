# API 路由一览

基础路径: `/api/v1`

## 资源 /resources

| Method | Path                       | Handler              | 说明                          |
|--------|----------------------------|----------------------|-------------------------------|
| GET    | /resources                 | handleList           | 列表查询（type/search/group_id/page/page_size） |
| POST   | /resources                 | handleCreate         | 创建资源                      |
| DELETE | /resources/batch           | handleBatchDelete    | 批量删除（body: {ids, confirm}）|
| GET    | /resources/:id             | handleGet            | 获取详情                      |
| PUT    | /resources/:id             | handleUpdate         | 更新元数据（name/description） |
| DELETE | /resources/:id             | handleDelete         | 删除资源（?confirm=true 级联） |
| GET    | /resources/:id/content     | handleGetContent     | 读取资源文件内容              |
| PUT    | /resources/:id/content     | handleUpdateContent  | 更新资源文件内容              |

## 分组 /groups

| Method | Path                           | Handler              | 说明                          |
|--------|--------------------------------|----------------------|-------------------------------|
| GET    | /groups                        | handleList           | 列表查询（type/page/page_size）|
| POST   | /groups                        | handleCreate         | 创建分组                      |
| PUT    | /groups/:id                    | handleUpdate         | 更新分组（name/sort_order）    |
| DELETE | /groups/:id                    | handleDelete         | 删除分组（级联撤销追踪部署）   |
| POST   | /groups/:id/resources          | handleAddResources   | 添加资源到分组                |
| DELETE | /groups/:id/resources/:rid     | handleRemoveResource | 从分组移除资源                |

## 路径别名 /aliases

| Method | Path           | Handler       | 说明                          |
|--------|----------------|---------------|-------------------------------|
| GET    | /aliases       | handleList    | 列表查询（?type=skill/agent/mcp）|
| POST   | /aliases       | handleCreate  | 创建别名（自动 RestoreFromMeta）|
| PUT    | /aliases/:id   | handleUpdate  | 更新别名                      |
| DELETE | /aliases/:id   | handleDelete  | 删除别名                      |

## 部署 /deployments

| Method | Path                                    | Handler                    | 说明                                    |
|--------|-----------------------------------------|----------------------------|-----------------------------------------|
| POST   | /deployments                            | handleDeploy               | 执行部署                                |
| GET    | /deployments                            | handleList                 | 部署记录列表（page/page_size）           |
| DELETE | /deployments/:id                        | handleUndeploy             | 撤销部署                                |
| GET    | /deployments/targets                    | handleTargets              | 按目标路径聚合查询（?type=过滤）         |
| GET    | /deployments/health                     | handleCheck                | 健康检查（返回 broken items）            |
| POST   | /deployments/check-path                 | handleCheckPath            | 检查路径是否存在                         |
| POST   | /deployments/check-conflicts            | handleCheckConflicts       | MCP 预检冲突（不写文件）                 |
| POST   | /deployments/open-folder                | handleOpenFolder           | 在系统文件管理器中打开路径               |
| POST   | /deployments/:id/repair                 | handleRepair               | 修复异常部署子项                         |
| DELETE | /deployments/:id/items/:item_id         | handleCleanItem            | 清理子项（?undeploy=true 同时撤销文件）  |
| GET    | /deployments/by-resource/:resourceId    | handleResourceDeployTargets| 查资源已部署到的所有目标                 |

## WebSocket /ws

| Method | Path    | Handler   | 说明                          |
|--------|---------|-----------|-------------------------------|
| GET    | /ws     | HandleWS  | WebSocket 升级（事件推送）     |

事件类型:
- `deploy:synced` — 分组资源变动触发追踪部署同步后广播
- `file:changed` — 资源文件内容变化通知

## 数据 /data

| Method | Path           | Handler       | 说明                          |
|--------|----------------|---------------|-------------------------------|
| GET    | /data/export   | handleExport  | 导出全部数据                  |
| POST   | /data/import   | handleImport  | 导入数据                      |

## 健康检查

| Method | Path         | Handler | 说明       |
|--------|--------------|---------|-----------|
| GET    | /health      | Health  | 服务存活检查 |

## 统一响应格式

```json
// 成功
{"code": 0, "data": ..., "msg": "ok"}

// 失败
{"code": 1001, "data": null, "msg": "资源不存在"}

// 带附加数据的失败
{"code": 3002, "data": {"conflicts": [...]}, "msg": "与已有内容存在 key 冲突"}
```
