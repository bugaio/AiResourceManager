// Package util file.go 提供跨平台的文件和路径操作工具
package util

import (
	"os"
	"path/filepath"
	"strings"
)

// HomeDir 获取当前用户的主目录路径
// 返回: 主目录绝对路径和可能的错误
func HomeDir() (string, error) {
	return os.UserHomeDir()
}

// ExpandPath 展开路径中的 ~ 为实际主目录
// 参数 path: 可能包含 ~ 前缀的路径
// 返回: 展开后的绝对路径和可能的错误
func ExpandPath(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return filepath.Abs(path)
	}

	home, err := HomeDir()
	if err != nil {
		return "", err
	}

	if path == "~" {
		return home, nil
	}

	// ~/something 或 ~\something
	return filepath.Join(home, path[2:]), nil
}

// EnsureDir 确保目录存在，如果不存在则递归创建
// 参数 dir: 目录路径
// 返回: 创建过程中的错误
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// FileExists 检查文件是否存在
// 参数 path: 文件路径
// 返回: 文件是否存在
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// IsDir 检查路径是否为目录
// 参数 path: 路径
// 返回: 是否为目录
func IsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// ReadFileContent 读取文件全部内容并返回字符串
// 参数 path: 文件绝对路径
// 返回: 文件内容字符串和可能的错误
func ReadFileContent(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteFileContent 将字符串内容写入文件，若父目录不存在则自动创建
// 参数 path: 文件绝对路径
// 参数 content: 要写入的字符串内容
// 返回: 写入过程中的错误
func WriteFileContent(path string, content string) error {
	if err := EnsureParentDir(path); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

// RemoveFileOrDir 删除文件或目录，若目标不存在则静默跳过不报错
// 参数 path: 文件或目录路径
// 返回: 删除过程中的错误（目标不存在时返回 nil）
func RemoveFileOrDir(path string) error {
	err := os.RemoveAll(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// EnsureParentDir 确保文件路径的父目录存在，不存在则递归创建
// 参数 path: 文件路径（非目录路径）
// 返回: 创建过程中的错误
func EnsureParentDir(path string) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0755)
}
