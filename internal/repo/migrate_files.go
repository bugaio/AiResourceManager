// Package repo migrate_files.go 提供文件系统层面的数据迁移
package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anthropic/airesourcemanager/internal/util"
)

// MigrateAgentFilesToMdConfig 把 type=mdconfig 但 path 仍指向 agents/ 目录的资源
// 迁移到 mdconfigs/ 目录，并重建已部署的 symlink。幂等：只处理 path 含 /agents/ 的记录。
func MigrateAgentFilesToMdConfig(db *DB, baseDir string) error {
	// 1. 查询所有仍指向 agents/ 的 mdconfig 资源
	rows, err := db.Conn.Query(`SELECT id, path FROM resource WHERE type = 'mdconfig' AND path LIKE '%/agents/%'`)
	if err != nil {
		return fmt.Errorf("查询待迁移资源失败: %w", err)
	}
	type row struct {
		id, oldPath string
	}
	var pending []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.oldPath); err != nil {
			rows.Close()
			return fmt.Errorf("读取资源记录失败: %w", err)
		}
		pending = append(pending, r)
	}
	rows.Close()

	if len(pending) == 0 {
		return nil // 幂等：无可迁移项
	}

	var migrated, skipped int
	for _, r := range pending {
		// 2a. 计算新路径：替换首个 /agents/ 段为 /mdconfigs/
		newPath := strings.Replace(r.oldPath, "/agents/", "/mdconfigs/", 1)

		// 2b. 旧文件不存在 → 已迁移过或被删，跳过文件移动但仍尝试更新 DB（path 已不含 agents 则跳过）
		if _, err := os.Stat(r.oldPath); os.IsNotExist(err) {
			// 文件不在旧路径。若新路径已存在则只更新 DB；否则无法迁移，跳过。
			if _, e2 := os.Stat(newPath); e2 == nil {
				db.Lock()
				_, _ = db.Conn.Exec("UPDATE resource SET path = ? WHERE id = ?", newPath, r.id)
				db.Unlock()
				migrated++
			}
			continue
		}

		// 2c. 确保新目录存在并移动文件
		if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
			fmt.Fprintf(os.Stderr, "[migrate] 创建目录失败 %s: %v\n", filepath.Dir(newPath), err)
			skipped++
			continue
		}
		if err := os.Rename(r.oldPath, newPath); err != nil {
			fmt.Fprintf(os.Stderr, "[migrate] 移动文件失败 %s → %s: %v\n", r.oldPath, newPath, err)
			skipped++
			continue
		}

		// 2d. 更新 DB path
		db.Lock()
		_, err = db.Conn.Exec("UPDATE resource SET path = ? WHERE id = ?", newPath, r.id)
		db.Unlock()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[migrate] 更新 DB path 失败 id=%s: %v\n", r.id, err)
			skipped++
			continue
		}

		// 2e. 重建指向旧路径的已部署 symlink
		rebuildSymlinksForResource(db, r.id, r.oldPath, newPath)
		migrated++
	}

	if migrated > 0 || skipped > 0 {
		fmt.Fprintf(os.Stderr, "[migrate] agent→mdconfig 文件迁移完成: 迁移 %d, 跳过 %d\n", migrated, skipped)
	}
	return nil
}

// rebuildSymlinksForResource 重建该资源所有指向 oldPath 的已部署 symlink，改为指向 newPath
func rebuildSymlinksForResource(db *DB, resourceID, oldPath, newPath string) {
	rows, err := db.Conn.Query(`SELECT link_path FROM deployment_item WHERE resource_id = ?`, resourceID)
	if err != nil {
		return
	}
	var links []string
	for rows.Next() {
		var lp string
		if rows.Scan(&lp) == nil {
			links = append(links, lp)
		}
	}
	rows.Close()

	for _, linkPath := range links {
		// 读取 symlink 当前指向
		target, err := os.Readlink(linkPath)
		if err != nil {
			// 不是 symlink 或不存在；若是死链 Lstat 成功但 Readlink 失败，尝试清理重建
			if _, lerr := os.Lstat(linkPath); lerr == nil {
				// ponytail: 死链兜底，直接删后重建指向新源
				os.Remove(linkPath)
				_ = util.CreateSymlink(newPath, linkPath)
			}
			continue
		}
		// 指向旧 agents 路径 → 重建指向新 mdconfigs 路径
		if target == oldPath {
			os.Remove(linkPath)
			if err := util.CreateSymlink(newPath, linkPath); err != nil {
				fmt.Fprintf(os.Stderr, "[migrate] 重建 symlink 失败 %s: %v\n", linkPath, err)
			}
			continue
		}
		// 死链（目标文件不存在）且 target 含 /agents/ → 也重建
		if strings.Contains(target, "/agents/") {
			if _, serr := os.Stat(target); serr != nil {
				os.Remove(linkPath)
				_ = util.CreateSymlink(newPath, linkPath)
			}
		}
	}
}
