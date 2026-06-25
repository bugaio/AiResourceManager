// Package util merge.go 提供类似 lodash.merge 的 JSON 对象深度合并
package util

// DeepMerge 将 src 深度合并到 dst，返回合并后的结果（类似 lodash.merge）
// 规则:
//   - 两边同 key 且都是对象(map) → 递归深度合并
//   - 两边同 key 且都是数组(slice) → 拼接(dst 元素在前, src 元素在后)
//   - 其余情况(标量、类型不一致) → src 的值覆盖 dst
//   - dst 中有而 src 中没有的 key → 保留
//
// 参数 dst: 目标对象（被合并方，通常是目标文件已有内容）
// 参数 src: 源对象（合并方，通常是本次要部署的配置）
// 返回: 合并后的新 map（不修改入参）
func DeepMerge(dst, src map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(dst))
	for k, v := range dst {
		out[k] = v
	}
	for k, sv := range src {
		if dv, ok := out[k]; ok {
			dvMap, dOk := dv.(map[string]interface{})
			svMap, sOk := sv.(map[string]interface{})
			if dOk && sOk {
				out[k] = DeepMerge(dvMap, svMap)
				continue
			}
			dvArr, dArrOk := dv.([]interface{})
			svArr, sArrOk := sv.([]interface{})
			if dArrOk && sArrOk {
				merged := make([]interface{}, 0, len(dvArr)+len(svArr))
				merged = append(merged, dvArr...)
				merged = append(merged, svArr...)
				out[k] = merged
				continue
			}
		}
		out[k] = sv
	}
	return out
}
