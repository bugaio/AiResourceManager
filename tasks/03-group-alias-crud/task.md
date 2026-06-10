# 任务03: 分组与别名 CRUD

## 目标

实现分组管理和路径别名管理的完整 CRUD，包括分组-资源多对多关联操作。分组资源关联变化时需检查 track 部署并触发自动同步。

## 交付内容

### 后端
- `internal/handler/group.go`（分组 CRUD + 资源关联操作）
- `internal/service/group.go`（分组业务逻辑 + track 同步触发）
- `internal/repo/group.go`（分组数据库操作，含关联表）
- `internal/model/group.go`（Group结构体 + DTO）
- `internal/handler/alias.go`（别名 CRUD）
- `internal/service/alias.go`（别名业务逻辑，含路径合法性校验 + 跨平台路径展开）
- `internal/repo/alias.go`（别名数据库操作）
- `internal/model/alias.go`（PathAlias结构体 + DTO）

### API — 分组

| 方法 | 路径 | 功能 |
|------|------|------|
| GET | /api/v1/groups | 列表（?type=skill&page=1&page_size=20） |
| POST | /api/v1/groups | 创建分组 |
| PUT | /api/v1/groups/:id | 更新（重命名/排序） |
| DELETE | /api/v1/groups/:id | 删除分组 |
| POST | /api/v1/groups/:id/resources | 批量添加资源到分组 |
| DELETE | /api/v1/groups/:id/resources/:rid | 移除资源 |

### API — 别名

| 方法 | 路径 | 功能 |
|------|------|------|
| GET | /api/v1/aliases | 列表 |
| POST | /api/v1/aliases | 创建 |
| PUT | /api/v1/aliases/:id | 更新 |
| DELETE | /api/v1/aliases/:id | 删除 |

### 分组跟踪同步逻辑

当分组资源关联发生变化时：

```
POST /api/v1/groups/:id/resources（添加资源）:
  1. 写入 group_resource 关联
  2. 查询 deployment 表: WHERE group_id=:id AND track=1
  3. 对每条 track 部署记录，自动为新增资源执行部署到对应 target_path
  4. 写入新的 deployment_item
  5. 更新目标路径 _meta.json
  6. 通过 WebSocket Hub 广播 deploy:synced 事件

DELETE /api/v1/groups/:id/resources/:rid（移除资源）:
  1. 查询 deployment 表: WHERE group_id=:id AND track=1
  2. 对每条 track 部署记录，自动撤销该资源在对应 target_path 的部署
  3. 删除对应 deployment_item
  4. 更新目标路径 _meta.json
  5. 删除 group_resource 关联
  6. 通过 WebSocket Hub 广播 deploy:synced 事件
```

### 业务规则
- 同一 type 下分组名不可重复
- 别名 name 全局唯一
- 删除分组时仅解除关联，不删除资源本身
- 删除分组时，若有 track 部署记录，需先撤销所有关联部署
- 别名路径校验规则：
  - 不能为空
  - 支持 `~` 展开为 os.UserHomeDir()
  - 路径使用 filepath.Clean() 标准化
  - Windows 下支持 `C:\` 和 `/` 两种分隔符
  - 不要求路径实际存在（用户可能后续创建）

## 验收标准

1. 分组 CRUD 全部接口调通，支持分页
2. 资源可添加到多个分组、从分组移除
3. 查询资源列表时 group_id=0 返回全部，group_id=N 筛选
4. 别名 CRUD 全部接口调通
5. 重复名称创建返回错误码 2002/4002
6. 删除分组后 group_resource 关联记录被级联清理
7. 添加资源到有 track 部署的分组时，自动部署到目标路径
8. 从有 track 部署的分组移除资源时，自动撤销对应部署
9. 自动同步后 WebSocket 推送 deploy:synced 事件
10. 别名路径 ~ 展开正确，路径标准化无多余斜杠
11. 每个文件顶部有模块功能注释，每个函数有入参/返回值中文注释
