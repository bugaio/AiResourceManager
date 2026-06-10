# 任务05: 前端资源展示与分组交互

## 目标

实现左侧栏分组列表（含齿轮配置）和右侧内容区资源卡片/列表展示，对接后端 API，完成资源浏览、分组筛选、分页的完整交互。

## 交付内容

### 组件 — 侧栏
- `src/components/sidebar/GroupList.vue` — 分组列表容器（"全部" + 自定义分组）
- `src/components/sidebar/GroupItem.vue` — 单个分组项（名称 + ••• 操作菜单）
- 侧栏顶部齿轮弹窗：新增分组、配置默认显示

### 组件 — 内容区
- `src/components/resource/ResourceGrid.vue` — 网格容器
- `src/components/resource/ResourceCard.vue` — 资源卡片（名称、描述、分组chip、部署状态、修改时间、多选框、•••菜单）
- `src/components/resource/ResourceList.vue` — 列表容器（表格形式）
- `src/components/resource/ResourceRow.vue` — 列表行
- `src/components/resource/EmptyState.vue` — 空状态引导页
- `src/components/common/BatchBar.vue` — 批量操作栏（选中时出现）

### 状态管理
- `src/stores/resource.ts` — 资源列表状态 + 搜索/筛选 + 分页状态
- `src/stores/group.ts` — 分组列表状态
- `src/stores/selection.ts` — 多选状态

### API层
- `src/api/resource.ts` — 资源 API 调用（含分页参数）
- `src/api/group.ts` — 分组 API 调用

### 类型
- `src/types/resource.ts` — Resource 接口定义
- `src/types/group.ts` — Group 接口定义

### 分页组件
- 内容区底部 `el-pagination` 组件
- 显示：总条数 + 每页条数选择(10/20/50) + 页码切换
- Store 中管理 `{page, pageSize, total}`

### 功能点
- 点击分组 → 筛选右侧资源（调接口 ?group_id=N）+ 分页重置
- 点击"全部" → 调接口 ?group_id=0（特殊ID表示不按分组筛选）
- Tab 切换 → 刷新分组列表和资源列表 + 分页重置
- 搜索框输入 → debounce 300ms → 实时筛选 + 分页重置
- 视图切换按钮 → 网格/列表模式
- 卡片多选 → 批量操作栏出现（含批量删除，调 DELETE /api/v1/resources/batch）
- 分组 ••• 菜单：重命名、删除（调接口）
- 空状态：无资源时展示引导图+创建按钮
- 分页切换时保持当前筛选条件

### 卡片样式参考（Tailwind）
```
rounded-xl border border-gray-200 bg-white shadow-sm
hover:shadow-md hover:-translate-y-[1px] transition-all
dark:bg-gray-800 dark:border-gray-700
```

## 验收标准

1. 左侧显示分组列表，点击切换右侧内容
2. 卡片正确显示资源信息（名称、描述、分组标签、部署状态、时间）
3. 网格/列表视图切换正常
4. 搜索筛选实时生效
5. 分页组件正常：切换页码加载对应数据，total正确
6. 切换Tab/分组/搜索时分页重置到第1页
7. 多选+批量操作栏正常出现/消失
8. 新增/删除/重命名分组后列表即时刷新
9. 空状态页面在无数据时正确展示
10. 亮暗主题下卡片样式正确
