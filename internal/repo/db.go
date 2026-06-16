// Package repo db.go 提供 SQLite 数据库的初始化和连接管理
// 使用 WAL 模式提升并发读性能，通过 sync.RWMutex 保证写入安全
package repo

import (
	"database/sql"
	"fmt"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

// DB 数据库连接封装，内含读写锁以保证并发安全
type DB struct {
	Conn *sql.DB
	mu   sync.RWMutex
}

// NewDB 创建并初始化 SQLite 数据库连接
// 参数 dbPath: 数据库文件路径
// 返回: DB 实例和可能的错误
// 说明: 启用 WAL 模式以提升并发读取性能
func NewDB(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// 验证连接可用
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("数据库连接测试失败: %w", err)
	}

	// 启用 WAL 模式
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("启用 WAL 模式失败: %w", err)
	}

	// 启用外键约束
	if _, err := conn.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return nil, fmt.Errorf("启用外键约束失败: %w", err)
	}

	return &DB{Conn: conn}, nil
}

// Close 关闭数据库连接
func (db *DB) Close() error {
	return db.Conn.Close()
}

// RLock 获取读锁
func (db *DB) RLock() {
	db.mu.RLock()
}

// RUnlock 释放读锁
func (db *DB) RUnlock() {
	db.mu.RUnlock()
}

// Lock 获取写锁
func (db *DB) Lock() {
	db.mu.Lock()
}

// Unlock 释放写锁
func (db *DB) Unlock() {
	db.mu.Unlock()
}
