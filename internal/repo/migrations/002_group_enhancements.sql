-- 002_group_enhancements.sql 增强分组表结构
-- 添加 type 和 sort_order 字段，为 deployment 添加 group_id 关联

ALTER TABLE "group" ADD COLUMN type TEXT NOT NULL DEFAULT '';
ALTER TABLE "group" ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0;

ALTER TABLE deployment ADD COLUMN group_id TEXT NOT NULL DEFAULT '';
