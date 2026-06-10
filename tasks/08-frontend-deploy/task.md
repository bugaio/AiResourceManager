# 任务08: 前端部署交互与目标路径管理

## 目标

实现前端的部署操作流程（部署弹窗、目标路径选择、track选项）、左侧栏下部的目标路径列表（显示已部署资源、支持撤销）。

## 交付内容

### 组件
- `src/components/deploy/DeployDialog.vue` — 部署弹窗（选择目标路径 + track选项 + 内容预览）
- `src/components/sidebar/TargetList.vue` — 目标路径列表容器
- `src/components/sidebar/TargetItem.vue` — 单个目标路径（展开显示已部署资源）
- `src/components/common/PathInput.vue` — 路径输入组件（支持选择已注册别名或手动输入）

### 状态管理
- `src/stores/deploy.ts` — 部署状态管理
- `src/stores/alias.ts` — 别名状态（供 PathInput 使用）

### API层
- `src/api/deploy.ts` — 部署相关 API
- `src/api/alias.ts` — 别名 API

### 类型
- `src/types/deploy.ts` — Deployment 接口定义
- `src/types/alias.ts` — PathAlias 接口定义

### 功能点
- 部署弹窗：
  - 从卡片菜单"部署到目录"触发，或批量操作"部署到..."触发
  - 目标路径二选一：下拉选择已注册别名 / 手动输入路径
  - 分组部署时显示 track 选项开关："跟踪分组变化（自动同步增减）"
  - 显示即将部署的资源列表预览
  - 确认后调 POST /api/v1/deployments（含 track 字段）
  - 冲突时弹出确认框：覆盖(force=true) / 取消
  - 成功后刷新侧栏目标路径列表

- 侧栏下部目标路径列表：
  - 调 GET /api/v1/targets 获取数据
  - 按路径分组展示，显示别名（有的话）
  - 展开显示已部署资源：
    - 整组引用标记 [组] + 分组名 + track标识(🔄)，✕ 按钮整组移除
    - 单个引用显示资源名，✕ 按钮单个移除
  - ✕ 撤销需二次确认 Dialog
  - 撤销后调 DELETE /api/v1/deployments/:id，刷新列表

- 分组菜单"部署到目录"：
  - 同部署弹窗流程，但 body 传 group_id
  - 显示 track 选项

- WebSocket 事件响应：
  - 收到 `deploy:synced` → 自动刷新目标路径列表 + toast提示

- 健康检查 & Broken 状态：
  - 侧栏目标路径列表标题旁有"检查"按钮，点击调 POST /api/v1/deployments/check
  - GET /api/v1/targets 返回的每个 item 含 status 字段
  - status="broken" 的资源显示 ⚠️ 图标 + 橙色文字
  - broken 项操作按钮变为：
    - "修复" → 调 POST /api/v1/deployments/:id/repair → 成功后刷新列表
    - "清理" → 调 DELETE /api/v1/deployments/:id/items/:item_id → 删除记录

## 验收标准

1. 点击"部署到目录"弹出 Dialog，选择别名或输入路径后部署成功
2. 分组部署时 track 开关可见且正常工作
3. 部署后侧栏下部立即显示对应目标路径和资源
4. track 部署显示 🔄 标识
5. 冲突时弹出覆盖确认，选择覆盖后成功
6. 点击 ✕ 撤销，二次确认后资源从列表消失
7. 整组部署的只能整组撤销
8. 批量选中多资源一次性部署正常
9. WebSocket 推送 deploy:synced 时自动刷新列表
10. 手动删除 symlink 后，列表中该项显示 ⚠️ broken 状态
11. 点击"修复"成功重建链接，状态恢复为 ok
12. 点击"清理"删除记录，项从列表消失
