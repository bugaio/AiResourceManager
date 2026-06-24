-- 006_preset.sql Preset 模块: preset / preset_resource / path_group 表
-- 同时给 resource 和 deployment 表追加可空外键列

-- 1. preset 本体
CREATE TABLE IF NOT EXISTS preset (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 2. preset 与已有资源的关联（私有资源不进此表）
CREATE TABLE IF NOT EXISTS preset_resource (
    preset_id   TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (preset_id, resource_id),
    FOREIGN KEY (preset_id)   REFERENCES preset(id)   ON DELETE CASCADE,
    FOREIGN KEY (resource_id) REFERENCES resource(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_preset_resource_resource ON preset_resource(resource_id);

-- 3. 路径组
CREATE TABLE IF NOT EXISTS path_group (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL UNIQUE,
    skill_path   TEXT NOT NULL DEFAULT '',
    agent_path   TEXT NOT NULL DEFAULT '',
    config_path  TEXT NOT NULL DEFAULT '',
    prompt_path  TEXT NOT NULL DEFAULT '',
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 4. resource 新增 owner_preset_id 列
ALTER TABLE resource ADD COLUMN owner_preset_id TEXT DEFAULT NULL;
CREATE INDEX IF NOT EXISTS idx_resource_owner_preset ON resource(owner_preset_id);

-- 5. deployment 新增 preset_id 列
ALTER TABLE deployment ADD COLUMN preset_id TEXT DEFAULT NULL;
CREATE INDEX IF NOT EXISTS idx_deployment_preset ON deployment(preset_id);
