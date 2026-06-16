# 项目目录功能划分

## 技术栈

| 类别 | 技术 |
|------|------|
| 框架 | Vue 3.4 Composition API (`<script setup>`) |
| 语言 | TypeScript 5.3 (strict mode) |
| UI 库 | Element Plus |
| 样式 | Tailwind CSS 3.4 (darkMode: 'class', preflight: false) |
| 状态管理 | Pinia |
| 代码编辑器 | Monaco Editor (vite-plugin-monaco-editor, json worker) |
| 构建工具 | Vite 5.1 |
| HTTP 客户端 | Axios |
| 路由 | Vue Router 4 |

## Vite 配置要点

- 路径别名: `@` → `src/`
- 开发服务器端口: 5173
- API 代理: `/api` → `http://localhost:3678`
- WebSocket 代理: `/ws` → `ws://localhost:3678`
- Monaco Editor 单独分包优化

## Tailwind 配置要点

- `darkMode: 'class'` — 通过 HTML class 切换暗色主题
- `corePlugins.preflight: false` — 不重置默认样式，避免与 Element Plus 冲突
- 扫描范围: `src/**/*.{vue,ts,js}`

---

## 目录结构 (`web/src/`)

```
src/
├── api/              # HTTP 请求层
├── components/
│   ├── common/       # 通用业务组件
│   ├── deploy/       # 部署相关弹窗
│   ├── editor/       # Monaco 编辑器
│   ├── layout/       # 全局布局
│   ├── resource/     # 资源展示
│   └── sidebar/      # 侧栏子组件
├── router/           # 路由定义
├── stores/           # Pinia 状态管理
├── styles/           # 全局样式
├── types/            # TS 类型定义
└── views/            # 页面视图
```

---

## 各目录详细说明

### `api/` — Axios 封装 + 模块 API

| 文件 | 职责 |
|------|------|
| `request.ts` | Axios 实例 (baseURL `/api/v1`, 15s 超时)，响应拦截器解包 `{code, msg, data}` 结构，非零 code 抛 ApiError |
| `resource.ts` | 资源 CRUD + 内容读写 + 批量删除 |
| `group.ts` | 分组 CRUD + 资源关联/移除 |
| `deploy.ts` | 部署/撤销/目标列表/健康检查/修复/清理/冲突预检/打开文件管理器 |
| `alias.ts` | 路径别名 CRUD (按 type 过滤) |
| `data.ts` | 数据导入/导出 |
| `ws.ts` | WebSocket 管理器 (单例，自动重连指数退避 1s→30s，心跳 30s ping/pong) |

### `components/common/` — 通用组件

| 组件 | 功能 |
|------|------|
| `BatchBar.vue` | 批量操作栏: 选中数量显示，批量部署/关联分组/批量删除，根据当前是否在特定分组切换"删除"/"从分组移除" |
| `GroupSelect.vue` | 分组多选下拉，用于创建资源时关联分组 |
| `PathInput.vue` | 路径输入组件: 输入框 + 别名快选下拉，支持排除已选别名 |

### `components/deploy/` — 部署相关

| 组件 | 功能 |
|------|------|
| `DeployDialog.vue` | 部署弹窗: 多目标路径选择，MCP 走冲突预检流程，Skill/Agent 走错误驱动覆盖流程 |
| `McpConflictDialog.vue` | MCP 冲突弹窗: Union-Find 分组展示，状态色 (ignored红/applied绿/existing黄)，可选目标路径 |
| `SyncDeployDialog.vue` | 同步部署弹窗: 保存 MCP 后询问已部署路径是否同步更新，默认排除有冲突的路径 |

### `components/editor/` — 编辑器

| 组件 | 功能 |
|------|------|
| `EditorDrawer.vue` | 右侧抽屉: Monaco 编辑器 + 保存/关闭按钮 + 未保存检测 + Ctrl/Cmd+S 快捷键 + 左边缘拖拽调整宽度 + MCP 格式化按钮 + 保存后触发 SyncDeployDialog |
| `MonacoEditor.vue` | Monaco Editor 封装: 支持 markdown/json 语言，json 开启 allowComments (JSONC) |

