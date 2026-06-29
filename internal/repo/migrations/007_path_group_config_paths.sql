-- 007: 路径组支持多个 config 目标路径
-- 一个 preset 内的多个 config 资源可分别部署到不同文件,故路径组需持有多条 config 路径。
-- 新增 config_paths(JSON 数组)作为真相源;保留 config_path 单值列作为"第一条"镜像,
-- 兼容仍按单值读取的旧代码(侧栏预览 / 部署弹窗显示等),写入时由后端同步。

ALTER TABLE path_group ADD COLUMN config_paths TEXT NOT NULL DEFAULT '[]';

-- 把存量的非空 config_path 迁入数组(单元素);空字符串保持空数组
UPDATE path_group
SET config_paths = '["' || replace(config_path, '"', '\"') || '"]'
WHERE config_path != '';
