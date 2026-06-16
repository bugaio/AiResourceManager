// Package repo resource.go 提供资源表的数据库操作
// 包括增删改查、分页列表、关联查询等
package repo

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/anthropic/airesourcemanager/internal/model"
)

// ResourceRepo 资源数据仓库
type ResourceRepo struct {
	db *DB
}

// NewResourceRepo 创建资源数据仓库实例
// 参数 db: 数据库连接
// 返回: ResourceRepo 指针
func NewResourceRepo(db *DB) *ResourceRepo {
	return &ResourceRepo{db: db}
}

// InsertResource 插入新资源记录
// 参数 r: 资源实体
// 返回: 错误信息
func (repo *ResourceRepo) InsertResource(r *model.Resource) error {
	repo.db.Lock()
	defer repo.db.Unlock()

	_, err := repo.db.Conn.Exec(
		`INSERT INTO resource (id, name, type, path, description, metadata, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.Name, r.Type, r.Path, r.Description, r.Metadata, r.CreatedAt, r.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("插入资源失败: %w", err)
	}
	return nil
}

// GetResourceByID 根据 ID 查询资源
// 参数 id: 资源 UUID
// 返回: 资源实体指针和错误信息
func (repo *ResourceRepo) GetResourceByID(id string) (*model.Resource, error) {
	repo.db.RLock()
	defer repo.db.RUnlock()

	r := &model.Resource{}
	err := repo.db.Conn.QueryRow(
		`SELECT id, name, type, path, description, metadata, created_at, updated_at
		 FROM resource WHERE id = ?`, id,
	).Scan(&r.ID, &r.Name, &r.Type, &r.Path, &r.Description, &r.Metadata, &r.CreatedAt, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询资源失败: %w", err)
	}
	return r, nil
}

// CheckNameExists 检查同类型下资源名称是否已存在
// 参数 resourceType: 资源类型
// 参数 name: 资源名称
// 参数 excludeID: 排除的资源 ID（更新时排除自身）
// 返回: 是否存在、错误信息
func (repo *ResourceRepo) CheckNameExists(resourceType, name, excludeID string) (bool, error) {
	repo.db.RLock()
	defer repo.db.RUnlock()

	var count int
	query := `SELECT COUNT(*) FROM resource WHERE type = ? AND name = ?`
	args := []interface{}{resourceType, name}
	if excludeID != "" {
		query += ` AND id != ?`
		args = append(args, excludeID)
	}
	err := repo.db.Conn.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("检查名称重复失败: %w", err)
	}
	return count > 0, nil
}

// ListResources 分页查询资源列表
// 参数 resourceType: 资源类型筛选
// 参数 search: 名称模糊搜索关键词
// 参数 groupID: 分组 ID，"0" 或空表示不筛选
// 参数 page: 页码（从 1 开始）
// 参数 pageSize: 每页数量
// 返回: 资源列表、总数、错误信息
func (repo *ResourceRepo) ListResources(resourceType, search, groupID string, page, pageSize int) ([]model.Resource, int, error) {
	repo.db.RLock()
	defer repo.db.RUnlock()

	var conditions []string
	var args []interface{}

	fromClause := `FROM resource r`

	if resourceType != "" {
		conditions = append(conditions, "r.type = ?")
		args = append(args, resourceType)
	}
	if search != "" {
		conditions = append(conditions, "r.name LIKE ?")
		args = append(args, "%"+search+"%")
	}
	if groupID != "" && groupID != "0" {
		fromClause += ` INNER JOIN group_resource gr ON r.id = gr.resource_id`
		conditions = append(conditions, "gr.group_id = ?")
		args = append(args, groupID)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	// 查询总数
	countQuery := "SELECT COUNT(*) " + fromClause + whereClause
	var total int
	if err := repo.db.Conn.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("查询资源总数失败: %w", err)
	}

	// 分页查询
	offset := (page - 1) * pageSize
	listQuery := "SELECT r.id, r.name, r.type, r.path, r.description, r.metadata, r.created_at, r.updated_at " +
		fromClause + whereClause + " ORDER BY r.created_at DESC LIMIT ? OFFSET ?"
	listArgs := append(args, pageSize, offset)

	rows, err := repo.db.Conn.Query(listQuery, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询资源列表失败: %w", err)
	}
	defer rows.Close()

	var list []model.Resource
	for rows.Next() {
		var r model.Resource
		if err := rows.Scan(&r.ID, &r.Name, &r.Type, &r.Path, &r.Description, &r.Metadata, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("扫描资源行失败: %w", err)
		}
		list = append(list, r)
	}
	if list == nil {
		list = []model.Resource{}
	}
	return list, total, nil
}

// UpdateResource 更新资源名称和描述
// 参数 id: 资源 ID
// 参数 name: 新名称（空字符串表示不更新）
// 参数 description: 新描述（nil 表示不更新）
// 返回: 错误信息
func (repo *ResourceRepo) UpdateResource(id string, name *string, description *string) error {
	repo.db.Lock()
	defer repo.db.Unlock()

	var setClauses []string
	var args []interface{}

	if name != nil {
		setClauses = append(setClauses, "name = ?")
		args = append(args, *name)
	}
	if description != nil {
		setClauses = append(setClauses, "description = ?")
		args = append(args, *description)
	}

	if len(setClauses) == 0 {
		return nil
	}

	setClauses = append(setClauses, "updated_at = ?")
	args = append(args, time.Now())
	args = append(args, id)

	query := "UPDATE resource SET " + strings.Join(setClauses, ", ") + " WHERE id = ?"
	_, err := repo.db.Conn.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("更新资源失败: %w", err)
	}
	return nil
}

// DeleteResource 删除资源记录及关联的 group_resource
// 参数 id: 资源 ID
// 返回: 错误信息
func (repo *ResourceRepo) DeleteResource(id string) error {
	repo.db.Lock()
	defer repo.db.Unlock()

	tx, err := repo.db.Conn.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}

	// 删除分组关联
	if _, err := tx.Exec("DELETE FROM group_resource WHERE resource_id = ?", id); err != nil {
		tx.Rollback()
		return fmt.Errorf("删除分组关联失败: %w", err)
	}

	// 删除资源记录
	if _, err := tx.Exec("DELETE FROM resource WHERE id = ?", id); err != nil {
		tx.Rollback()
		return fmt.Errorf("删除资源记录失败: %w", err)
	}

	return tx.Commit()
}

// GetResourceDeployments 查询资源的部署关联信息
// 参数 resourceID: 资源 ID
// 返回: 部署信息列表、错误信息
func (repo *ResourceRepo) GetResourceDeployments(resourceID string) ([]model.DeploymentInfo, error) {
	repo.db.RLock()
	defer repo.db.RUnlock()

	rows, err := repo.db.Conn.Query(
		`SELECT d.id, d.target_path FROM deployment d
		 INNER JOIN deployment_item di ON d.id = di.deployment_id
		 WHERE di.resource_id = ?
		 GROUP BY d.id`, resourceID,
	)
	if err != nil {
		return nil, fmt.Errorf("查询部署关联失败: %w", err)
	}
	defer rows.Close()

	var deployments []model.DeploymentInfo
	for rows.Next() {
		var info model.DeploymentInfo
		if err := rows.Scan(&info.ID, &info.TargetPath); err != nil {
			return nil, fmt.Errorf("扫描部署信息失败: %w", err)
		}
		deployments = append(deployments, info)
	}
	return deployments, nil
}

// TouchResource 仅更新资源的 updated_at 时间戳
// 用于文件监听器检测到变更时刷新时间，不修改其他字段
// 参数 id: 资源 ID
// 返回: 错误信息
func (repo *ResourceRepo) TouchResource(id string) error {
	repo.db.Lock()
	defer repo.db.Unlock()

	_, err := repo.db.Conn.Exec(
		`UPDATE resource SET updated_at = ? WHERE id = ?`,
		time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("触摸资源时间戳失败: %w", err)
	}
	return nil
}

// InsertGroupResource 添加资源到分组
// 参数 groupID: 分组 ID
// 参数 resourceID: 资源 ID
// 返回: 错误信息
func (repo *ResourceRepo) InsertGroupResource(groupID, resourceID string) error {
	repo.db.Lock()
	defer repo.db.Unlock()

	_, err := repo.db.Conn.Exec(
		`INSERT OR IGNORE INTO group_resource (group_id, resource_id, created_at) VALUES (?, ?, ?)`,
		groupID, resourceID, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("添加分组关联失败: %w", err)
	}
	return nil
}
