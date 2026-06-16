-- 001_init.sql 初始化数据库表结构
-- 创建 AiResourceManager 所需的全部基础表

-- 资源表：存储单个 AI 资源的基本信息
CREATE TABLE IF NOT EXISTS resource (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT '',
    path TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    metadata TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 分组表：资源的逻辑分组
CREATE TABLE IF NOT EXISTS "group" (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 分组-资源关联表：多对多关系
CREATE TABLE IF NOT EXISTS group_resource (
    group_id TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (group_id, resource_id),
    FOREIGN KEY (group_id) REFERENCES "group"(id) ON DELETE CASCADE,
    FOREIGN KEY (resource_id) REFERENCES resource(id) ON DELETE CASCADE
);

-- 路径别名表：为资源路径提供友好的别名
CREATE TABLE IF NOT EXISTS path_alias (
    id TEXT PRIMARY KEY,
    alias TEXT NOT NULL UNIQUE,
    target_path TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 部署表：记录部署任务
CREATE TABLE IF NOT EXISTS deployment (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    track INTEGER NOT NULL DEFAULT 0,
    source_path TEXT NOT NULL DEFAULT '',
    target_path TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 部署项表：部署任务中的具体文件操作
CREATE TABLE IF NOT EXISTS deployment_item (
    id TEXT PRIMARY KEY,
    deployment_id TEXT NOT NULL,
    source TEXT NOT NULL,
    target TEXT NOT NULL,
    action TEXT NOT NULL DEFAULT 'copy',
    status TEXT NOT NULL DEFAULT 'pending',
    error_msg TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (deployment_id) REFERENCES deployment(id) ON DELETE CASCADE
);
