# Skill 模块功能与交互

## 概述

Skill 是三种资源类型之一 (`type='skill'`)，代表 Claude Code 的 skill 文件。每个 skill 由一个目录组成，包含 SKILL.md 内容文件和 meta.json 元数据。

---

## 文件结构

```
storage/skills/{uuid}/
├── SKILL.md      # skill 内容 (用户编辑的 markdown)
├── meta.json     # 元数据 (name, description, created_at 等)
```

- `uuid` 由后端生成，全局唯一
- SKILL.md 是用户通过 Monaco Editor 编辑的主体内容
- meta.json 存储资源基本信息，前端不直接操作

---

## 部署方式

| 属性 | 值 |
|------|-----|
| deploy_type | `symlink` |
| 部署单位 | 整个目录 |
| link_path | `{target_path}/{resource_name}` |
| 产物 | 目标路径下出现一个同名目录的 symlink |

**部署流程:**
1. 用户在 DeployDialog 中选择一个或多个目标路径
2. 后端在目标路径下创建 symlink → 指向 `storage/skills/{uuid}/`
3. link_path 记录到 deployment_items 表

**撤销(undeploy):**
- 删除 symlink 文件
- 删除 deployment_items 记录

---

## 编辑器

- 语言: `markdown`
- 通过 EditorDrawer 打开
- 触发方式: 卡片菜单"编辑内容" 或 双击卡片
- 保存: Ctrl/Cmd+S 或点击保存按钮
- 无格式化按钮 (仅 MCP 类型有)
- 保存后无同步弹窗 (仅 MCP 类型有)

---

## 页面交互

### 资源展示区 (右侧主区域)

- **视图模式:** Grid (网格卡片) / List (列表表格)，通过 TopBar 切换
- **搜索:** 顶部搜索框，300ms 防抖过滤
- **全选:** 搜索框旁的全选 checkbox
- **新建:** "新建" 按钮打开 ResourceForm 弹窗 (name + description + 可选分组)
- **分页:** 底部 Element Plus Pagination

### 卡片 (ResourceCard)

- **徽标:** 蓝色 (skill 类型标识)
- **双击:** 打开编辑器抽屉编辑内容
- **右键菜单 (dropdown):**
  - 编辑 — 打开 ResourceForm 修改 name/description
  - 编辑内容 — 打开 EditorDrawer
  - 部署 — 打开 DeployDialog
  - 在文件管理器中打开 — 调用 `openFolder` API (macOS: `open -R`)
  - 删除 — 确认后删除

### 批量操作 (BatchBar)

选中多个资源后底部浮出操作栏:
- **批量部署** — 打开 DeployDialog，批量部署所选资源到目标路径
- **关联分组** — 弹出分组选择 popover，将选中资源加入目标分组
- **批量删除** — 确认后删除所有选中资源 (含级联撤销部署)

---

## 删除逻辑

1. 检查该资源是否有关联部署记录
2. 弹出确认对话框 (若有部署会提示"将同时撤销已部署的 symlink")
3. 确认后:
   - 撤销所有关联 symlink (删除目标路径下的 symlink)
   - 删除 `storage/skills/{uuid}/` 目录
   - 删除数据库记录 (resources + deployment_items)
4. 触发 WebSocket 通知 `resource:deleted`

---

## 部署弹窗 (DeployDialog)

### 流程

1. 打开弹窗，可输入路径或从别名下拉选择
2. 点击"添加"将路径加入待部署列表
3. 可勾选"保存为别名" + "追踪变更"
4. 点击确认部署
5. **冲突处理:** 若目标路径已存在同名文件/目录 → 简单 confirm 覆盖 (不走 McpConflictDialog)

### 多目标

- 可同时添加多个目标路径
- 每个路径独立部署
- PathInput 组件支持别名快选 (排除已在列表中的别名)

---

## 分组

### 显示

- GroupItem: 颜色圆点 + 分组名称 + 资源数量
- 颜色: 创建时从 20 色池随机选取
- `resource_count`: 后端动态计算，非前端计数

### 操作

- 创建分组: Sidebar 顶部 "创建分组" 按钮，输入名称即可
- 重命名/删除: GroupItem 下拉菜单
- 关联资源: 创建时 GroupSelect 多选 / BatchBar 批量关联
- 移除资源: 在特定分组视图下，卡片菜单显示"从分组移除"替代"删除"

### 按分组过滤

- 点击 GroupItem 切换到该分组视图
- `resource.currentGroupId` 传入 API 过滤
- 点击"全部"回到无分组过滤

---

## 侧栏目标路径

### TargetList

- 合并两类数据: 已有部署目标 (targets) + 纯别名 (aliases)
- 按创建时间倒序排列

### TargetItem

- **展开:** 显示该目标路径下已部署的所有 skill 列表
- **子项信息:** 资源名 + 状态(ok/broken) + 分组标签
- **分组标签:** 仅当资源当前仍在该分组时显示 (IsResourceInGroup 实时判断)
- **操作按钮:**
  - 检查 — 健康状态检查 (symlink 是否正常)
  - 更多 — 下拉菜单: 清空部署 / 删除路径 / 转别名 / 在文件管理器中打开
  - 移除 — 常显按钮，撤销单个部署项

### 健康检查

- 检查 symlink 是否指向有效目标
- 状态: `ok` (正常) / `broken` (断链)
- broken 状态可修复 (repair: 重建 symlink) 或清理 (clean: 删记录)
