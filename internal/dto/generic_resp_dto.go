package dto

import "encoding/json"

// GenericStructResp  通用响应结构 返回数据
type GenericStructResp struct {
	Code string          `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"` // 延迟解析
}

// ParseData 智能解析器：可以解析成任意结构体、map、string
func (r *GenericStructResp) ParseData(v any) error {
	if len(r.Data) == 0 {
		return nil
	}
	return json.Unmarshal(r.Data, v)
}

// GenericResp 泛型封装（类型安全）
type GenericResp[T any] struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data"`
}

// UnmarshalGeneric 工具函数：从 json 解析到泛型响应
func UnmarshalGeneric[T any](body []byte) (*GenericResp[T], error) {
	var resp GenericResp[T]
	err := json.Unmarshal(body, &resp)
	return &resp, err
}
