package service

import (
	"encoding/json"
	"testing"
)

// mustJSON 解析 JSON 字符串为 map（测试辅助）
func mustJSON(t *testing.T, s string) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("解析 JSON 失败: %v", err)
	}
	return m
}

func TestConfigsConflict(t *testing.T) {
	cases := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{
			name: "深层不同叶子-不冲突",
			a:    `{"aa":{"a1":{"a2":"66"}}}`,
			b:    `{"aa":{"a1":{"a3":"77"}}}`,
			want: false,
		},
		{
			name: "六层嵌套-最内层不同key-不冲突",
			a:    `{"a":{"a1":{"a2":{"a3":{"a4":{"a51":1,"a52":2}}}}}}`,
			b:    `{"a":{"a1":{"a2":{"a3":{"a4":{"a53":1}}}}}}`,
			want: false,
		},
		{
			name: "中间层分叉(a1 vs b1)-不冲突",
			a:    `{"a":{"a1":{"a2":{"a3":{"a4":{"a51":1}}}}}}`,
			b:    `{"a":{"b1":{"a2":{"a3":{"a4":{"a51":1}}}}}}`,
			want: false,
		},
		{
			name: "相同路径同名叶子-冲突",
			a:    `{"a":{"a1":1}}`,
			b:    `{"a":{"a1":2}}`,
			want: true,
		},
		{
			name: "一方叶子一方map相同位置-冲突",
			a:    `{"a":{"a1":1}}`,
			b:    `{"a":{"a1":{"x":1}}}`,
			want: true,
		},
		{
			name: "顶层完全不同-不冲突",
			a:    `{"aa":{"a1":"x"}}`,
			b:    `{"bb":{"b1":"y"}}`,
			want: false,
		},
		{
			name: "mcpServers同名子项-冲突",
			a:    `{"mcpServers":{"github":{"cmd":"a"}}}`,
			b:    `{"mcpServers":{"github":{"cmd":"b"}}}`,
			want: true,
		},
		{
			name: "mcpServers不同子项-不冲突",
			a:    `{"mcpServers":{"github":{"cmd":"a"}}}`,
			b:    `{"mcpServers":{"gitlab":{"cmd":"b"}}}`,
			want: false,
		},
		{
			name: "两侧空-不冲突",
			a:    `{}`,
			b:    `{}`,
			want: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := mustJSON(t, c.a)
			b := mustJSON(t, c.b)
			// 冲突判断应对称
			if got := configsConflict(a, b); got != c.want {
				t.Errorf("configsConflict(a,b)=%v, want %v", got, c.want)
			}
			if got := configsConflict(b, a); got != c.want {
				t.Errorf("configsConflict(b,a)=%v, want %v (不对称)", got, c.want)
			}
		})
	}
}
