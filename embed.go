// Package main embed.go 负责嵌入前端静态资源和配置文件
package main

import (
	"embed"
	"io/fs"
)

//go:embed all:configs
var configFS embed.FS

//go:embed all:web/dist
var webDist embed.FS

// WebDistFS 返回前端静态资源的子文件系统（web/dist 目录）
// 返回: 去掉 "web/dist" 前缀的 fs.FS；若嵌入为空则返回 nil
func WebDistFS() fs.FS {
	subFS, err := fs.Sub(webDist, "web/dist")
	if err != nil {
		return nil
	}
	return subFS
}
