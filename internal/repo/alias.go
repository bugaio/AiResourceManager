// Package repo alias.go 提供路径别名表的数据库操作
// 包括增删改查、名称唯一性检查
package repo

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/anthropic/airesourcemanager/internal/model"
)

// AliasRepo 路径别名数据仓库
type AliasRepo struct {
	db *DB
}

// NewAliasRepo 创建路径别名数据仓库实例
// 参数 db: 数据库连接
// 返回: AliasRepo 指针
func NewAliasRepo(db *DB) *AliasRepo {
	return &AliasRepo{db: db}
}

// InsertAlias 插入新路径别名记录
// 参数 alias: 别名实体
// 返回: 插入记录的 ID、错误信息
func (r *AliasRepo) InsertAlias(alias *model.PathAlias) (string, error) {
	r.db.Lock()
	defer r.db.Unlock()

	_, err := r.db.Conn.Exec(
		`INSERT INTO path_alias (id, alias, type, target_path, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		alias.ID, alias.Name, alias.Type, alias.Path, alias.CreatedAt, alias.CreatedAt,
	)
	if err != nil {
		return "", fmt.Errorf("插入别名失败: %w", err)
	}
	return alias.ID, nil
}

// GetAliasByID 根据 ID 查询路径别名
// 参数 id: 别名 UUID
// 返回: 别名实体指针、错误信息
func (r *AliasRepo) GetAliasByID(id string) (*model.PathAlias, error) {
	r.db.RLock()
	defer r.db.RUnlock()

	a := &model.PathAlias{}
	err := r.db.Conn.QueryRow(
		`SELECT id, alias, type, target_path, created_at FROM path_alias WHERE id = ?`, id,
	).Scan(&a.ID, &a.Name, &a.Type, &a.Path, &a.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询别名失败: %w", err)
	}
	return a, nil
}

// ListAliases 按资源类型查询路径别名列表
// 参数 aliasType: 资源类型过滤（skill/agent/config）；为空则返回全部
// 返回: 别名列表、错误信息
func (r *AliasRepo) ListAliases(aliasType string) ([]model.PathAlias, error) {
	r.db.RLock()
	defer r.db.RUnlock()

	query := `SELECT id, alias, type, target_path, created_at FROM path_alias`
	var args []interface{}
	if aliasType != "" {
		query += ` WHERE type = ?`
		args = append(args, aliasType)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := r.db.Conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询别名列表失败: %w", err)
	}
	defer rows.Close()

	var list []model.PathAlias
	for rows.Next() {
		var a model.PathAlias
		if err := rows.Scan(&a.ID, &a.Name, &a.Type, &a.Path, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("扫描别名行失败: %w", err)
		}
		list = append(list, a)
	}
	if list == nil {
		list = []model.PathAlias{}
	}
	return list, nil
}

// UpdateAlias 更新路径别名的名称和路径
// 参数 id: 别名 ID
// 参数 name: 新别名名称
// 参数 path: 新目标路径
// 返回: 错误信息
func (r *AliasRepo) UpdateAlias(id, name, path string) error {
	r.db.Lock()
	defer r.db.Unlock()

	_, err := r.db.Conn.Exec(
		`UPDATE path_alias SET alias = ?, target_path = ?, updated_at = ? WHERE id = ?`,
		name, path, time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("更新别名失败: %w", err)
	}
	return nil
}

// DeleteAlias 删除路径别名记录
// 参数 id: 别名 ID
// 返回: 错误信息
func (r *AliasRepo) DeleteAlias(id string) error {
	r.db.Lock()
	defer r.db.Unlock()

	_, err := r.db.Conn.Exec(`DELETE FROM path_alias WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("删除别名失败: %w", err)
	}
	return nil
}

// CheckAliasNameExists 检查别名名称在指定类型下是否已存在
// 参数 name: 别名名称
// 参数 aliasType: 资源类型（别名按类型隔离，仅在同类型内检查重名）
// 参数 excludeID: 排除的别名 ID（更新时排除自身，为空则不排除）
// 返回: 是否存在
func (r *AliasRepo) CheckAliasNameExists(name, aliasType, excludeID string) bool {
	r.db.RLock()
	defer r.db.RUnlock()

	var count int
	query := `SELECT COUNT(*) FROM path_alias WHERE alias = ? AND type = ?`
	args := []interface{}{name, aliasType}
	if excludeID != "" {
		query += ` AND id != ?`
		args = append(args, excludeID)
	}
	_ = r.db.Conn.QueryRow(query, args...).Scan(&count)
	return count > 0
}
