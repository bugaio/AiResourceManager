# 任务09: 数据导入导出与文件监听

## 目标

实现数据导入导出功能（前后端完整闭环）和 fsnotify 文件监听服务（检测中央仓库文件变化，通过 WebSocket 推送事件到前端）。

## 交付内容

### 后端 — 导入导出
- `internal/handler/data.go`（导入导出 handler）
- `internal/service/data.go`（导入导出业务逻辑）

### 后端 — 文件监听
- `internal/service/watcher.go`（fsnotify 监听 + 事件处理 + WebSocket 推送）

### 前端
- `src/views/DataView.vue`（完整的导入导出页面）
- `src/api/data.ts`（导入导出 API）

### 导出逻辑
1. 接收目标目录路径
2. 将 `{HOME}/.aiManager/` 下所有内容复制到目标目录：
   - `data/aimanager.db`
   - `skills/` 目录（含所有子目录和文件）
   - `agents/` 目录
   - `mcps/` 目录
   - `config.yaml`
3. 返回导出结果（文件数量、总大小）

### 导入逻辑
1. 接收源目录路径
2. 校验源目录结构合法性
3. 冲突策略（前端选择）：
   - 覆盖已有：UUID 相同的直接覆盖
   - 跳过重复：UUID 已存在的跳过
   - 两者都保留：生成新 UUID 导入
4. 合并 SQLite 数据（资源、分组、关联、别名）
5. 复制文件到中央仓库
6. 返回导入结果（新增/覆盖/跳过数量）

### 文件监听 + WebSocket 推送
- 启动时注册 watcher 监听 `{HOME}/.aiManager/skills/`、`agents/`、`mcps/`
- 事件处理：
  - MODIFY → 更新 resource.updated_at → WebSocket 广播 `resource:updated`
  - DELETE → 日志告警 → WebSocket 广播 `resource:deleted`
  - CREATE → 日志记录（外部添加，不自动导入）
- 去重策略：短时间窗口去重
  - 维护 `map[string]time.Time` 记录每个文件路径的最后处理时间
  - 收到事件时检查：距上次处理 < 500ms → 跳过
  - 这同时解决了自身 API 操作触发的重复事件问题（API写文件后500ms内的fsnotify事件被忽略）

### 前端页面
- 导出区域：目标目录输入 + 导出按钮 + 结果展示
- 导入区域：源目录输入 + 冲突策略单选 + 导入按钮 + 结果展示
- 操作中显示 loading 状态
- 成功/失败 toast 通知

### 前端 WebSocket 事件响应
- `resource:updated` → 若资源在当前列表中，刷新该资源卡片数据
- `resource:deleted` → 从当前列表移除，toast 提示"资源 xxx 已被外部删除"

## 验收标准

1. 导出到指定目录后，目录中包含完整的仓库副本
2. 从导出目录导入到空仓库，所有资源/分组/别名恢复
3. 三种冲突策略各自行为正确
4. 文件监听启动后，外部修改文件 → 数据库 updated_at 更新
5. 外部修改文件后，前端自动刷新卡片（通过 WebSocket）
6. 外部删除文件后，前端自动移除卡片 + toast 提示
7. 自身 API 操作不触发重复的 WebSocket 推送
8. 前端页面交互完整，导入导出结果正确展示
