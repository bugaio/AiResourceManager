-- 009_path_group_mdconfig_paths.sql mdconfig 支持多路径
ALTER TABLE path_group ADD COLUMN mdconfig_paths TEXT NOT NULL DEFAULT '[]';
UPDATE path_group SET mdconfig_paths = json_array(mdconfig_path) WHERE mdconfig_path != '';
