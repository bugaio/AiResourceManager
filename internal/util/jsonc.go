// Package util jsonc.go 提供 JSONC（带注释的 JSON）解析和格式化工具
package util

import (
	"encoding/json"

	"github.com/tailscale/hujson"
)

// ParseJSONC 将 JSONC 格式数据（含注释）转换为标准 JSON
// 参数 data: JSONC 格式的字节数据
// 返回: 标准 JSON 字节数据和可能的错误
func ParseJSONC(data []byte) ([]byte, error) {
	ast, err := hujson.Parse(data)
	if err != nil {
		return nil, err
	}
	ast.Standardize()
	return ast.Pack(), nil
}

// FormatJSONC 将 JSON 数据格式化为带 2 空格缩进的标准 JSON
// 参数 data: JSON 字节数据
// 返回: 格式化后的 JSON 字节数据和可能的错误
func FormatJSONC(data []byte) ([]byte, error) {
	var obj interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	return json.MarshalIndent(obj, "", "  ")
}
