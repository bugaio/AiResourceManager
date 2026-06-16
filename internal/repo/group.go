// Package repo group.go 提供分组表的数据库操作
// 包括分组增删改查、资源关联、部署关联查询等
package repo

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/anthropic/airesourcemanager/internal/model"
)

// GroupRepo 分组数据仓库
type GroupRepo struct {
	db *DB
}

// NewGroupRepo 创建分组数据仓库实例
// 参数 db: 数据库连接
// 返回: GroupRepo 指针
func NewGroupRepo(db *DB) *GroupRepo {
	return &GroupRepo{db: db}
}

// InsertGroup 插入新分组记录
// 参数 g: 分组实体
// 返回: 错误信息
func (repo *GroupRepo) InsertGroup(g *model.Group) error {
	repo.db.Lock()
	defer repo.db.Unlock()

	_, err := repo.db.Conn.Exec(
		`INSERT INTO "group" (id, name, type, color, sort_order, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		g.ID, g.Name, g.Type, g.Color, g.SortOrder, g.CreatedAt, g.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("插入分组失败: %w", err)
	}
	return nil
}

// GetGroupByID 根据 ID 查询分组
// 参数 id: 分组 UUID
// 返回: 分组实体指针和错误信息
func (repo *GroupRepo) GetGroupByID(id string) (*model.Group, error) {
	repo.db.RLock()
	defer repo.db.RUnlock()

	g := &model.Group{}
	err := repo.db.Conn.QueryRow(
		`SELECT id, name, type, color, sort_order, created_at, updated_at
		 FROM "group" WHERE id = ?`, id,
	).Scan(&g.ID, &g.Name, &g.Type, &g.Color, &g.SortOrder, &g.CreatedAt, &g.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询分组失败: %w", err)
	}
	return g, nil
}

// ListGroups 分页查询分组列表
// 参数 groupType: 分组类型筛选（空字符串不筛选）
// 参数 page: 页码（从 1 开始）
// 参数 pageSize: 每页数量
// 返回: 分组列表、总数、错误信息
func (repo *GroupRepo) ListGroups(groupType string, page, pageSize int) ([]model.Group, int, error) {
	repo.db.RLock()
	defer repo.db.RUnlock()

	var conditions []string
	var args []interface{}

	if groupType != "" {
		conditions = append(conditions, "type = ?")
		args = append(args, groupType)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	// 查询总数
	var total int
	countQuery := `SELECT COUNT(*) FROM "group"` + whereClause
	if err := repo.db.Conn.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("查询分组总数失败: %w", err)
	}

	// 分页查询
	offset := (page - 1) * pageSize
	listQuery := `SELECT id, name, type, color, sort_order, created_at, updated_at FROM "group"` +
		whereClause + " ORDER BY sort_order ASC, created_at DESC LIMIT ? OFFSET ?"
	listArgs := append(args, pageSize, offset)

	rows, err := repo.db.Conn.Query(listQuery, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询分组列表失败: %w", err)
	}
	defer rows.Close()

	var list []model.Group
	for rows.Next() {
		var g model.Group
		if err := rows.Scan(&g.ID, &g.Name, &g.Type, &g.Color, &g.SortOrder, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("扫描分组行失败: %w", err)
		}
		list = append(list, g)
	}
	if list == nil {
		list = []model.Group{}
	}
	return list, total, nil
}

// UpdateGroup 更新分组名称和排序
// 参数 id: 分组 ID
// 参数 name: 新名称（nil 表示不更新）
// 参数 sortOrder: 新排序权重（nil 表示不更新）
// 返回: 错误信息
func (repo *GroupRepo) UpdateGroup(id string, name *string, sortOrder *int) error {
	repo.db.Lock()
	defer repo.db.Unlock()

	var setClauses []string
	var args []interface{}

	if name != nil {
		setClauses = append(setClauses, "name = ?")
		args = append(args, *name)
	}
	if sortOrder != nil {
		setClauses = append(setClauses, "sort_order = ?")
		args = append(args, *sortOrder)
	}

	if len(setClauses) == 0 {
		return nil
	}

	setClauses = append(setClauses, "updated_at = ?")
	args = append(args, time.Now())
	args = append(args, id)

	query := `UPDATE "group" SET ` + strings.Join(setClauses, ", ") + " WHERE id = ?"
	_, err := repo.db.Conn.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("更新分组失败: %w", err)
	}
	return nil
}

// DeleteGroup 删除分组记录及关联的 group_resource
// 参数 id: 分组 ID
// 返回: 错误信息
func (repo *GroupRepo) DeleteGroup(id string) error {
	repo.db.Lock()
	defer repo.db.Unlock()

	tx, err := repo.db.Conn.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}

	// 删除分组关联
	if _, err := tx.Exec("DELETE FROM group_resource WHERE group_id = ?", id); err != nil {
		tx.Rollback()
		return fmt.Errorf("删除分组关联失败: %w", err)
	}

	// 删除分组记录
	if _, err := tx.Exec(`DELETE FROM "group" WHERE id = ?`, id); err != nil {
		tx.Rollback()
		return fmt.Errorf("删除分组记录失败: %w", err)
	}

	return tx.Commit()
}

// CheckGroupNameExists 检查同类型下分组名称是否已存在
// 参数 name: 分组名称
// 参数 groupType: 分组类型
// 参数 excludeID: 排除的分组 ID（更新时排除自身）
// 返回: 是否存在、错误信息
func (repo *GroupRepo) CheckGroupNameExists(name, groupType, excludeID string) (bool, error) {
	repo.db.RLock()
	defer repo.db.RUnlock()

	var count int
	query := `SELECT COUNT(*) FROM "group" WHERE type = ? AND name = ?`
	args := []interface{}{groupType, name}
	if excludeID != "" {
		query += ` AND id != ?`
		args = append(args, excludeID)
	}
	err := repo.db.Conn.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("检查分组名称重复失败: %w", err)
	}
	return count > 0, nil
}

// AddResourcesToGroup 批量添加资源到分组
// 参数 groupID: 分组 ID
// 参数 resourceIDs: 资源 ID 列表
// 返回: 错误信息
func (repo *GroupRepo) AddResourcesToGroup(groupID string, resourceIDs []string) error {
	repo.db.Lock()
	defer repo.db.Unlock()

	tx, err := repo.db.Conn.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}

	now := time.Now()
	for _, rid := range resourceIDs {
		_, err := tx.Exec(
			`INSERT OR IGNORE INTO group_resource (group_id, resource_id, created_at) VALUES (?, ?, ?)`,
			groupID, rid, now,
		)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("添加资源到分组失败: %w", err)
		}
	}

	return tx.Commit()
}

// RemoveResourceFromGroup 从分组中移除单个资源
// 参数 groupID: 分组 ID
// 参数 resourceID: 资源 ID
// 返回: 错误信息
func (repo *GroupRepo) RemoveResourceFromGroup(groupID, resourceID string) error {
	repo.db.Lock()
	defer repo.db.Unlock()

	_, err := repo.db.Conn.Exec(
		`DELETE FROM group_resource WHERE group_id = ? AND resource_id = ?`,
		groupID, resourceID,
	)
	if err != nil {
		return fmt.Errorf("从分组移除资源失败: %w", err)
	}
	return nil
}

// IsResourceInGroup 检查资源是否仍在指定分组中
func (repo *GroupRepo) IsResourceInGroup(groupID, resourceID string) bool {
	repo.db.RLock()
	defer repo.db.RUnlock()

	var count int
	_ = repo.db.Conn.QueryRow(
		`SELECT COUNT(*) FROM group_resource WHERE group_id = ? AND resource_id = ?`,
		groupID, resourceID,
	).Scan(&count)
	return count > 0
}

// GetGroupResources 获取分组内所有资源 ID
// 参数 groupID: 分组 ID
// 返回: 资源 ID 列表、错误信息
func (repo *GroupRepo) GetGroupResources(groupID string) ([]string, error) {
	repo.db.RLock()
	defer repo.db.RUnlock()

	rows, err := repo.db.Conn.Query(
		`SELECT resource_id FROM group_resource WHERE group_id = ?`, groupID,
	)
	if err != nil {
		return nil, fmt.Errorf("查询分组资源失败: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("扫描资源 ID 失败: %w", err)
		}
		ids = append(ids, id)
	}
	if ids == nil {
		ids = []string{}
	}
	return ids, nil
}

// GetTrackDeployments 查询分组关联的追踪部署
// 参数 groupID: 分组 ID
// 返回: 部署信息列表、错误信息
func (repo *GroupRepo) GetTrackDeployments(groupID string) ([]model.DeploymentInfo, error) {
	repo.db.RLock()
	defer repo.db.RUnlock()

	rows, err := repo.db.Conn.Query(
		`SELECT id, target_path FROM deployment WHERE group_id = ? AND track = 1`, groupID,
	)
	if err != nil {
		return nil, fmt.Errorf("查询追踪部署失败: %w", err)
	}
	defer rows.Close()

	var deployments []model.DeploymentInfo
	for rows.Next() {
		var d model.DeploymentInfo
		if err := rows.Scan(&d.ID, &d.TargetPath); err != nil {
			return nil, fmt.Errorf("扫描部署信息失败: %w", err)
		}
		deployments = append(deployments, d)
	}
	return deployments, nil
}

// GetGroupDeployments 查询分组关联的所有部署（包含 track 和非 track）
// 参数 groupID: 分组 ID
// 返回: 部署信息列表、错误信息
func (repo *GroupRepo) GetGroupDeployments(groupID string) ([]model.DeploymentInfo, error) {
	repo.db.RLock()
	defer repo.db.RUnlock()

	rows, err := repo.db.Conn.Query(
		`SELECT id, target_path FROM deployment WHERE group_id = ?`, groupID,
	)
	if err != nil {
		return nil, fmt.Errorf("查询分组部署失败: %w", err)
	}
	defer rows.Close()

	var deployments []model.DeploymentInfo
	for rows.Next() {
		var d model.DeploymentInfo
		if err := rows.Scan(&d.ID, &d.TargetPath); err != nil {
			return nil, fmt.Errorf("扫描部署信息失败: %w", err)
		}
		deployments = append(deployments, d)
	}
	return deployments, nil
}

// GetFirstGroupByResourceID 查询资源所属的第一个分组（用于显示分组标签）
// 参数 resourceID: 资源 ID
// 返回: 分组实体指针（无分组时返回 nil）、错误信息
func (repo *GroupRepo) GetFirstGroupByResourceID(resourceID string) (*model.Group, error) {
	repo.db.RLock()
	defer repo.db.RUnlock()

	g := &model.Group{}
	err := repo.db.Conn.QueryRow(
		`SELECT g.id, g.name, g.type, g.color, g.sort_order, g.created_at, g.updated_at
		 FROM "group" g
		 JOIN group_resource gr ON g.id = gr.group_id
		 WHERE gr.resource_id = ?
		 LIMIT 1`, resourceID,
	).Scan(&g.ID, &g.Name, &g.Type, &g.Color, &g.SortOrder, &g.CreatedAt, &g.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询资源分组失败: %w", err)
	}
	return g, nil
}
