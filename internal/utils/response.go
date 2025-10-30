package utils

import "wht-order-api/internal/constant"

// 统一响应格式（支持中英文提示）
type Response struct {
	Code    int         `json:"code"`
	Msg     string      `json:"msg"`              // 中文描述
	MsgEN   string      `json:"msg_en,omitempty"` // 英文描述
	Data    interface{} `json:"data,omitempty"`
	TraceID string      `json:"trace_id,omitempty"`
}

// 成功响应
func Success(data interface{}) Response {
	return Response{
		Code:  constant.CodeSuccess,
		Msg:   "成功",
		MsgEN: "Success",
		Data:  data,
	}
}

// 错误响应（自动从 constant 中获取中英文描述）
func Error(code int) Response {
	if info, exists := constant.GetErrorInfo(code); exists {
		return Response{
			Code:  code,
			Msg:   info.CN,
			MsgEN: info.EN,
		}
	}
	return Response{
		Code:  code,
		Msg:   "未知错误",
		MsgEN: "Unknown error",
	}
}

// 带数据的错误响应
func ErrorWithData(code int, data interface{}) Response {
	if info, exists := constant.GetErrorInfo(code); exists {
		return Response{
			Code:  code,
			Msg:   info.CN,
			MsgEN: info.EN,
			Data:  data,
		}
	}
	return Response{
		Code:  code,
		Msg:   "未知错误",
		MsgEN: "Unknown error",
		Data:  data,
	}
}

// 错误响应（带 TraceID）
func ErrorWithTrace(code int, traceID string) Response {
	info, _ := constant.GetErrorInfo(code)
	return Response{
		Code:    code,
		Msg:     info.CN,
		MsgEN:   info.EN,
		TraceID: traceID,
	}
}

// 自定义错误响应（仅中文）
func CustomError(code int, message string) Response {
	return Response{
		Code:  code,
		Msg:   message,
		MsgEN: "Custom error",
	}
}

// 自定义错误响应（带 TraceID）
func CustomErrorWithTrace(code int, message string, traceID string) Response {
	return Response{
		Code:    code,
		Msg:     message,
		MsgEN:   "Custom error",
		TraceID: traceID,
	}
}
