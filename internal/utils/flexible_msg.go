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

// UnmarshalJSON implements the json.Unmarshaler interface.
// It supports decoding string, object and array structures into a readable string.
func (m *FlexibleMsg) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		m.Text = s
		return nil
	}

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

	var arr []interface{}
	if err := json.Unmarshal(data, &arr); err == nil {
		b, _ := json.Marshal(arr)
		m.Text = string(b)
		return nil
	}

	m.Text = string(data)
	return nil
}

// MarshalJSON implements the json.Marshaller interface.
// It ensures FlexibleMsg is encoded as a plain string instead of {"Text":"..."}.
func (m *FlexibleMsg) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.Text)
}

// String returns the inner text of FlexibleMsg.
func (m *FlexibleMsg) String() string {
	return m.Text
}
