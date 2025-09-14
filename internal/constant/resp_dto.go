package constant

// ErrorCode 错误码结构
type ErrorCode struct {
	Code int    `json:"code"` // 错误码数字
	Msg  string `json:"msg"`  // 错误信息
	EN   string `json:"en"`   // 英文错误信息
}

// Response 统一响应格式
type Response struct {
	Code    int         `json:"code"`
	Msg     string      `json:"msg"`
	Data    interface{} `json:"data,omitempty"`
	TraceID string      `json:"trace_id,omitempty"`
}
