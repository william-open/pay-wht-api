package utils

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FlexibleMsg 支持 string / object / array 任意结构的 msg
type FlexibleMsg struct {
	Text string
}

func (m *FlexibleMsg) UnmarshalJSON(data []byte) error {
	// 尝试解析为字符串
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		m.Text = s
		return nil
	}

	// 尝试解析为 map 对象
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err == nil {
		parts := make([]string, 0)
		for k, v := range obj {
			switch val := v.(type) {
			case string:
				parts = append(parts, fmt.Sprintf("%s: %s", k, val))
			case float64, int:
				parts = append(parts, fmt.Sprintf("%s: %v", k, val))
			default:
				b, _ := json.Marshal(val)
				parts = append(parts, fmt.Sprintf("%s: %s", k, string(b)))
			}
		}
		m.Text = strings.Join(parts, "; ")
		return nil
	}

	// 尝试解析为数组
	var arr []interface{}
	if err := json.Unmarshal(data, &arr); err == nil {
		b, _ := json.Marshal(arr)
		m.Text = string(b)
		return nil
	}

	// 全部失败
	m.Text = string(data)
	return nil
}
