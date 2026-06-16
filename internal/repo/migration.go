// Package repo migration.go 提供数据库迁移功能
// 读取 migrations 目录下的 SQL 文件，按序号执行未应用的迁移
package repo

import (
	"embed"
	"fmt"
	"sort"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// RunMigrations 执行所有未应用的数据库迁移
// 参数 db: 数据库连接实例
// 返回: 执行过程中的错误
// 说明: 通过 _migrations 表记录已执行的迁移，避免重复执行
func RunMigrations(db *DB) error {
	db.Lock()
	defer db.Unlock()

	// 确保迁移记录表存在
	_, err := db.Conn.Exec(`
		CREATE TABLE IF NOT EXISTS _migrations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("创建迁移记录表失败: %w", err)
	}

	// 读取所有迁移文件
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("读取迁移目录失败: %w", err)
	}

	// 按文件名排序确保执行顺序
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		// 检查是否已经执行过
		var count int
		err := db.Conn.QueryRow("SELECT COUNT(*) FROM _migrations WHERE name = ?", name).Scan(&count)
		if err != nil {
			return fmt.Errorf("检查迁移状态失败 [%s]: %w", name, err)
		}
		if count > 0 {
			continue // 已执行，跳过
		}

		// 读取并执行迁移 SQL
		content, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("读取迁移文件失败 [%s]: %w", name, err)
		}

		tx, err := db.Conn.Begin()
		if err != nil {
			return fmt.Errorf("开始事务失败 [%s]: %w", name, err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("执行迁移失败 [%s]: %w", name, err)
		}

		// 记录迁移
		if _, err := tx.Exec("INSERT INTO _migrations (name, applied_at) VALUES (?, ?)", name, time.Now()); err != nil {
			tx.Rollback()
			return fmt.Errorf("记录迁移失败 [%s]: %w", name, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("提交迁移事务失败 [%s]: %w", name, err)
		}
	}

	return nil
}
