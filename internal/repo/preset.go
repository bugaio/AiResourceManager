// Package repo preset.go 提供 preset 表 + preset_resource 关联表的数据库操作
package repo

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/anthropic/airesourcemanager/internal/model"
)

// PresetRepo Preset 数据仓库
type PresetRepo struct {
	db *DB
}

// NewPresetRepo 创建 PresetRepo
func NewPresetRepo(db *DB) *PresetRepo {
	return &PresetRepo{db: db}
}

// InsertPreset 插入 preset
func (r *PresetRepo) InsertPreset(p *model.Preset) error {
	r.db.Lock()
	defer r.db.Unlock()
	_, err := r.db.Conn.Exec(
		`INSERT INTO preset (id, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Description, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("插入 preset 失败: %w", err)
	}
	return nil
}

// GetPresetByID 根据 ID 查询 preset
func (r *PresetRepo) GetPresetByID(id string) (*model.Preset, error) {
	r.db.RLock()
	defer r.db.RUnlock()
	p := &model.Preset{}
	err := r.db.Conn.QueryRow(
		`SELECT id, name, description, created_at, updated_at FROM preset WHERE id = ?`, id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询 preset 失败: %w", err)
	}
	return p, nil
}

// ListPresets 列出全部 preset（按 created_at 倒序）
func (r *PresetRepo) ListPresets() ([]model.Preset, error) {
	r.db.RLock()
	defer r.db.RUnlock()
	rows, err := r.db.Conn.Query(
		`SELECT id, name, description, created_at, updated_at FROM preset ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("查询 preset 列表失败: %w", err)
	}
	defer rows.Close()
	var list []model.Preset
	for rows.Next() {
		var p model.Preset
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("扫描 preset 行失败: %w", err)
		}
		list = append(list, p)
	}
	if list == nil {
		list = []model.Preset{}
	}
	return list, nil
}

// CheckPresetNameExists 检查 preset 名称是否已存在，excludeID 为空表示不排除
func (r *PresetRepo) CheckPresetNameExists(name, excludeID string) (bool, error) {
	r.db.RLock()
	defer r.db.RUnlock()
	var count int
	query := `SELECT COUNT(*) FROM preset WHERE name = ?`
	args := []interface{}{name}
	if excludeID != "" {
		query += ` AND id != ?`
		args = append(args, excludeID)
	}
	if err := r.db.Conn.QueryRow(query, args...).Scan(&count); err != nil {
		return false, fmt.Errorf("检查 preset 名称重复失败: %w", err)
	}
	return count > 0, nil
}

// UpdatePreset 更新 preset 字段（name/description）
func (r *PresetRepo) UpdatePreset(id string, name *string, description *string) error {
	r.db.Lock()
	defer r.db.Unlock()
	var sets []string
	var args []interface{}
	if name != nil {
		sets = append(sets, "name = ?")
		args = append(args, *name)
	}
	if description != nil {
		sets = append(sets, "description = ?")
		args = append(args, *description)
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, time.Now())
	args = append(args, id)
	query := "UPDATE preset SET " + strings.Join(sets, ", ") + " WHERE id = ?"
	if _, err := r.db.Conn.Exec(query, args...); err != nil {
		return fmt.Errorf("更新 preset 失败: %w", err)
	}
	return nil
}

// DeletePreset 删除 preset 本体（preset_resource 由外键 CASCADE 自动清理）
func (r *PresetRepo) DeletePreset(id string) error {
	r.db.Lock()
	defer r.db.Unlock()
	if _, err := r.db.Conn.Exec(`DELETE FROM preset WHERE id = ?`, id); err != nil {
		return fmt.Errorf("删除 preset 失败: %w", err)
	}
	return nil
}

