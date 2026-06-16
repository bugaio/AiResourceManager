// Package util uuid.go 提供 UUID 生成功能
// 使用 crypto/rand 生成符合 UUID v4 标准的唯一标识符
package util

import (
	"crypto/rand"
	"fmt"
)

// NewUUID 生成一个新的 UUID v4 字符串
// 使用 crypto/rand 保证密码学安全的随机性
// 返回: 格式为 xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx 的 UUID 字符串
func NewUUID() string {
	uuid := make([]byte, 16)
	_, _ = rand.Read(uuid)

	// 设置版本号为 4
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	// 设置变体为 RFC 4122
	uuid[8] = (uuid[8] & 0x3f) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}