### `components/layout/` — 布局

| 组件 | 功能 |
|------|------|
| `AppLayout.vue` | 三栏布局容器: TopBar + Sidebar + 主内容区 |
| `TopBar.vue` | 顶部导航: 应用名 + 类型切换按钮 (Skill/MCP/SubAgent) + 暗色主题切换 + 设置菜单 |
| `Sidebar.vue` | 侧栏: 上半区 GroupList (含创建按钮) + 下半区 TargetList (含管理别名按钮) |
| `ResizeDivider.vue` | 拖拽分割线: 调整侧栏宽度 |

### `components/resource/` — 资源展示

| 组件 | 功能 |
|------|------|
| `ResourceCard.vue` | 资源卡片: 类型徽标色(skill蓝/mcp绿/agent紫) + 双击编辑内容 + 右键菜单 + 选择态 |
| `ResourceGrid.vue` | 网格视图: 卡片栅格排列 |
| `ResourceList.vue` | 列表视图: 表格形式展示 |
| `ResourceForm.vue` | 新建/编辑表单弹窗: name + description + 分组选择(创建时) |
| `EmptyState.vue` | 空状态占位 |

### `components/sidebar/` — 侧栏子组件

| 组件 | 功能 |
|------|------|
| `GroupList.vue` | 分组列表: "全部" + 分组项，高亮当前选中，内联重命名/删除弹窗 |
| `GroupItem.vue` | 单个分组: 颜色圆点 + 名称 + 数量 + 下拉菜单(重命名/删除) |
| `TargetList.vue` | 目标路径列表: 合并已部署目标 + 纯别名，按创建时间倒序 |
| `TargetItem.vue` | 单个目标: 可展开子项，健康检查/清空部署/删除路径/转别名/打开文件管理器，部署项可修复/清理 |

### `router/` — 路由

| 路径 | 视图 | 说明 |
|------|------|------|
| `/` | — | 重定向到 `/resources` |
| `/resources` | ResourcesView.vue | 资源管理主页 |
| `/aliases` | AliasesView.vue | 别名管理 |
| `/data` | DataView.vue | 数据导入/导出 |

所有路由组件均为懒加载 (`() => import(...)`)。

### `stores/` — Pinia 状态管理

| Store | 职责 |
|-------|------|
| `ui` | 当前类型/视图模式/主题/侧栏宽度，持久化到 localStorage |
| `resource` | 资源列表 + 分页 + 搜索(300ms 防抖) + 按 type/group 过滤 |
| `selection` | 选中 ID 集合 + 全选/反选 |
| `group` | 分组列表 CRUD + 资源关联 |
| `deploy` | 部署目标列表 + 部署/撤销/健康检查/修复/清理 |
| `alias` | 路径别名 CRUD (按 type 隔离) |

**跨 Store 联动:**
- `ui.currentType` 变化 → resource/group/alias/deploy 自动 refetch
- group 删除 → deploy.fetchTargets() 刷新
- selection 在切组时自动清空

### `types/` — 类型定义

| 文件 | 核心类型 |
|------|----------|
| `resource.ts` | `Resource`, `ResourceType` ('skill'\|'mcp'\|'agent'), `PaginatedList<T>` |
| `group.ts` | `Group` (id, name, type, color, sort_order, resource_count) |
| `alias.ts` | `PathAlias` (id, name, type, path) |
| `deploy.ts` | `DeploymentItem`, `TargetInfo`, `DeployRequest`, `DeployType` ('symlink'\|'merge') |

### `views/` — 页面视图

| 视图 | 功能 |
|------|------|
| `ResourcesView.vue` | 主页面: 搜索/全选/新建 + Grid/List 切换 + 分页 + 编辑器抽屉 + 部署弹窗 + 批量栏 + WS 消息监听 |
| `AliasesView.vue` | 别名管理: 表格展示 + CRUD 弹窗 + MCP 路径 .json 校验 + 删除级联处理 |
| `DataView.vue` | 数据导入/导出: 路径输入 + 冲突策略选择(覆盖/跳过/保留两者) |
