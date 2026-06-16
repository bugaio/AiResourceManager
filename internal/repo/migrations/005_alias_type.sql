-- 005: 路径别名按资源类型隔离
-- path_alias 增加 type 列，唯一约束从全局 alias 改为 (type, alias) 复合唯一
-- SQLite 无法直接删除列级 UNIQUE 约束，需重建表
-- 存量别名无类型归属，统一默认归为 skill

CREATE TABLE path_alias_new (
    id TEXT PRIMARY KEY,
    alias TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'skill',
    target_path TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(type, alias)
);

INSERT INTO path_alias_new (id, alias, type, target_path, description, created_at, updated_at)
SELECT id, alias, 'skill', target_path, description, created_at, updated_at FROM path_alias;

DROP TABLE path_alias;

ALTER TABLE path_alias_new RENAME TO path_alias;