// LinkResources 关联资源到 preset (INSERT OR IGNORE)
func (r *PresetRepo) LinkResources(presetID string, resourceIDs []string) error {
	if len(resourceIDs) == 0 {
		return nil
	}
	r.db.Lock()
	defer r.db.Unlock()
	tx, err := r.db.Conn.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	now := time.Now()
	for _, rid := range resourceIDs {
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO preset_resource (preset_id, resource_id, created_at) VALUES (?, ?, ?)`,
			presetID, rid, now,
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("关联资源失败: %w", err)
		}
	}
	return tx.Commit()
}

// UnlinkResources 解除资源与 preset 的关联
func (r *PresetRepo) UnlinkResources(presetID string, resourceIDs []string) error {
	if len(resourceIDs) == 0 {
		return nil
	}
	r.db.Lock()
	defer r.db.Unlock()
	tx, err := r.db.Conn.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	for _, rid := range resourceIDs {
		if _, err := tx.Exec(
			`DELETE FROM preset_resource WHERE preset_id = ? AND resource_id = ?`,
			presetID, rid,
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("解除关联失败: %w", err)
		}
	}
	return tx.Commit()
}

// ListPresetResources 返回 preset 关联的资源 ID 列表（不含私有资源）
func (r *PresetRepo) ListPresetResources(presetID string) ([]string, error) {
	r.db.RLock()
	defer r.db.RUnlock()
	rows, err := r.db.Conn.Query(
		`SELECT resource_id FROM preset_resource WHERE preset_id = ?`, presetID,
	)
	if err != nil {
		return nil, fmt.Errorf("查询关联资源失败: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var rid string
		if err := rows.Scan(&rid); err != nil {
			return nil, fmt.Errorf("扫描关联资源失败: %w", err)
		}
		ids = append(ids, rid)
	}
	return ids, nil
}

// ListPresetsByResourceID 查询某资源被关联到的所有 preset (id, name)
func (r *PresetRepo) ListPresetsByResourceID(resourceID string) ([]model.PresetLinkInfo, error) {
	r.db.RLock()
	defer r.db.RUnlock()
	rows, err := r.db.Conn.Query(
		`SELECT p.id, p.name FROM preset p
		 INNER JOIN preset_resource pr ON pr.preset_id = p.id
		 WHERE pr.resource_id = ?`, resourceID,
	)
	if err != nil {
		return nil, fmt.Errorf("查询资源所属 preset 失败: %w", err)
	}
	defer rows.Close()
	var list []model.PresetLinkInfo
	for rows.Next() {
		var item model.PresetLinkInfo
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			return nil, fmt.Errorf("扫描 preset 行失败: %w", err)
		}
		list = append(list, item)
	}
	return list, nil
}

// ListPresetsByResourceIDs 批量查询多个资源各自被关联到的 preset (id, name)
// 返回: map[resourceID] -> []PresetLinkInfo，避免逐条查询的 N+1 问题
func (r *PresetRepo) ListPresetsByResourceIDs(resourceIDs []string) (map[string][]model.PresetLinkInfo, error) {
	result := make(map[string][]model.PresetLinkInfo)
	if len(resourceIDs) == 0 {
		return result, nil
	}
	r.db.RLock()
	defer r.db.RUnlock()
	placeholders := make([]string, len(resourceIDs))
	args := make([]interface{}, len(resourceIDs))
	for i, id := range resourceIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	query := `SELECT pr.resource_id, p.id, p.name FROM preset p
		 INNER JOIN preset_resource pr ON pr.preset_id = p.id
		 WHERE pr.resource_id IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := r.db.Conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("批量查询资源所属 preset 失败: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var resourceID string
		var item model.PresetLinkInfo
		if err := rows.Scan(&resourceID, &item.ID, &item.Name); err != nil {
			return nil, fmt.Errorf("扫描 preset 行失败: %w", err)
		}
		result[resourceID] = append(result[resourceID], item)
	}
	return result, nil
}

// CountPresetByResource 返回某资源被关联到的 preset 数量
func (r *PresetRepo) CountPresetByResource(resourceID string) (int, error) {
	r.db.RLock()
	defer r.db.RUnlock()
	var count int
	if err := r.db.Conn.QueryRow(
		`SELECT COUNT(*) FROM preset_resource WHERE resource_id = ?`, resourceID,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("查询资源 preset 关联数量失败: %w", err)
	}
	return count, nil
}

// ListPrivateResourceIDs 查询某 preset 下所有私有资源 ID（owner_preset_id = presetID）
func (r *PresetRepo) ListPrivateResourceIDs(presetID string) ([]string, error) {
	r.db.RLock()
	defer r.db.RUnlock()
	rows, err := r.db.Conn.Query(
		`SELECT id FROM resource WHERE owner_preset_id = ?`, presetID,
	)
	if err != nil {
		return nil, fmt.Errorf("查询私有资源失败: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var rid string
		if err := rows.Scan(&rid); err != nil {
			return nil, fmt.Errorf("扫描私有资源失败: %w", err)
		}
		ids = append(ids, rid)
	}
	return ids, nil
}

// OwnerPresetIDOf 返回某资源的 owner_preset_id（无则空串）
func (r *PresetRepo) OwnerPresetIDOf(resourceID string) (string, error) {
	r.db.RLock()
	defer r.db.RUnlock()
	var pid sql.NullString
	err := r.db.Conn.QueryRow(
		`SELECT owner_preset_id FROM resource WHERE id = ?`, resourceID,
	).Scan(&pid)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("查询 owner_preset_id 失败: %w", err)
	}
	if pid.Valid {
		return pid.String, nil
	}
	return "", nil
}

// GetPathGroupByIDByID 查询路径组（preset 部署时使用,避免额外注入 PathGroupRepo）
// 注意: 未找到时返回 (nil, nil)
func (r *PresetRepo) GetPathGroupByIDByID(id string) (*model.PathGroup, error) {
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
