// Package banner 负责启动时打印应用横幅
// 包含哆啦A梦 ASCII 艺术、版本信息和运行配置展示
package banner

import (
	"fmt"
	"os"
	"strings"
)

// Version 应用版本号
const Version = "v0.1.0"

// ANSI 颜色码
const (
	colorReset  = "\033[0m"
	colorCyan   = "\033[36m"
	colorBlue   = "\033[34m"
	colorYellow = "\033[33m"
	colorWhite  = "\033[37m"
	colorBold   = "\033[1m"
)

// isColorSupported 检测终端是否支持颜色输出
func isColorSupported() bool {
	// NO_COLOR 环境变量是通用的禁用颜色标准
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	term := os.Getenv("TERM")
	if term == "dumb" {
		return false
	}
	// 检测是否为终端
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// Print 打印启动横幅
// 参数 port: 服务监听端口
// 参数 dbPath: 数据库文件路径
// 参数 logLevel: 日志级别
func Print(port int, dbPath string, logLevel string) {
	useColor := isColorSupported()

	cyan := ""
	blue := ""
	yellow := ""
	white := ""
	bold := ""
	reset := ""

	if useColor {
		cyan = colorCyan
		blue = colorBlue
		yellow = colorYellow
		white = colorWhite
		bold = colorBold
		reset = colorReset
	}

	// 哆啦A梦 ASCII 艺术（紧凑版，约12行）
	doraemon := []string{
		blue + `      ╭──────────────────────────────────╮` + reset,
		blue + `      │` + cyan + `        ████████████████          ` + blue + `│` + reset,
		blue + `      │` + cyan + `      ██` + white + `██████████████` + cyan + `██        ` + blue + `│` + reset,
		blue + `      │` + cyan + `    ██` + white + `██  ████  ████████` + cyan + `██      ` + blue + `│` + reset,
		blue + `      │` + cyan + `    ██` + white + `██  ●  ██  ●  ████` + cyan + `██      ` + blue + `│` + reset,
		blue + `      │` + cyan + `    ██` + white + `████████` + yellow + `◆` + white + `████████` + cyan + `██      ` + blue + `│` + reset,
		blue + `      │` + cyan + `    ██` + white + `██  ╰──────╯  ████` + cyan + `██      ` + blue + `│` + reset,
		blue + `      │` + cyan + `      ██` + white + `██████████████` + cyan + `██        ` + blue + `│` + reset,
		blue + `      │` + cyan + `        ██` + yellow + `╭━◯━╮` + cyan + `████          ` + blue + `│` + reset,
		blue + `      │` + cyan + `      ████` + yellow + `┃    ┃` + cyan + `████          ` + blue + `│` + reset,
		blue + `      │` + cyan + `      ██████████████            ` + blue + `│` + reset,
		blue + `      │                                  │` + reset,
		blue + `      │` + bold + white + `    AiResourceManager  ` + reset + yellow + Version + blue + `     │` + reset,
		blue + `      ╰──────────────────────────────────╯` + reset,
	}

	info := []string{
		"",
		bold + cyan + "  功能: " + reset + white + "AI 资源集中管理与分发" + reset,
		bold + cyan + "  端口: " + reset + white + fmt.Sprintf("%d", port) + reset,
		bold + cyan + "  数据库: " + reset + white + dbPath + reset,
		bold + cyan + "  日志级别: " + reset + white + logLevel + reset,
		"",
	}

	fmt.Println()
	for _, line := range doraemon {
		fmt.Println(line)
	}
	for _, line := range info {
		fmt.Println(line)
	}
	fmt.Println(strings.Repeat("─", 46))
	fmt.Println()
}
