// Package util prompt.go 提供 prompt 资源的标记/分隔符工具
//
// prompt 部署语义: 把资源全文以注释标记包裹后,追加到目标 .md 文件末尾
// 注释标记使用资源 UUID 作为锚点,保证全局唯一,撤销/二次部署时按 UUID 精准定位
package util

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// Prompt 标记常量模板
// 使用方法: fmt.Sprintf(PromptMarkerStartTpl, uuid)
const (
	PromptMarkerStartTpl = "<!-- prompt:%s start -->"
	PromptMarkerEndTpl   = "<!-- prompt:%s end -->"
)

// IsPromptFile 判断路径是否为 prompt 目标文件 (后缀 .md, 大小写不敏感)
func IsPromptFile(path string) bool {
	return strings.ToLower(filepath.Ext(path)) == ".md"
}

// BuildPromptStartMarker 构造资源 UUID 对应的开始标记
func BuildPromptStartMarker(uuid string) string {
	return fmt.Sprintf(PromptMarkerStartTpl, uuid)
}

// BuildPromptEndMarker 构造资源 UUID 对应的结束标记
func BuildPromptEndMarker(uuid string) string {
	return fmt.Sprintf(PromptMarkerEndTpl, uuid)
}

// HasPromptMarkers 判定 text 中是否完整包含 uuid 对应的标记块
// 遍历所有 start 位置，对每个 start 在其后查找 end；
// 只要找到任意一对 (start_i, end_j) 满足 start_i < end_j → 返回 true。
// 全部 start 之后都没有匹配的 end → false。
// 设计意图: 目标文件可能含残留损坏块（用户手动编辑），不应因为
// 第一个 start 之后找不到 end 就误判为不存在。
func HasPromptMarkers(text, uuid string) bool {
	start := BuildPromptStartMarker(uuid)
	end := BuildPromptEndMarker(uuid)

	pos := 0
	for {
		s := strings.Index(text[pos:], start)
		if s < 0 {
			return false
		}
		s += pos // 转为绝对位置

		// 在该 start 之后查找 end
		e := strings.Index(text[s+len(start):], end)
		if e >= 0 {
			return true // 找到有效对
		}

		// 该 start 之后无 end，继续从下一个位置查找
		pos = s + 1
	}
}

// BuildPromptBlock 拼接资源标记块
// 形式:
//
//	\n<start>\n{trimmed content}\n<end>\n
//
// content 内首尾空白会被裁剪,内部内容原样保留
func BuildPromptBlock(uuid, content string) string {
	trimmed := strings.TrimSpace(content)
	return fmt.Sprintf("\n%s\n%s\n%s\n",
		BuildPromptStartMarker(uuid),
		trimmed,
		BuildPromptEndMarker(uuid),
	)
}

// StripPromptBlock 从 text 中删除所有 uuid 对应的标记块
// 正则匹配整块 (含前后空行规范化为单空行)
// 出现多次 (异常情况) 时一并删除
func StripPromptBlock(text, uuid string) string {
	start := regexp.QuoteMeta(BuildPromptStartMarker(uuid))
	end := regexp.QuoteMeta(BuildPromptEndMarker(uuid))
	// (?s) 让 . 匹配换行
	// 前后允许多个空行 \n*,被替换为单个 \n,保证不留多余空行
	pattern := fmt.Sprintf(`(?s)\n*%s.*?%s\n*`, start, end)
	re := regexp.MustCompile(pattern)
	out := re.ReplaceAllString(text, "\n")
	// 规范文件末尾仅保留一个 \n
	out = strings.TrimRight(out, "\n") + "\n"
	// 防御: 文件可能本来就空,保持空字符串而不是单 \n
	if strings.TrimSpace(out) == "" {
		return ""
	}
	return out
}

// EnsureTrailingNewline 保证 text 以 \n 结尾 (空字符串原样返回)
// 用于 prompt 追加部署前规范化目标文件末尾
func EnsureTrailingNewline(text string) string {
	if text == "" {
		return text
	}
	if strings.HasSuffix(text, "\n") {
		return text
	}
	return text + "\n"
}
