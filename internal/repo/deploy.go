// Package repo deploy.go 提供部署表的数据库操作
// 包括部署记录的增删改查、部署子项操作、目标路径聚合
package repo

import (
	"database/sql"
	"fmt"

	"github.com/anthropic/airesourcemanager/internal/model"
)

// DeployRepo 部署数据仓库
type DeployRepo struct {
	db *DB
}

// NewDeployRepo 创建部署数据仓库实例
// 参数 db: 数据库连接
// 返回: DeployRepo 指针
func NewDeployRepo(db *DB) *DeployRepo {
	return &DeployRepo{db: db}
}

// InsertDeployment 插入部署记录
// 参数 d: 部署实体
// 返回: 生成的 ID、错误信息
func (r *DeployRepo) InsertDeployment(d *model.Deployment) (string, error) {
	r.db.Lock()
	defer r.db.Unlock()

	_, err := r.db.Conn.Exec(
		`INSERT INTO deployment (id, group_id, resource_id, target_path, alias_id, deploy_type, track, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.GroupID, d.ResourceID, d.TargetPath, d.AliasID, d.DeployType, d.Track, d.CreatedAt,
	)
	if err != nil {
		return "", fmt.Errorf("插入部署记录失败: %w", err)
	}
	return d.ID, nil
}

// InsertDeploymentItem 插入部署明细
// 参数 item: 部署明细实体
// 返回: 错误信息
func (r *DeployRepo) InsertDeploymentItem(item *model.DeploymentItem) error {
	r.db.Lock()
	defer r.db.Unlock()

	_, err := r.db.Conn.Exec(
		`INSERT INTO deployment_item (id, deployment_id, resource_id, link_path)
		 VALUES (?, ?, ?, ?)`,
		item.ID, item.DeploymentID, item.ResourceID, item.LinkPath,
	)
	if err != nil {
		return fmt.Errorf("插入部署明细失败: %w", err)
	}
	return nil
}

// GetDeploymentByID 根据 ID 查询部署记录
// 参数 id: 部署 UUID
// 返回: 部署实体指针、错误信息
func (r *DeployRepo) GetDeploymentByID(id string) (*model.Deployment, error) {
	r.db.RLock()
	defer r.db.RUnlock()

	d := &model.Deployment{}
	err := r.db.Conn.QueryRow(
		`SELECT id, group_id, resource_id, target_path, alias_id, deploy_type, track, created_at
		 FROM deployment WHERE id = ?`, id,
	).Scan(&d.ID, &d.GroupID, &d.ResourceID, &d.TargetPath, &d.AliasID, &d.DeployType, &d.Track, &d.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询部署记录失败: %w", err)
	}
	return d, nil
}

// UpdateDeploymentAliasID 更新 deployment 的 alias_id
func (r *DeployRepo) UpdateDeploymentAliasID(deploymentID, aliasID string) error {
	r.db.Lock()
	defer r.db.Unlock()
	_, err := r.db.Conn.Exec(
		`UPDATE deployment SET alias_id = ? WHERE id = ?`, aliasID, deploymentID,
	)
	return err
}

// ListDeployments 分页查询部署列表
// 参数 page: 页码
// 参数 pageSize: 每页数量
// 返回: 部署列表、总数、错误信息
func (r *DeployRepo) ListDeployments(page, pageSize int) ([]model.Deployment, int, error) {
	r.db.RLock()
	defer r.db.RUnlock()

	var total int
	if err := r.db.Conn.QueryRow(`SELECT COUNT(*) FROM deployment`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("查询部署总数失败: %w", err)
	}

	offset := (page - 1) * pageSize
	rows, err := r.db.Conn.Query(
		`SELECT id, group_id, resource_id, target_path, alias_id, deploy_type, track, created_at
		 FROM deployment ORDER BY created_at DESC LIMIT ? OFFSET ?`, pageSize, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("查询部署列表失败: %w", err)
	}
	defer rows.Close()

	var list []model.Deployment
	for rows.Next() {
		var d model.Deployment
		if err := rows.Scan(&d.ID, &d.GroupID, &d.ResourceID, &d.TargetPath, &d.AliasID, &d.DeployType, &d.Track, &d.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("扫描部署行失败: %w", err)
		}
		list = append(list, d)
	}
	if list == nil {
		list = []model.Deployment{}
	}
	return list, total, nil
}

// GetDeploymentItems 获取部署的所有明细
// 参数 deploymentID: 部署 ID
// 返回: 明细列表、错误信息
func (r *DeployRepo) GetDeploymentItems(deploymentID string) ([]model.DeploymentItem, error) {
	r.db.RLock()
	defer r.db.RUnlock()

	rows, err := r.db.Conn.Query(
		`SELECT id, deployment_id, resource_id, link_path
		 FROM deployment_item WHERE deployment_id = ?`, deploymentID,
	)
	if err != nil {
		return nil, fmt.Errorf("查询部署明细失败: %w", err)
	}
	defer rows.Close()

	var list []model.DeploymentItem
	for rows.Next() {
		var item model.DeploymentItem
		if err := rows.Scan(&item.ID, &item.DeploymentID, &item.ResourceID, &item.LinkPath); err != nil {
			return nil, fmt.Errorf("扫描部署明细行失败: %w", err)
		}
		list = append(list, item)
	}
	if list == nil {
		list = []model.DeploymentItem{}
	}
	return list, nil
}

// DeleteDeployment 删除部署记录（级联删除明细）
// 参数 id: 部署 ID
// 返回: 错误信息
func (r *DeployRepo) DeleteDeployment(id string) error {
	r.db.Lock()
	defer r.db.Unlock()

	tx, err := r.db.Conn.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}

	if _, err := tx.Exec("DELETE FROM deployment_item WHERE deployment_id = ?", id); err != nil {
		tx.Rollback()
		return fmt.Errorf("删除部署明细失败: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM deployment WHERE id = ?", id); err != nil {
		tx.Rollback()
		return fmt.Errorf("删除部署记录失败: %w", err)
	}

	return tx.Commit()
}

// DeleteDeploymentItem 删除单条部署明细
// 参数 itemID: 明细 ID
// 返回: 错误信息
func (r *DeployRepo) DeleteDeploymentItem(itemID string) error {
	r.db.Lock()
	defer r.db.Unlock()

	_, err := r.db.Conn.Exec("DELETE FROM deployment_item WHERE id = ?", itemID)
	if err != nil {
		return fmt.Errorf("删除部署明细失败: %w", err)
	}
	return nil
}

// GetDeploymentsByTarget 按目标路径聚合查询所有部署
// 返回: target_path → []Deployment 映射、错误信息
func (r *DeployRepo) GetDeploymentsByTarget() (map[string][]model.Deployment, error) {
	r.db.RLock()
	defer r.db.RUnlock()

	rows, err := r.db.Conn.Query(
		`SELECT id, group_id, resource_id, target_path, alias_id, deploy_type, track, created_at
		 FROM deployment ORDER BY target_path, created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("查询部署失败: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]model.Deployment)
	for rows.Next() {
		var d model.Deployment
		if err := rows.Scan(&d.ID, &d.GroupID, &d.ResourceID, &d.TargetPath, &d.AliasID, &d.DeployType, &d.Track, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("扫描部署行失败: %w", err)
		}
		result[d.TargetPath] = append(result[d.TargetPath], d)
	}
	return result, nil
}

// GetAllDeploymentItems 获取所有部署明细
// 返回: 明细列表、错误信息
func (r *DeployRepo) GetAllDeploymentItems() ([]model.DeploymentItem, error) {
	r.db.RLock()
	defer r.db.RUnlock()

	rows, err := r.db.Conn.Query(
		`SELECT id, deployment_id, resource_id, link_path FROM deployment_item`,
	)
	if err != nil {
		return nil, fmt.Errorf("查询所有部署明细失败: %w", err)
	}
	defer rows.Close()

	var list []model.DeploymentItem
	for rows.Next() {
		var item model.DeploymentItem
		if err := rows.Scan(&item.ID, &item.DeploymentID, &item.ResourceID, &item.LinkPath); err != nil {
			return nil, fmt.Errorf("扫描部署明细行失败: %w", err)
		}
		list = append(list, item)
	}
	if list == nil {
		list = []model.DeploymentItem{}
	}
	return list, nil
}

// GetDeploymentItemByID 根据 ID 查询单条部署明细
// 参数 itemID: 明细 ID
// 返回: 明细实体指针、错误信息
func (r *DeployRepo) GetDeploymentItemByID(itemID string) (*model.DeploymentItem, error) {
	r.db.RLock()
	defer r.db.RUnlock()

	item := &model.DeploymentItem{}
	err := r.db.Conn.QueryRow(
		`SELECT id, deployment_id, resource_id, link_path
		 FROM deployment_item WHERE id = ?`, itemID,
	).Scan(&item.ID, &item.DeploymentID, &item.ResourceID, &item.LinkPath)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询部署明细失败: %w", err)
	}
	return item, nil
}

// GetDeploymentItemCount 查询部署下的明细数量
// 参数 deploymentID: 部署 ID
// 返回: 数量、错误信息
func (r *DeployRepo) GetDeploymentItemCount(deploymentID string) (int, error) {
	r.db.RLock()
	defer r.db.RUnlock()

	var count int
	err := r.db.Conn.QueryRow(
		`SELECT COUNT(*) FROM deployment_item WHERE deployment_id = ?`, deploymentID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("查询部署明细数量失败: %w", err)
	}
	return count, nil
}

// GetTrackDeploymentsByGroupID 查询分组的追踪部署
// 参数 groupID: 分组 ID
// 返回: 部署列表、错误信息
func (r *DeployRepo) GetTrackDeploymentsByGroupID(groupID string) ([]model.Deployment, error) {
	r.db.RLock()
	defer r.db.RUnlock()

	rows, err := r.db.Conn.Query(
		`SELECT id, group_id, resource_id, target_path, alias_id, deploy_type, track, created_at
		 FROM deployment WHERE group_id = ? AND track = 1`, groupID,
	)
	if err != nil {
		return nil, fmt.Errorf("查询追踪部署失败: %w", err)
	}
	defer rows.Close()

	var list []model.Deployment
	for rows.Next() {
		var d model.Deployment
		if err := rows.Scan(&d.ID, &d.GroupID, &d.ResourceID, &d.TargetPath, &d.AliasID, &d.DeployType, &d.Track, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("扫描部署行失败: %w", err)
		}
		list = append(list, d)
	}
	return list, nil
}

// GetDeploymentItemsByResourceAndTarget 根据资源 ID 和目标路径查询部署明细
// 参数 resourceID: 资源 ID
// 参数 targetPath: 目标路径
// 返回: 明细列表、错误信息
func (r *DeployRepo) GetDeploymentItemsByResourceAndTarget(resourceID, targetPath string) ([]model.DeploymentItem, error) {
	r.db.RLock()
	defer r.db.RUnlock()

	rows, err := r.db.Conn.Query(
		`SELECT di.id, di.deployment_id, di.resource_id, di.link_path
		 FROM deployment_item di
		 JOIN deployment d ON di.deployment_id = d.id
		 WHERE di.resource_id = ? AND d.target_path = ?`, resourceID, targetPath,
	)
	if err != nil {
		return nil, fmt.Errorf("查询资源目标部署明细失败: %w", err)
	}
	defer rows.Close()

	var list []model.DeploymentItem
	for rows.Next() {
		var item model.DeploymentItem
		if err := rows.Scan(&item.ID, &item.DeploymentID, &item.ResourceID, &item.LinkPath); err != nil {
			return nil, fmt.Errorf("扫描部署明细行失败: %w", err)
		}
		list = append(list, item)
	}
	return list, nil
}

// GetDeploymentItemsByResourceID 根据资源 ID 查询相关部署明细
// 参数 resourceID: 资源 ID
// 返回: 明细列表、错误信息
func (r *DeployRepo) GetDeploymentItemsByResourceID(resourceID string) ([]model.DeploymentItem, error) {
	r.db.RLock()
	defer r.db.RUnlock()

	rows, err := r.db.Conn.Query(
		`SELECT id, deployment_id, resource_id, link_path
		 FROM deployment_item WHERE resource_id = ?`, resourceID,
	)
	if err != nil {
		return nil, fmt.Errorf("查询资源部署明细失败: %w", err)
	}
	defer rows.Close()

	var list []model.DeploymentItem
	for rows.Next() {
		var item model.DeploymentItem
		if err := rows.Scan(&item.ID, &item.DeploymentID, &item.ResourceID, &item.LinkPath); err != nil {
			return nil, fmt.Errorf("扫描部署明细行失败: %w", err)
		}
		list = append(list, item)
	}
	return list, nil
}
