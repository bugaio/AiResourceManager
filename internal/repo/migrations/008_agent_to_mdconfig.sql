-- 008_agent_to_mdconfig.sql 将 agent 类型重命名为 mdconfig
-- 1. resource 表 type 字段数据迁移
UPDATE resource SET type = 'mdconfig' WHERE type = 'agent';

-- 2. path_group 表新增 mdconfig_path 列并从 agent_path 复制数据
--    （SQLite 旧版不支持 DROP COLUMN，保留 agent_path 列但代码不再读写）
ALTER TABLE path_group ADD COLUMN mdconfig_path TEXT NOT NULL DEFAULT '';
UPDATE path_group SET mdconfig_path = agent_path;
