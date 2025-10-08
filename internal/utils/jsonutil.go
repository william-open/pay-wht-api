package utils

import (
	"encoding/json"
	"strings"
)

// StringOrNumber 支持 JSON 中字段为 string 或 number 的场景
// 常用于上游响应中 code、status 等字段兼容解析。
type StringOrNumber string

// UnmarshalJSON 支持自动兼容 string 或 number
func (s *StringOrNumber) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		*s = ""
		return nil
	}

	// 判断首字符是否为引号 => 说明是字符串
	if b[0] == '"' {
		var str string
		if err := json.Unmarshal(b, &str); err != nil {
			return err
		}
		*s = StringOrNumber(str)
		return nil
	}

	// 否则为数字 => 直接转成字符串
	*s = StringOrNumber(strings.TrimSpace(string(b)))
	return nil
}
