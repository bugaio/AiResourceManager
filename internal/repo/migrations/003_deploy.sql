-- 003_deploy.sql 重建部署表结构以适配 Task 07 部署机制
-- 清除旧的 deployment/deployment_item 表并重建

DROP TABLE IF EXISTS deployment_item;
DROP TABLE IF EXISTS deployment;

-- 部署记录表
CREATE TABLE deployment (
    id TEXT PRIMARY KEY,
    group_id TEXT,
    resource_id TEXT,
    target_path TEXT NOT NULL,
    alias_id TEXT,
    deploy_type TEXT NOT NULL DEFAULT 'symlink',
    track INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 部署明细表
CREATE TABLE deployment_item (
    id TEXT PRIMARY KEY,
    deployment_id TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    link_path TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (deployment_id) REFERENCES deployment(id) ON DELETE CASCADE
);
