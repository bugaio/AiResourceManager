# 任务04: 前端布局与路由

## 目标

搭建完整的前端页面骨架：三栏布局、TopBar（Logo+Tab+设置）、可拖拽侧栏、路由切换、主题系统（亮/暗切换），全部使用 Tailwind CSS 实现样式。此阶段不填充实际数据，只搭结构。

## 交付内容

### 组件
- `src/components/layout/AppLayout.vue` — 整体布局容器（TopBar + Sidebar + Content）
- `src/components/layout/TopBar.vue` — 顶部栏（左Logo、中Tab、右主题切换+设置下拉）
- `src/components/layout/Sidebar.vue` — 左侧栏容器（上下两个区域占位）
- `src/components/layout/ResizeDivider.vue` — 可拖拽分割线组件

### 视图
- `src/views/ResourceView.vue` — 主视图（三栏布局页）
- `src/views/AliasView.vue` — 路径别名管理页（带返回按钮）
- `src/views/DataView.vue` — 数据导入导出页（带返回按钮）

### 状态管理
- `src/stores/ui.ts` — UI状态：当前Tab(skill/mcp/agent)、视图模式(grid/list)、主题(light/dark)、侧栏宽度

### 路由
- `src/router/index.ts` — 三条路由：/resources、/aliases、/data

### 样式 (Tailwind)
- 使用 Tailwind `dark:` 前缀实现暗色主题
- HTML 根元素切换 `class="dark"`
- `src/styles/element-override.css` — Element Plus 暗色兼容覆盖
- 侧栏使用 `bg-slate-800`（亮色）/ `bg-slate-950`（暗色）
- 内容区使用 `bg-neutral-100`（亮色）/ `bg-gray-900`（暗色）

### 功能点
- TopBar Tab 切换更新 store 中 currentType
- 主题切换按钮（太阳/月亮）→ 切换 HTML root class="dark" + localStorage 持久化
- 首次加载跟随系统 prefers-color-scheme
- 侧栏宽度可拖拽（最小180px，最大400px），宽度持久化到 localStorage
- 设置下拉菜单：点击跳转 /aliases 或 /data
- /aliases 和 /data 页面左上角有返回按钮回到 /resources

## 验收标准

1. 浏览器打开看到完整三栏布局，TopBar有Tab切换和设置按钮
2. 点击Tab切换，store中状态更新（Vue DevTools确认）
3. 拖拽分割线，侧栏宽度实时变化，刷新后宽度保持
4. 主题切换，页面亮暗切换无闪烁（dark class方案），刷新后主题保持
5. 路由跳转正常：/resources ↔ /aliases ↔ /data
6. Element Plus 暗色主题跟随全局切换
7. Tailwind 样式类正确生效（检查侧栏/内容区背景色）
8. 代码使用中文注释
