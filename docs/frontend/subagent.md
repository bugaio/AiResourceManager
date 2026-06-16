# SubAgent 模块功能与交互

## 概述

SubAgent 是三种资源类型之一 (`type='agent'`)，代表 Claude Code 的子代理定义文件。每个 SubAgent 是一个 markdown 文件，部署时通过 symlink 链接到目标路径。

前端显示名称为 **SubAgent** (非 agent)。

---

## 文件结构

```
storage/agents/{uuid}.md
```

- 单个 markdown 文件 (不同于 skill 的目录结构)
- 文件名即 uuid，扩展名 `.md`
- 用户通过 Monaco Editor 编辑 markdown 内容

---

## 部署方式

| 属性 | 值 |
|------|-----|
| deploy_type | `symlink` |
| 部署单位 | 单文件 |
| link_path | `{target_path}/{resource_name}.md` |
| 产物 | 目标路径下出现一个 `.md` 文件的 symlink |

**与 Skill 的区别:**
- Skill: symlink 整个目录 → `{target}/{name}` (目录)
- SubAgent: symlink 单文件 → `{target}/{name}.md` (文件)

### 部署流程

1. 用户在 DeployDialog 中选择目标路径
2. 后端在目标路径下创建 symlink → 指向 `storage/agents/{uuid}.md`
3. link_path 格式为 `{target_path}/{resource_name}.md`

### 撤销

- 删除 symlink 文件
- 删除 deployment_items 记录
- 与 Skill 撤销逻辑完全一致

---

## 编辑器

- 语言: `markdown`
- 通过 EditorDrawer 打开
- 触发方式: 卡片菜单"编辑内容" 或 双击卡片
- 保存: Ctrl/Cmd+S 或点击保存按钮
- 无格式化按钮
- 保存后无同步弹窗

---

## 页面交互

SubAgent 与 Skill **共用同一套前端组件和交互逻辑**，仅以下维度不同:

### 相同点 (与 Skill 完全一致)

- 视图模式: Grid / List
- 搜索、全选、新建流程
- 卡片右键菜单: 编辑 / 编辑内容 / 部署 / 在文件管理器中打开 / 删除
- 批量操作: 批量部署 / 关联分组 / 批量删除
- 删除逻辑: 检查部署 → 确认 → 级联撤销 + 删文件 + 删记录
- 部署弹窗: 多目标路径选择，冲突时简单 confirm 覆盖
- 分组: 颜色圆点 + 名称 + 数量，关联/移除
- 侧栏目标路径: 展开显示已部署项，健康检查，操作按钮

### 不同点

| 维度 | Skill | SubAgent |
|------|-------|----------|
| 卡片徽标颜色 | 蓝色 | **紫色** |
| 文件结构 | 目录 (`skills/{uuid}/`) | 单文件 (`agents/{uuid}.md`) |
| symlink 产物 | 目录 symlink | 文件 symlink (.md) |
| link_path | `{target}/{name}` | `{target}/{name}.md` |

---

## 卡片徽标

ResourceCard 组件根据 `resource.type` 动态设置徽标颜色:

| type | 颜色 | 类名 |
|------|------|------|
| skill | 蓝色 | 蓝色系 |
| mcp | 绿色 | 绿色系 |
| agent | 紫色 | 紫色系 |

---

## 分组

- 分组按 type 隔离: SubAgent 分组只在 SubAgent Tab 下可见
- 操作逻辑与 Skill 完全一致
- `resource_count` 统计的是该分组下 agent 类型资源的数量

---

## 侧栏目标路径

- 与 Skill 完全一致的 TargetList/TargetItem 组件
- 展开后显示 `.md` 文件 symlink 列表
- 健康检查: 检查 symlink 是否指向有效的 `.md` 文件
- 分组标签: 仅当资源当前仍在该分组时显示

---

## 别名

- 路径别名按 `type='agent'` 隔离
- 无路径后缀限制 (不像 MCP 要求 .json)
- 跨类型可同名

---

## 设计意图

SubAgent 与 Skill 高度同构，目的是:
1. 共用组件降低维护成本 (一套 ResourceCard/ResourceGrid/DeployDialog 通吃)
2. 通过 `type` 字段 switch 区分少量差异 (文件结构、link_path 后缀、徽标色)
3. 用户心智模型一致: 学会 Skill 操作即会 SubAgent 操作
