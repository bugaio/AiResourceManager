// Package util path_format.go 提供按资源类型校验路径格式的工具
//
// 与前端 AliasesView.vue 的格式约束保持一致:
//   - skill / agent: 必须是目录（不能包含已知文件扩展名）
//   - config:        必须是 .json/.jsonc/.yaml/.yml/.toml 文件
//   - prompt:        必须是 .md 文件
package util

import (
	"path/filepath"
	"strings"
)

// IsDirectoryPath 判定路径看起来像目录（不带任何后缀），用于 skill / agent 校验
// 注意：这是字符串级别检查，不真的去访问文件系统
func IsDirectoryPath(p string) bool {
	if p == "" {
		return false
	}
	return filepath.Ext(p) == ""
}

// ValidatePathByType 按资源类型校验路径格式
// 返回错误信息（空字符串表示通过）
func ValidatePathByType(resourceType, path string) string {
	if path == "" {
		return ""
	}
	switch resourceType {
	case "skill", "agent":
		if !IsDirectoryPath(path) {
			return "skill/agent 路径必须是目录（不能带文件后缀）"
		}
	case "config":
		if !IsConfigFile(path) {
			return "config 路径后缀必须是 .json/.jsonc/.yaml/.yml/.toml"
		}
	case "prompt":
		if !IsPromptFile(path) {
			return "prompt 路径后缀必须是 .md"
		}
	default:
		return "未知资源类型: " + resourceType
	}
	return ""
}

// HasKnownExt 判定路径后缀是否在已知后缀列表中（任意一种类型的合法后缀）
func HasKnownExt(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".md", ".json", ".jsonc", ".yaml", ".yml", ".toml":
		return true
	}
	return false
}
