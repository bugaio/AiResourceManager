# 通用功能（跨模块共享）

## Tab 切换

**位置:** TopBar 中央三个按钮 (Skill / MCP / SubAgent)

**切换流程:**
1. 用户点击 Tab 按钮
2. 调用 `uiStore.setType(type)` 修改 `currentType`
3. 以下 store 的 watcher 自动响应:
   - `resourceStore` → 重置分页，refetch 资源列表
   - `groupStore` → refetch 当前 type 的分组
   - `aliasStore` → refetch 当前 type 的别名
   - `deployStore` → refetch 当前 type 的目标路径
   - `selectionStore` → 清空选中

**数据隔离:** 所有数据按 type 过滤，切 Tab 等于切到一个独立的工作空间。

---

## 别名管理 (AliasesView)

### 数据结构

```typescript
interface PathAlias {
  id: number
  name: string
  type: ResourceType  // 'skill' | 'mcp' | 'agent'
  path: string
  created_at: string
}
```

### 隔离机制

- `path_alias` 表有 `type` 列
- 前端按当前 `ui.currentType` 过滤显示
- **跨类型可同名:** skill 别名 "prod" 和 mcp 别名 "prod" 互不冲突

### MCP 专属校验

- 创建/编辑 MCP 别名时，路径必须以 `.json` 结尾
- 前端校验 + 后端二次校验

### 页面功能

- 搜索过滤
- 表格展示: 名称 / 路径 / 创建时间
- CRUD 操作弹窗
- 删除时检查是否有部署记录使用该别名 → 级联处理

---

## 分组

### 数据结构

```typescript
interface Group {
  id: number
  name: string
  type: ResourceType
  color: string        // hex 颜色值
  sort_order: number
  resource_count: number  // 动态计算
  created_at: string
}
```

### 颜色分配

- 创建时从 **20 色池**随机选取
- 颜色在 GroupItem 中以小圆点形式展示
- 用于 TargetItem 中的分组标签底色

### resource_count

- 后端动态计算 (不是前端本地计数)
- 随分组列表 API 一起返回
- 资源增删/关联/移除后自动更新

### 操作

| 操作 | 触发位置 | 说明 |
|------|----------|------|
| 创建 | Sidebar 顶部按钮 | 输入名称，自动分配颜色 |
| 重命名 | GroupItem 下拉菜单 | 内联编辑 |
| 删除 | GroupItem 下拉菜单 | 确认弹窗，删除后 refetch targets |
| 关联资源 | ResourceForm (创建时) / BatchBar | 支持多选 |
| 移除资源 | ResourceCard 菜单 (分组视图下) | 单个移除 |

---

## 目标路径列表 (TargetList)

### 数据合并

TargetList 展示两类数据的有序集合:
1. **已部署目标** — deployStore.targets (有实际部署记录的路径)
2. **纯别名** — aliasStore.aliases 中未被部署的路径

### 排序

- 按创建时间倒序 (最新创建的在最上面)
- computed `orderedItems` 合并后统一排序

### TargetItem 交互

| 操作 | 说明 |
|------|------|
| 展开/折叠 | 显示/隐藏该路径下的部署子项 |
| 检查 | 健康状态检查 (symlink 有效性 / MCP key 存在性) |
| 更多 → 清空部署 | 移除该路径下当前 type 的所有部署 |
| 更多 → 删除路径 | 删除该别名记录 (若有) + 清空部署 |
| 更多 → 转别名 | 将目标路径保存为别名 |
| 更多 → 在文件管理器中打开 | 调用系统命令打开 |
| 移除按钮 | 常显 (非 hover 才出现)，撤销单个部署项 |

### 部署子项

展开后每个子项显示:
- 资源名称
- 状态标记 (ok = 正常, broken = 异常)
- 分组标签 (仅当资源**当前仍在该分组**时显示)
- 操作: repair (修复) / clean (清理记录)

### 分组标签显示逻辑

```
IsResourceInGroup(resource_id, group_id) → boolean
```

- 实时判断，不依赖缓存
- 资源被移出分组后，标签立即消失
- 仅作辅助信息，不影响功能

---

## WebSocket

### 连接管理

- 单例 `WebSocketManager`
- 自动重连: 指数退避 (1s → 2s → 4s → ... → 30s 上限)
- 心跳: 每 30s 发送 ping，检测连接存活
- 状态: connecting / connected / disconnected

### 消息类型

| 事件 | 触发时机 | 前端响应 |
|------|----------|----------|
| `deploy:synced` | 部署/撤销操作完成 | refetch targets + 刷新当前资源列表 |
| `resource:updated` | 资源内容/属性被修改 | 刷新对应资源 |
| `resource:deleted` | 资源被删除 | 从列表移除 + 清空相关选中 |

### 注册方式

```typescript
const ws = WebSocketManager.getInstance()
ws.onMessage((msg) => {
  switch (msg.type) {
    case 'deploy:synced': ...
    case 'resource:updated': ...
    case 'resource:deleted': ...
  }
})
```

ResourcesView.vue 中注册监听，组件卸载时 `offMessage` 清理。

---

## 打开文件管理器

### API

```typescript
openFolder(path: string) // deploy.ts
```

### 行为差异

| 目标类型 | macOS 命令 | 效果 |
|----------|-----------|------|
| 文件 | `open -R {path}` | 在 Finder 中定位选中该文件 |
| 目录 | `open {path}` | 直接打开目录 |

### 触发点

- ResourceCard 菜单 "在文件管理器中打开"
- TargetItem 更多菜单 "在文件管理器中打开"

---

## 主题切换

- `uiStore.theme`: `'light'` | `'dark'`
- 切换方式: TopBar 右侧按钮
- 实现: 修改 HTML `<html>` 元素的 class (`dark`)
- 持久化: localStorage
- Tailwind `darkMode: 'class'` 配合 `dark:` 前缀样式
- Element Plus 暗色通过 `element-override.css` 覆盖
