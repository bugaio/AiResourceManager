# 数据库

## 迁移机制

- SQL 文件嵌入: `//go:embed migrations/*.sql`
- 记录表: `_migrations`（name + applied_at），避免重复执行
- 执行顺序: 按文件名字典序
- 事务: 每个迁移文件包在独立事务中
- 触发时机: `serve` 命令启动时自动执行 `RunMigrations(db)`

## 迁移文件列表

| 文件                         | 说明                                              |
|------------------------------|---------------------------------------------------|
| 001_init.sql                 | 初始建表（resource, group, group_resource, path_alias, deployment, deployment_item） |
| 002_group_enhancements.sql   | group 加 type + sort_order；deployment 加 group_id |
| 003_deploy.sql               | 重建 deployment + deployment_item（适配新部署机制）|
| 004_group_color.sql          | group 加 color 字段                               |
| 005_alias_type.sql           | path_alias 重建，加 type 列 + UNIQUE(type, alias) |

## 最终表结构

### resource

```sql
CREATE TABLE resource (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT '',          -- skill/agent/mcp
    path TEXT NOT NULL DEFAULT '',          -- 文件/目录绝对路径
    description TEXT NOT NULL DEFAULT '',
    metadata TEXT NOT NULL DEFAULT '{}',    -- JSON 扩展字段（当前未用）
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### group

```sql
CREATE TABLE "group" (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    type TEXT NOT NULL DEFAULT '',          -- skill/agent/mcp
    sort_order INTEGER NOT NULL DEFAULT 0,
    color TEXT DEFAULT '',                  -- 分组颜色（#hex）
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### group_resource

```sql
CREATE TABLE group_resource (
    group_id TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (group_id, resource_id),
    FOREIGN KEY (group_id) REFERENCES "group"(id) ON DELETE CASCADE,
    FOREIGN KEY (resource_id) REFERENCES resource(id) ON DELETE CASCADE
);
```

### path_alias

```sql
CREATE TABLE path_alias (
    id TEXT PRIMARY KEY,
    alias TEXT NOT NULL,                    -- 别名名称
    type TEXT NOT NULL DEFAULT 'skill',     -- skill/agent/mcp
    target_path TEXT NOT NULL,              -- 展开后的绝对路径
    description TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(type, alias)                     -- 同类型内名称唯一
);
```

### deployment

```sql
CREATE TABLE deployment (
    id TEXT PRIMARY KEY,
    group_id TEXT,                          -- 整组部署时非空
    resource_id TEXT,                       -- 单资源部署时非空
    target_path TEXT NOT NULL,              -- 部署目标路径
    alias_id TEXT,                          -- 关联的别名 ID
    deploy_type TEXT NOT NULL DEFAULT 'symlink', -- symlink/merge
    track INTEGER NOT NULL DEFAULT 0,      -- 0=静态, 1=追踪
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### deployment_item

```sql
CREATE TABLE deployment_item (
    id TEXT PRIMARY KEY,
    deployment_id TEXT NOT NULL,
    resource_id TEXT NOT NULL,              -- 具体部署的资源
    link_path TEXT NOT NULL,               -- symlink: 链接目标路径; merge: JSON key 名
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (deployment_id) REFERENCES deployment(id) ON DELETE CASCADE
);
```

### _migrations

```sql
CREATE TABLE _migrations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,             -- 迁移文件名
    applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

## 连接配置

```go
sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
```

- WAL 模式: 提升并发读性能
- busy_timeout=5000ms: 写锁等待超时
- 外键约束: `PRAGMA foreign_keys=ON`

## 并发安全

`repo.DB` 封装 `sync.RWMutex`:
- 读操作: `db.RLock()` / `db.RUnlock()`
- 写操作: `db.Lock()` / `db.Unlock()`
- 迁移: 持写锁执行全部迁移
