# 任务06: 资源编辑与Monaco编辑器

## 目标

实现资源的新建表单、信息编辑、Monaco编辑器抽屉（读写文件内容），完成资源的完整创建和编辑流程。

## 交付内容

### 组件
- `src/components/editor/EditorDrawer.vue` — 编辑器抽屉（右侧滑出，默认70%宽度，可拖拽左边缘调整）
- `src/components/editor/MonacoEditor.vue` — Monaco编辑器封装（语言自动切换：md/json）
- `src/components/resource/ResourceForm.vue` — 新建/编辑资源Dialog（名称、描述、分组选择、mini编辑器）
- `src/components/common/GroupSelect.vue` — 分组多选弹窗（checkbox列表 + 快速新建分组）

### 功能点
- 新建资源 Dialog：
  - 名称（必填）+ 描述（选填）+ 分组选择（多选）+ mini Monaco编辑器
  - 创建后调 POST /api/v1/resources，成功后刷新列表
  - Skill 类型：创建后初始内容为空的 SKILL.md
  - MCP 类型：创建后初始内容为 jsonc 模板（含 serverName 占位）
  - Agent 类型：创建后初始内容为空 .md
- 编辑信息 Dialog：
  - 修改名称、描述
  - 调 PUT /api/v1/resources/:id
- Monaco 编辑器抽屉：
  - 点击卡片"编辑内容"或双击卡片触发
  - 从右侧滑出，宽度70%，左边缘可拖拽调整
  - 自动根据资源类型设置语言（skill/agent → markdown，mcp → json）
  - 加载内容：GET /api/v1/resources/:id/content
  - 保存：PUT /api/v1/resources/:id/content
  - Ctrl+S / Cmd+S 快捷键保存
  - 未保存关闭时弹出确认提示
  - Monaco 主题跟随全局亮暗切换（vs / vs-dark）
- 分组选择弹窗：
  - 展示当前 type 所有分组（checkbox多选）
  - 显示资源当前所在分组（已勾选）
  - 底部"+ 新建分组"快速创建
  - 确认后调接口更新关联

### 删除确认增强
- 删除资源时：
  - 调 DELETE /api/v1/resources/:id（不带 confirm）
  - 若返回 code=1004（有关联部署）→ 展示关联信息弹窗（部署到了哪些路径）
  - 用户确认 → 带 confirm=true 再次调用
  - 成功后刷新列表 + 侧栏目标路径列表

## 验收标准

1. 点击新建按钮，弹出表单，填写后创建成功，列表中出现新资源
2. Monaco 抽屉打开，正确加载文件内容
3. 编辑后 Ctrl+S 保存，关闭抽屉重新打开确认内容已持久化
4. skill/agent 文件显示 Markdown 语法高亮，mcp 文件显示 JSON 高亮
5. 未保存关闭时弹出确认框
6. 抽屉宽度可拖拽调整
7. 暗色主题下 Monaco 自动切换为 vs-dark
8. 删除有关联部署的资源时，展示关联信息并要求确认
