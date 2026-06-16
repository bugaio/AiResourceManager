# MCP 模块功能与交互

## 概述

MCP 是三种资源类型之一 (`type='mcp'`)，代表 Claude Code 的 MCP Server 配置片段。每个 MCP 资源是一个 JSONC 文件，部署时通过深度合并写入目标 JSON 配置文件。

---

## 文件结构

```
storage/mcps/{uuid}.jsonc
```

- 单文件，JSONC 格式 (JSON with Comments)
- 用户通过 Monaco Editor 编辑
- 支持注释，方便用户标注配置说明

### 初始模板

新建 MCP 资源时，编辑器内容预填注释示例:
```jsonc
// MCP Server 配置
// 示例:
// {
//   "my-server": {
//     "command": "npx",
//     "args": ["-y", "@my/mcp-server"]
//   }
// }
```

---

## 部署方式

| 属性 | 值 |
|------|-----|
| deploy_type | `merge` |
| 部署单位 | JSON 键值对 |
| 目标文件 | 必须是 `.json` 文件 |
| 合并策略 | lodash 式深度合并 (DeepMerge) |

### 目标路径校验

- **前端校验:** PathInput/AliasesView 中路径必须以 `.json` 结尾
- **后端校验:** 目标文件必须已存在 (不自动创建)

### 部署流程

1. 解析 JSONC 资源内容为 JSON 对象
2. 读取目标 `.json` 文件现有内容
3. 深度合并 (资源内容的每个顶层 key 合并进目标文件)
4. 写回目标文件
5. 记录 deployment_items (link_path = 目标文件路径)

### 撤销 (undeploy)

**关键区别:** 不是简单删除文件，而是从目标 JSON 中移除该资源贡献的所有 key。

- 调用 `removeMCPResourceKeys`: 读取目标 JSON → 删除该资源的所有顶层 key → 写回
- 注意: 删的是资源实际包含的 key，不仅是 deployment_items 表中 link_path 记录的路径

---

## 编辑器

- 语言: `json` (Monaco 配置 `allowComments: true` 支持 JSONC)
- **格式化按钮:** EditorDrawer 中仅 MCP 类型显示，点击自动格式化 JSON
- **保存后同步:** 保存内容后弹出询问"是否同步到已部署路径？" → 打开 SyncDeployDialog

---

## 冲突检测

### 预检 API (`checkConflicts`)

- 触发时机: DeployDialog 确认部署时 (MCP 类型专属流程)
- 特点: **不写文件**，纯检测
- 返回: 冲突列表，每项包含 resource_name, target_path, status, group 信息

### 冲突分组 (Union-Find)

后端使用 Union-Find 算法将有 key 重叠的资源归为同一冲突组:
- `group > 0`: 有依赖关系的资源
- `group = 0`: 无冲突

### 冲突类型/状态

| 状态 | 颜色 | 含义 |
|------|------|------|
| `ignored` | 红色 | 本次部署将被忽略的资源 (key 被覆盖) |
| `applied` | 绿色 | 本次部署将生效的资源 (最终写入的) |
| `existing` | 黄色/琥珀 | 目标文件中已存在的资源 (不在本次部署中) |

### McpConflictDialog 弹窗

**布局:**
1. 按目标路径分组展示
2. 同一冲突组内的资源用虚线框圈起
3. 非冲突组 (`group=0`) 的资源单独列出
4. 底部图例说明颜色含义

**交互:**
- 复选框选择要处理的目标路径 (默认全选)
- 全选/反选，支持 indeterminate 状态
- 确认后: 仅对选中路径执行 force 重新部署 (只部署 applied 状态的资源)

---

## 保存后同步 (SyncDeployDialog)

### 触发条件

编辑器保存 MCP 内容后，若该资源已有部署记录，弹出同步弹窗。

### 交互流程

1. 加载该资源的所有已部署目标路径 (`getResourceDeployTargets`)
2. 列出所有路径，带冲突标记
3. 默认选中无冲突的路径，有冲突的默认不选
4. 用户可手动勾选/取消
5. 确认后: 对选中路径执行 `force: true` 重新部署 (用最新内容覆盖)

---

## 健康检查

与 Skill/Agent 不同，MCP 的健康检查逻辑:

- 检查目标 JSON 文件的**顶层**或 `mcpServers` 字段下是否仍包含该资源的 key
- 若 key 已被外部修改/删除 → 状态为 `broken`
- repair: 重新合并写入
- clean: 删除 deployment_items 记录 (不改目标文件)

---

## 别名

- 路径别名按 `type='mcp'` 隔离
- MCP 别名的路径**必须以 `.json` 结尾** (前端 AliasesView 创建/编辑时校验)
- 跨类型可同名 (skill 别名 "config" 和 mcp 别名 "config" 互不冲突)

---

## 与 Skill/Agent 的关键差异

| 维度 | Skill/Agent | MCP |
|------|-------------|-----|
| 部署方式 | symlink | deep merge 到 JSON |
| 目标类型 | 目录路径 | .json 文件 |
| 冲突处理 | 简单 confirm 覆盖 | 预检 API + Union-Find 分组 + McpConflictDialog |
| 撤销方式 | 删 symlink | 从目标 JSON 移除 key |
| 编辑器功能 | 无格式化 | 有格式化按钮 |
| 保存后行为 | 无 | 询问同步到已部署路径 |
| 健康检查 | symlink 是否有效 | key 是否仍存在于目标 JSON |
| 路径校验 | 无特殊限制 | 必须 .json 结尾 |
