// Package repo path_group.go 提供 path_group 表的数据库操作
package repo

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/anthropic/airesourcemanager/internal/model"
)

// PathGroupRepo 路径组数据仓库
type PathGroupRepo struct {
	db *DB
}

// NewPathGroupRepo 创建 PathGroupRepo
func NewPathGroupRepo(db *DB) *PathGroupRepo {
	return &PathGroupRepo{db: db}
}

// marshalConfigPaths 把 ConfigPaths 序列化为 JSON 字符串（空切片 → "[]"）
func marshalConfigPaths(paths []string) string {
	if len(paths) == 0 {
		return "[]"
	}
	b, err := json.Marshal(paths)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// hydrateConfigPaths 反序列化 config_paths 字段并回填 ConfigPath 镜像（= 第一条）
// 兼容历史数据：若 config_paths 为空但旧 config_path 非空，用旧值组成单元素数组
func hydrateConfigPaths(g *model.PathGroup, raw string) {
	var paths []string
	if raw != "" && raw != "[]" {
		_ = json.Unmarshal([]byte(raw), &paths)
	}
	if len(paths) == 0 && g.ConfigPath != "" {
		paths = []string{g.ConfigPath}
	}
	if paths == nil {
		paths = []string{}
	}
	g.ConfigPaths = paths
	if len(paths) > 0 {
		g.ConfigPath = paths[0]
	} else {
		g.ConfigPath = ""
	}
}

// InsertPathGroup 插入路径组
func (r *PathGroupRepo) InsertPathGroup(g *model.PathGroup) error {
	r.db.Lock()
	defer r.db.Unlock()
	// ConfigPath 镜像 = ConfigPaths[0]
	if len(g.ConfigPaths) > 0 {
		g.ConfigPath = g.ConfigPaths[0]
	} else {
		g.ConfigPath = ""
	}
	_, err := r.db.Conn.Exec(
		`INSERT INTO path_group (id, name, skill_path, agent_path, config_path, config_paths, prompt_path, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		g.ID, g.Name, g.SkillPath, g.AgentPath, g.ConfigPath, marshalConfigPaths(g.ConfigPaths), g.PromptPath, g.CreatedAt, g.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("插入路径组失败: %w", err)
	}
	return nil
}

// GetPathGroupByID 根据 ID 查询路径组
func (r *PathGroupRepo) GetPathGroupByID(id string) (*model.PathGroup, error) {
	r.db.RLock()
	defer r.db.RUnlock()
	g := &model.PathGroup{}
	var configPaths string
	err := r.db.Conn.QueryRow(
		`SELECT id, name, skill_path, agent_path, config_path, config_paths, prompt_path, created_at, updated_at
		 FROM path_group WHERE id = ?`, id,
	).Scan(&g.ID, &g.Name, &g.SkillPath, &g.AgentPath, &g.ConfigPath, &configPaths, &g.PromptPath, &g.CreatedAt, &g.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询路径组失败: %w", err)
	}
	hydrateConfigPaths(g, configPaths)
	return g, nil
}

// ListPathGroups 列出全部路径组（按 created_at 倒序）
func (r *PathGroupRepo) ListPathGroups() ([]model.PathGroup, error) {
	r.db.RLock()
	defer r.db.RUnlock()
	rows, err := r.db.Conn.Query(
		`SELECT id, name, skill_path, agent_path, config_path, config_paths, prompt_path, created_at, updated_at
		 FROM path_group ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("查询路径组列表失败: %w", err)
	}
	defer rows.Close()
	var list []model.PathGroup
	for rows.Next() {
		var g model.PathGroup
		var configPaths string
		if err := rows.Scan(&g.ID, &g.Name, &g.SkillPath, &g.AgentPath, &g.ConfigPath, &configPaths, &g.PromptPath, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, fmt.Errorf("扫描路径组行失败: %w", err)
		}
		hydrateConfigPaths(&g, configPaths)
		list = append(list, g)
	}
	if list == nil {
		list = []model.PathGroup{}
	}
	return list, nil
}

// CheckPathGroupNameExists 名称唯一性
func (r *PathGroupRepo) CheckPathGroupNameExists(name, excludeID string) (bool, error) {
	r.db.RLock()
	defer r.db.RUnlock()
	var count int
	query := `SELECT COUNT(*) FROM path_group WHERE name = ?`
	args := []interface{}{name}
	if excludeID != "" {
		query += ` AND id != ?`
		args = append(args, excludeID)
	}
	if err := r.db.Conn.QueryRow(query, args...).Scan(&count); err != nil {
		return false, fmt.Errorf("检查路径组名称重复失败: %w", err)
	}
	return count > 0, nil
}

// UpdatePathGroup 部分字段更新
func (r *PathGroupRepo) UpdatePathGroup(id string, req *model.UpdatePathGroupReq) error {
	r.db.Lock()
	defer r.db.Unlock()
	var sets []string
	var args []interface{}
	if req.Name != nil {
		sets = append(sets, "name = ?")
		args = append(args, *req.Name)
	}
	if req.SkillPath != nil {
		sets = append(sets, "skill_path = ?")
		args = append(args, *req.SkillPath)
	}
	if req.AgentPath != nil {
		sets = append(sets, "agent_path = ?")
		args = append(args, *req.AgentPath)
	}
	if req.ConfigPaths != nil {
		// 多条 config 路径整体替换；同步 config_path 镜像 = 第一条
		paths := *req.ConfigPaths
		sets = append(sets, "config_paths = ?")
		args = append(args, marshalConfigPaths(paths))
		mirror := ""
		if len(paths) > 0 {
			mirror = paths[0]
		}
		sets = append(sets, "config_path = ?")
		args = append(args, mirror)
	} else if req.ConfigPath != nil {
		// 仅传单值（旧客户端）：同时更新镜像与数组
		sets = append(sets, "config_path = ?")
		args = append(args, *req.ConfigPath)
		paths := []string{}
		if *req.ConfigPath != "" {
			paths = []string{*req.ConfigPath}
		}
		sets = append(sets, "config_paths = ?")
		args = append(args, marshalConfigPaths(paths))
	}
	if req.PromptPath != nil {
		sets = append(sets, "prompt_path = ?")
		args = append(args, *req.PromptPath)
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, time.Now())
	args = append(args, id)
	query := "UPDATE path_group SET " + strings.Join(sets, ", ") + " WHERE id = ?"
	if _, err := r.db.Conn.Exec(query, args...); err != nil {
		return fmt.Errorf("更新路径组失败: %w", err)
	}
	return nil
}

// DeletePathGroup 删除路径组
func (r *PathGroupRepo) DeletePathGroup(id string) error {
	r.db.Lock()
	defer r.db.Unlock()
	if _, err := r.db.Conn.Exec(`DELETE FROM path_group WHERE id = ?`, id); err != nil {
		return fmt.Errorf("删除路径组失败: %w", err)
	}
	return nil
}
