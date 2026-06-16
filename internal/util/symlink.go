// Package util symlink.go 提供符号链接操作工具
package util

import (
	"fmt"
	"os"
	"path/filepath"
)

// CreateSymlink 创建符号链接
// 参数 source: 源文件/目录路径
// 参数 target: 符号链接路径
// 返回: 创建过程中的错误
// 说明: 如果目标已存在会先删除再创建
func CreateSymlink(source, target string) error {
	// 确保源路径存在
	if _, err := os.Stat(source); os.IsNotExist(err) {
		return fmt.Errorf("源路径不存在: %s", source)
	}

	// 确保目标目录存在
	targetDir := filepath.Dir(target)
	if err := EnsureDir(targetDir); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	// 如果目标已存在，先删除
	if _, err := os.Lstat(target); err == nil {
		if err := os.Remove(target); err != nil {
			return fmt.Errorf("删除已存在的目标失败: %w", err)
		}
	}

	return os.Symlink(source, target)
}

// ReadSymlink 读取符号链接指向的实际路径
// 参数 path: 符号链接路径
// 返回: 实际路径和可能的错误
func ReadSymlink(path string) (string, error) {
	return os.Readlink(path)
}

// IsSymlink 检查路径是否为符号链接
// 参数 path: 路径
// 返回: 是否为符号链接
func IsSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}
