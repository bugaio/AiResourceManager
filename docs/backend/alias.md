# 别名服务 (service/alias.go)

## 概述

路径别名为常用部署目标路径提供友好名称，避免每次部署都输入完整路径。别名按资源类型隔离——同一个名称可以分别存在于 skill、agent、mcp 三个命名空间中。

## 结构体

```go
type AliasService struct {
    repo *repo.AliasRepo
}
```

## 按 type 隔离

`path_alias` 表有 `type` 列，唯一约束为 `UNIQUE(type, alias)`。

- 创建别名时必须指定 type
- 查询别名按 type 过滤
- 同名别名在不同 type 下可共存

## CreateAlias

流程:
1. 校验 name 不为空
2. 校验 type 必须为 skill/agent/mcp
3. 校验 path 不为空
4. 同类型内重名检查: `CheckAliasNameExists(name, type, "")`
5. `expandAndCleanPath(path)` 展开并清理路径
6. 生成 UUID → `InsertAlias`

### expandAndCleanPath

```go
func expandAndCleanPath(path string) string
```

- `~` 开头 → 替换为 `os.UserHomeDir()`
- `filepath.Clean()` 规范化路径

## ListAliases

```go
func (s *AliasService) ListAliases(aliasType string) ([]model.PathAlias, error)
```

按 type 过滤。type 为空时返回全部。

## UpdateAlias

- 检查别名是否存在
- 校验 name 和 path 不为空
- 同类型内重名检查（排除自身）
- `expandAndCleanPath` 处理路径
- 更新 DB

## DeleteAlias

- 检查别名是否存在
- `repo.DeleteAlias(id)`

## RestoreFromMeta（handler 层触发）

创建别名时，handler 层调用:
```go
h.deploySvc.RestoreFromMeta(alias.Path, alias.ID)
```

作用: 若该路径下存在 `.aiResource/_meta.json`（由之前的部署操作留下），自动还原部署记录到 DB 并关联到新别名。

## MCP 别名路径约定

MCP 类型的别名路径必须以 `.json` 结尾，因为 MCP 部署需要合并到一个 JSON 文件。该校验由前端负责。

## 数据模型

```go
type PathAlias struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`       // 别名名称（表中列名为 alias）
    Type      string    `json:"type"`       // skill/agent/mcp
    Path      string    `json:"path"`       // 展开后的绝对路径（表中列名为 target_path）
    CreatedAt time.Time `json:"created_at"`
}

type CreateAliasReq struct {
    Name string `json:"name" binding:"required"`
    Type string `json:"type" binding:"required"`
    Path string `json:"path" binding:"required"`
}

type UpdateAliasReq struct {
    Name string `json:"name" binding:"required"`
    Path string `json:"path" binding:"required"`
}
```
