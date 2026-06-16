# 资源服务 (service/resource.go)

## 概述

三种资源类型 `skill` / `agent` / `mcp` 共用同一张 `resource` 表，通过 `type` 字段区分。ResourceService 负责资源的创建、删除、内容读写，以及文件系统操作。

## 结构体

```go
type ResourceService struct {
    repo      *repo.ResourceRepo
    baseDir   string           // ~/.aiManager
    deploySvc *DeployService   // 后注入，解决循环依赖
}
```

## SetDeployService

```go
func (s *ResourceService) SetDeployService(deploySvc *DeployService)
```

在 `serve.go` 中，先创建 ResourceService 和 DeployService，再互相注入。目的：删除资源时需要级联撤销部署，而 DeployService 又依赖 ResourceRepo。

## CreateResource

流程：
1. **校验** — type 必须为 skill/agent/mcp，name 不能为空
2. **重名检查** — 同类型下 name 唯一（`CheckNameExists`）
3. **生成 UUID** — `util.NewUUID()`
4. **创建文件** — `createFiles(type, name, uuid, description)` 按 type 分流
5. **写 DB** — `InsertResource`
6. **关联分组** — 若请求带 `group_id`，写 `group_resource` 表

### 文件创建（createFiles）

| type  | 目录                          | 产出文件                    |
|-------|-------------------------------|-----------------------------|
| skill | `~/.aiManager/skills/{uuid}/` | `SKILL.md` + `meta.json`   |
| agent | `~/.aiManager/agents/`        | `{uuid}.md`                |
| mcp   | `~/.aiManager/mcps/`          | `{uuid}.jsonc`（含注释模板）|

**skill 模板**:
- `SKILL.md`: `# {name}\n\n`
- `meta.json`: `{name, description, version: "1.0.0"}`

**mcp 模板** (.jsonc):
```jsonc
{
  // mcp 配置
  // 示例
  /*
  "chrome-devtools": {
    "command": "npx",
    "args": ["-y", "chrome-devtools-mcp@latest", "--autoConnect"]
  }
  */
  "mcpServers": {}
}
```

## getContentPath

根据类型确定「内容文件」的实际路径：

| type        | 逻辑                         |
|-------------|------------------------------|
| skill       | `path/SKILL.md`（path 是目录）|
| agent / mcp | `path`（path 就是文件本身）   |

## GetContent / UpdateContent

- `GetContent(id)`: 查 DB 获取 path → `getContentPath` → `os.ReadFile`
- `UpdateContent(id, content)`: 查 DB → `getContentPath` → `os.WriteFile`

## DeleteResource

流程：
1. 查资源是否存在
2. 查关联部署（`GetResourceDeployments`）
3. 若有部署且 `confirm=false` → 返回部署列表 + `ErrResourceHasDeploy`
4. 若 `confirm=true`:
   - 逐个调用 `deploySvc.UndeployResourceFromTarget` 级联撤销
   - `deleteFiles` 删除文件系统
   - `repo.DeleteResource` 删 DB 记录（级联删 group_resource）

### deleteFiles

| type  | 操作                |
|-------|---------------------|
| skill | `os.RemoveAll(path)` — 整目录删除 |
| agent/mcp | `os.Remove(path)` — 单文件删除 |

## BatchDelete

逐个调用 `DeleteResource`，收集每条结果（success/fail + code + msg + deployments）。不是事务级批量，失败不回滚。
