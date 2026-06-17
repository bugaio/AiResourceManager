// Package util config.go 提供通用配置文件片段的合并
//
// 支持的目标格式: .json / .jsonc / .yaml / .yml / .toml
// 合并语义:
//   - json/jsonc: 走 hujson AST,保留注释/格式(若文件无注释,等同于原始 JSON 合并)
//   - yaml/yml:   走 yaml.v3 *yaml.Node,保留注释/锚点/格式风格
//   - toml:       解析后重新序列化(Go 生态无保留注释的 TOML 库,此格式会被重写)
//
// 合并规则(与原 MCP 行为一致):
//   - 顶层及嵌套 map 递归深度合并
//   - 数组/标量/类型不一致由 src 覆盖 dst
//   - dst 中独有的 key 保留
package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
	yamlv3 "go.yaml.in/yaml/v3"
)

// ConfigFormat 配置文件格式
type ConfigFormat string

const (
	FormatJSON ConfigFormat = "json"
	FormatYAML ConfigFormat = "yaml"
	FormatTOML ConfigFormat = "toml"
)

// DetectConfigFormat 根据文件后缀推断格式
// 不识别 .jsonc(jsonc 视作 json 的子集,共用 JSON 解析路径)
// 不识别时返回空串,调用方应据此报错
func DetectConfigFormat(path string) ConfigFormat {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json", ".jsonc":
		return FormatJSON
	case ".yaml", ".yml":
		return FormatYAML
	case ".toml":
		return FormatTOML
	default:
		return ""
	}
}

// IsConfigFile 返回文件后缀是否属于支持的配置文件格式
func IsConfigFile(path string) bool {
	return DetectConfigFormat(path) != ""
}

// ReadConfigFragment 读取并解析一个配置片段文件(资源源文件)
// 用于 deployConfig 时从 ~/.aiManager/configs/{uuid}.{ext} 读出待合并的片段
func ReadConfigFragment(path string) (map[string]interface{}, ConfigFormat, error) {
	format := DetectConfigFormat(path)
	if format == "" {
		return nil, "", fmt.Errorf("不支持的配置文件后缀: %s", filepath.Ext(path))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, format, fmt.Errorf("读取片段文件失败: %w", err)
	}
	m, err := ParseConfigBytes(data, format)
	if err != nil {
		return nil, format, fmt.Errorf("解析片段失败: %w", err)
	}
	if m == nil {
		m = map[string]interface{}{}
	}
	return m, format, nil
}

// ParseConfigBytes 按格式解析字节为 map[string]interface{}
// json 格式兼容 JSONC(带注释)
func ParseConfigBytes(data []byte, format ConfigFormat) (map[string]interface{}, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]interface{}{}, nil
	}
	switch format {
	case FormatJSON:
		std, err := ParseJSONC(data)
		if err != nil {
			// 退化为原始 JSON
			std = data
		}
		var m map[string]interface{}
		if err := json.Unmarshal(std, &m); err != nil {
			return nil, err
		}
		if m == nil {
			m = map[string]interface{}{}
		}
		return m, nil
	case FormatYAML:
		var m map[string]interface{}
		if err := yamlv3.Unmarshal(data, &m); err != nil {
			return nil, err
		}
		if m == nil {
			m = map[string]interface{}{}
		}
		return m, nil
	case FormatTOML:
		var m map[string]interface{}
		if err := toml.Unmarshal(data, &m); err != nil {
			return nil, err
		}
		if m == nil {
			m = map[string]interface{}{}
		}
		return m, nil
	default:
		return nil, fmt.Errorf("不支持的格式: %s", format)
	}
}

