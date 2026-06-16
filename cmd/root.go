// Package cmd 提供 CLI 命令的根定义，基于 cobra 框架
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// 构建时通过 ldflags 注入的版本信息
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "aimanager",
	Short: "AI 资源管理器 - 统一管理 AI 项目资源",
	Long:  "AiResourceManager 是一个用于统一管理 AI 项目资源的工具，支持资源分组、部署追踪和路径别名等功能。",
}

// versionCmd 显示版本信息子命令
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示版本信息",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("AiResourceManager %s\nCommit: %s\nBuilt:  %s\n", Version, Commit, Date)
	},
}

// Execute 执行根命令，是 CLI 的总入口
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().IntP("port", "p", 3678, "服务监听端口")
	rootCmd.PersistentFlags().StringP("config", "c", "", "配置文件路径")
	rootCmd.AddCommand(versionCmd)
}
