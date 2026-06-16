# 前端设计规范

项目技术栈：Vue3 + Element Plus + Tailwind CSS（darkMode: class, preflight: false）

## 1. 配色主题

### 亮色模式

| 用途 | 类名 |
|------|------|
| 页面背景 | `bg-white` |
| 侧栏背景 | `bg-white` + `border-gray-200` 分割线 |
| 卡片 | `bg-white` + `border-gray-200` + `shadow-sm` + `rounded-xl` |
| 主文字 | `text-gray-800` |
| 次要文字 | `text-gray-600` |
| 辅助文字 | `text-gray-400` |
| 活跃项 | `bg-blue-100 text-blue-700` |
| 链接/按钮 | `text-blue-500` / `text-blue-600` |

### 暗色模式（dark class）

| 用途 | 类名 |
|------|------|
| 页面背景 | `dark:bg-gray-900` |
| 侧栏 | `dark:bg-gray-900` + `dark:border-gray-700` |
| 卡片 | `dark:bg-gray-800` + `dark:border-gray-700` |
| 主文字 | `dark:text-gray-100` |
| 次要文字 | `dark:text-gray-300` |
| 辅助文字 | `dark:text-gray-500` |
| 活跃项 | `dark:bg-blue-900/40 dark:text-blue-300` |

### 资源类型配色

| 类型 | 亮色 | 暗色 |
|------|------|------|
| skill | `bg-blue-100 text-blue-700` | `bg-blue-900/30 text-blue-300` |
| mcp | `bg-green-100 text-green-700` | `bg-green-900/30 text-green-300` |
| agent | `bg-purple-100 text-purple-700` | `bg-purple-900/30 text-purple-300` |

### 状态色

| 状态 | 样式 |
|------|------|
| 正常 (ok) | `bg-green-400` 圆点 |
| 异常 (broken) | `bg-orange-400` 圆点 + `text-orange-500` 文字 |
| 冲突-忽略 | `bg-red-100 text-red-700` |
| 冲突-应用 | `bg-green-100 text-green-700` |
| 冲突-已有 | `bg-amber-100 text-amber-700` |

### 分组颜色池（20色）

```
#3B82F6, #10B981, #F59E0B, #EF4444, #8B5CF6,
#EC4899, #06B6D4, #84CC16, #F97316, #6366F1,
#14B8A6, #D946EF, #0EA5E9, #22C55E, #E11D48,
#7C3AED, #2DD4BF, #FBBF24, #FB7185, #A78BFA
```

按分组创建顺序依次分配，循环使用。

## 2. 布局与间距

### 整体布局

```
┌─────────────────────────────────────────┐
│ TopBar  h-14 (56px)  border-b           │
├────────────┬────────────────────────────┤
│ 侧栏       │ 主内容区                    │
│ 240px      │                            │
│ 可拖拽      │                            │
│ 180-400px  │                            │
│ border-r   │                            │
│            │                            │
│ ┌────────┐ │                            │
│ │分组区   │ │                            │
│ ├────────┤ │                            │
│ │目标路径 │ │                            │
│ └────────┘ │                            │
└────────────┴────────────────────────────┘
```

- TopBar：`h-14` 固定顶部，`border-b`
- 内容区：`flex`，侧栏 + 主区域
- 侧栏：默认宽 `240px`，可拖拽范围 `180-400px`，`border-r`
- 侧栏内部：上下分割为分组区 + 目标路径区

### 间距规范

| 场景 | 值 |
|------|------|
| 页面内边距 | `px-4` / `px-6` |
| 卡片网格间距 | `gap-4` |
| 卡片内边距 | `p-4` |
| 列表项 | `px-3 py-2` |
| 组件间 | `gap-2` / `gap-3` |
| 按钮间 | `gap-2` |

### 圆角

| 元素 | 值 |
|------|------|
| 卡片 | `rounded-xl`（12px） |
| 按钮/输入框 | Element Plus 默认（4px） |
| 标签/badge | `rounded`（4px） |
| 圆点 | `rounded-full` |

## 3. 字体大小

| 用途 | 类名 | 大小 |
|------|------|------|
| 标题 | `text-lg font-semibold` | 18px |
| 卡片名称 | `text-sm font-medium` | 14px |
| 正文 | `text-sm` | 14px |
| 辅助文字 | `text-xs` | 12px |
| 微型标签 | `text-[10px]` | 10px |

## 4. 组件样式

### 分组标签

```
text-[10px] px-1 py-0.5 rounded text-white
背景色取分组颜色池对应色
```

### 别名标识

```
border border-dashed border-purple-200
bg-purple-50/50
```

紫色系视觉区分，表示该资源为别名引用。

### 操作按钮

```
text-gray-400
hover:text-blue-500  （编辑类）
hover:text-red-500   （删除类）
```

### 编辑器抽屉

```
位置：右侧滑出
宽度：70vw，可拖拽 40-90vw
样式：bg-white shadow-2xl
```

### 弹窗冲突组

```
border border-dashed border-gray-300 rounded
内部包含冲突 tags
```

## 5. 动画

| 场景 | 方式 |
|------|------|
| 抽屉进出 | `opacity 0.2s` + `translateX 0.25s` |
| 按钮/链接 | `transition-colors` |
| 主题切换 | 通过 `class 'dark'` 切换，无过渡动画 |

## 6. Element Plus 定制

| 配置 | 说明 |
|------|------|
| preflight | `false`，不覆盖 Element Plus 基础样式 |
| 主色 | 默认蓝（与 Tailwind blue-500 一致） |
| 组件尺寸 | 大部分使用 `size="small"` |
| 弹窗 | `:close-on-click-modal="false"` 防误关 |