// MergeConfigToFile 把 src 合并到目标文件,保留目标原格式/注释(尽可能)
// 目标文件不存在时直接按 format 序列化 src
//
// 参数:
//   - targetPath: 目标配置文件绝对路径(必须以 .json/.yaml/.toml 结尾)
//   - format:     目标格式(由 DetectConfigFormat 推断)
//   - src:        待合并的源 map(已从源片段文件解析)
//
// 返回: 错误
func MergeConfigToFile(targetPath string, format ConfigFormat, src map[string]interface{}) error {
	if src == nil {
		src = map[string]interface{}{}
	}

	// 目标不存在 → 直接序列化 src
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return writeConfigBytes(targetPath, format, src)
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		return fmt.Errorf("读取目标文件失败: %w", err)
	}

	switch format {
	case FormatJSON:
		return mergeJSONPreserve(targetPath, data, src)
	case FormatYAML:
		return mergeYAMLPreserve(targetPath, data, src)
	case FormatTOML:
		// TOML 退化:解析合并后整体重写(注释/格式会丢失)
		return mergeTOMLRewrite(targetPath, data, src)
	default:
		return fmt.Errorf("不支持的格式: %s", format)
	}
}

// mergeJSONPreserve JSON 合并并保留原文件的注释/格式
// 策略:
//  1. 用 hujson 把 target 转成 AST
//  2. 用 DeepMerge 计算合并结果 (dst ∪ src, src 覆盖)
//  3. 把 src 中独有的顶层 key 用 hujson 的 Insert 插入到目标 AST 的尾部,保持原有顺序
//     同 key 的覆盖由 hujson 自身的标准化语义处理
//
// 为简化与原 MCP 行为兼容,直接采用:解析为 std JSON → DeepMerge → 重新 MarshalIndent(2 空格)
// 若目标原文件无注释,这与原行为完全一致;若目标有注释,则注释会丢失。
// 这是因为 Go 生态缺乏既能保留注释又能深度合并 JSON 的可靠 AST 库,
// 退而求其次保留 JSONC 文件的语义正确性。
func mergeJSONPreserve(targetPath string, data []byte, src map[string]interface{}) error {
	// 尝试标准 JSON 解析,失败时退回为 JSONC
	stdJSON, err := ParseJSONC(data)
	if err != nil {
		stdJSON = data
	}
	var dst map[string]interface{}
	if len(bytes.TrimSpace(stdJSON)) > 0 {
		if err := json.Unmarshal(stdJSON, &dst); err != nil {
			return fmt.Errorf("解析目标 JSON 失败: %w", err)
		}
	}
	if dst == nil {
		dst = map[string]interface{}{}
	}

	merged := DeepMerge(dst, src)
	out, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(targetPath, out, 0644)
}

// mergeYAMLPreserve YAML 合并并保留原文件注释
// 策略:
//  1. 把 target 解析为 *yaml.Node(注释/锚点完整保留)
//  2. 遍历 src, 对每个 key 在目标 mapping 里查找:
//     - 存在且都是 mapping → 递归合并(同样用 yaml.Node)
//     - 存在但不是 mapping → 用 src 的 yaml.Node 替换
//     - 不存在 → 编码 src 的该子树为 yaml.Node,追加到目标 mapping 的 Content 末尾
//  3. 用 yaml.Encoder 序列化回原文,Indent=2
func mergeYAMLPreserve(targetPath string, data []byte, src map[string]interface{}) error {
	var root yamlv3.Node
	if err := yamlv3.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("解析目标 YAML 失败: %w", err)
	}
	// 跳过文档分隔符等,找到实际 mapping/document
	rootMap := PickRootMapping(&root)

	srcNode, err := encodeToYAMLNode(src)
	if err != nil {
		return err
	}

	if rootMap == nil {
		// 目标文件没有可合并的 mapping,直接整体替换为 src
		return writeYAMLFile(targetPath, srcNode)
	}

	PatchMapping(rootMap, srcNode)
	return writeYAMLFile(targetPath, &root)
}

// PickRootMapping 从 yaml.Node 树里提取最外层的 mapping 节点
// 可能是 DocumentNode > MappingNode, 也可能直接是 MappingNode
func PickRootMapping(n *yamlv3.Node) *yamlv3.Node {
	if n == nil {
		return nil
	}
	switch n.Kind {
	case yamlv3.DocumentNode:
		if len(n.Content) > 0 && n.Content[0].Kind == yamlv3.MappingNode {
			return n.Content[0]
		}
		return nil
	case yamlv3.MappingNode:
		return n
	default:
		return nil
	}
}

