// Package main 是 AiResourceManager 的入口文件，负责启动 CLI 应用
package main

import (
	"github.com/anthropic/airesourcemanager/cmd"
)

func main() {
	// 将嵌入的前端静态资源传递给 cmd 包
	cmd.EmbeddedFS = WebDistFS()
	cmd.Execute()
}
