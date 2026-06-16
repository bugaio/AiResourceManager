# 总结

## 核心架构: 一套组件, 三种类型

三种资源类型 (Skill / MCP / SubAgent) 共用:
- 同一张数据库表 (`resources`, `deployments`, `deployment_items`, `groups`, `path_alias`)
- 同一套 REST API (`/api/v1/resources`, `/api/v1/deploy` 等)
- 同一套前端组件 (ResourceCard, DeployDialog, TargetList 等)

通过 `type` 字段 switch 区分行为差异。

---

## 类型差异一览

| 维度 | Skill | MCP | SubAgent |
|------|-------|-----|----------|
| type 值 | `skill` | `mcp` | `agent` |
| 前端显示名 | Skill | MCP | SubAgent |
| 存储结构 | `skills/{uuid}/` 目录 | `mcps/{uuid}.jsonc` 文件 | `agents/{uuid}.md` 文件 |
| deploy_type | symlink | merge | symlink |
| symlink 产物 | 目录 symlink `{target}/{name}` | 无 | 文件 symlink `{target}/{name}.md` |
| 合并产物 | 无 | deep merge 到 .json 文件 | 无 |
| 编辑器语言 | markdown | json (JSONC) | markdown |
| 格式化按钮 | 无 | 有 | 无 |
| 保存后同步 | 无 | SyncDeployDialog | 无 |
| 冲突处理 | confirm 覆盖 | 预检 + Union-Find + McpConflictDialog | confirm 覆盖 |
| 撤销方式 | 删 symlink | 从 JSON 移除 key | 删 symlink |
| 健康检查 | symlink 有效性 | key 是否存在于目标 JSON | symlink 有效性 |
| 路径约束 | 无 | 必须 .json 结尾 | 无 |
| 卡片徽标色 | 蓝色 | 绿色 | 紫色 |
| 别名隔离 | type='skill' | type='mcp' (路径须 .json) | type='agent' |

---

## 关键设计决策

### 为什么用 type 字段而不是三套独立实体?

- 三种资源的元数据结构完全相同 (name, description, uuid, path)
- CRUD 操作一致，只有部署行为不同
- 一套组件 + switch 比三套组件维护成本低一个量级
- 新增类型只需加一个 type 枚举值 + 少量 switch case

### 为什么 MCP 用 deep merge 而不是 symlink?

- MCP 配置的目标文件 (如 `claude_desktop_config.json`) 是多个 server 的聚合
- 每个 MCP 资源只是该文件中的一个或几个 key
- 不能整文件覆盖，必须合并进去
- 撤销时也必须精确移除自己的 key，不影响其他 server

### 为什么 MCP 需要冲突预检?

- 多个 MCP 资源可能声明同名 key (如两个资源都定义 `my-server`)
- 直接合并会静默覆盖，用户可能不知情
- Union-Find 分组让用户看到哪些资源互相冲突
- 预检不写文件，用户可以取消或选择性部署

### 为什么 TargetItem 分组标签要实时判断?

- 资源可能随时被移出分组
- 若缓存旧的分组信息，展开 TargetItem 看到过期标签会误导用户
- `IsResourceInGroup` 每次展开时实时查询，保证准确性

### 为什么别名按 type 隔离?

- 不同类型的目标路径性质不同 (MCP 要 .json 文件，Skill/Agent 要目录)
- 隔离后同名别名互不干扰，简化用户心智模型
- UI 上切 Tab 后别名自动切换，无需手动过滤

### 为什么 WebSocket 用单例 + 自动重连?

- 部署/撤销是耗时操作，完成后需通知前端刷新
- 单例避免多组件重复建连
- 自动重连保证长时间使用不断线 (指数退避避免打满服务端)

---

## 已知约束和注意事项

### 前端

1. **Monaco Editor 体积:** vite-plugin-monaco-editor 单独分包，但首次加载仍较大。仅加载了 json worker，markdown 无语法高亮 worker。
2. **Tailwind preflight: false:** 不重置浏览器默认样式，依赖 Element Plus 的 normalize。自定义样式需注意浏览器默认 margin/padding。
3. **暗色主题:** 通过 class 切换 + element-override.css 覆盖。Element Plus 官方暗色方案是 CSS Variables，当前实现为手动覆盖，后续可考虑迁移。
4. **分页:** 前端分页组件 + 后端分页查询，非前端全量缓存。切分组/搜索时重置到第一页。
5. **搜索防抖:** 300ms，不支持高级搜索语法。
6. **别名数量:** 无上限限制，大量别名时 PathInput 下拉可能需要虚拟滚动。

### 数据一致性

1. **symlink 可被外部删除:** 用户手动删除 symlink 后，deployment_items 记录仍在 → 健康检查报 broken → repair 或 clean。
2. **MCP 目标 JSON 可被外部编辑:** 其他工具修改目标 JSON 后，key 可能已变更 → 健康检查报 broken。
3. **WebSocket 消息丢失:** 网络中断期间的操作不会补发 → 重连后前端主动 refetch 一次。

### 后端依赖

1. 后端端口: 3678，前端 Vite proxy 硬编码
2. API 响应格式: `{code: number, msg: string, data: T}`，code=0 表示成功
3. WebSocket 端点: `/ws`
4. 文件系统操作: symlink 创建/删除、JSON 读写、目录管理均在后端执行