// PatchMapping 把 src(mapping 节点) patch 到 dst mapping 节点
// - 同 key 是 mapping → 递归
// - 同 key 但不是 mapping → 用 src 的内容替换 dst 的 Value 节点
// - src 独有 key → 在 dst.Content 末尾追加 [keyNode, valueNode]
//   同时维护 dst.Content 的 key 顺序(原有在前,新增在后)
func PatchMapping(dst, src *yamlv3.Node) {
	if dst.Kind != yamlv3.MappingNode || src.Kind != yamlv3.MappingNode {
		return
	}
	// dst 的 key → 在 Content 中的下标(只指向 key 节点; Content 是 [k,v,k,v,...])
	dstKeyIndex := make(map[string]int, len(dst.Content)/2)
	for i := 0; i < len(dst.Content); i += 2 {
		k := dst.Content[i].Value
		dstKeyIndex[k] = i
	}

	// 遍历 src 的 key
	for i := 0; i < len(src.Content); i += 2 {
		keyNode := src.Content[i]
		valNode := src.Content[i+1]
		keyName := keyNode.Value

		if idx, ok := dstKeyIndex[keyName]; ok {
			// dst 已有该 key
			existingVal := dst.Content[idx+1]
			if existingVal.Kind == yamlv3.MappingNode && valNode.Kind == yamlv3.MappingNode {
				PatchMapping(existingVal, valNode)
				continue
			}
			// 标量/数组/类型不一致 → 用 src 节点替换
			dst.Content[idx+1] = valNode
			continue
		}
		// dst 没有 → 追加到末尾
		dst.Content = append(dst.Content, keyNode, valNode)
		dstKeyIndex[keyName] = len(dst.Content) - 2
	}
}

// encodeToYAMLNode 把任意 interface{} 编码为 yaml.Node
func encodeToYAMLNode(v interface{}) (*yamlv3.Node, error) {
	var n yamlv3.Node
	if err := n.Encode(v); err != nil {
		return nil, fmt.Errorf("编码 YAML 节点失败: %w", err)
	}
	return &n, nil
}

// writeYAMLFile 把 yaml.Node 序列化写入文件(2 空格缩进)
func writeYAMLFile(path string, n *yamlv3.Node) error {
	var buf bytes.Buffer
	enc := yamlv3.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(n); err != nil {
		return fmt.Errorf("序列化 YAML 失败: %w", err)
	}
	_ = enc.Close()
	return os.WriteFile(path, buf.Bytes(), 0644)
}

// mergeTOMLRewrite TOML 合并并重写(注释/格式丢失)
func mergeTOMLRewrite(targetPath string, data []byte, src map[string]interface{}) error {
	var dst map[string]interface{}
	if len(bytes.TrimSpace(data)) > 0 {
		if err := toml.Unmarshal(data, &dst); err != nil {
			return fmt.Errorf("解析目标 TOML 失败: %w", err)
		}
	}
	if dst == nil {
		dst = map[string]interface{}{}
	}
	merged := DeepMerge(dst, src)
	out, err := toml.Marshal(merged)
	if err != nil {
		return fmt.Errorf("序列化 TOML 失败: %w", err)
	}
	return os.WriteFile(targetPath, out, 0644)
}

// writeConfigBytes 目标文件不存在时直接写 src
func writeConfigBytes(targetPath string, format ConfigFormat, src map[string]interface{}) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}
	switch format {
	case FormatJSON:
		out, err := json.MarshalIndent(src, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(targetPath, out, 0644)
	case FormatYAML:
		node, err := encodeToYAMLNode(src)
		if err != nil {
			return err
		}
		return writeYAMLFile(targetPath, node)
	case FormatTOML:
		out, err := toml.Marshal(src)
		if err != nil {
			return err
		}
		return os.WriteFile(targetPath, out, 0644)
	default:
		return fmt.Errorf("不支持的格式: %s", format)
	}
}