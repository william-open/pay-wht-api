package utils

import "wht-order-api/internal/constant"

// 统一响应格式
type Response struct {
	Code    int         `json:"code"`
	Msg     string      `json:"msg"`
	Data    interface{} `json:"data,omitempty"`
	TraceID string      `json:"trace_id,omitempty"`
}

// 成功响应
func Success(data interface{}) Response {
	return Response{
		Code: constant.CodeSuccess,
		Msg:  "成功",
		Data: data,
	}
}

// 错误响应
func Error(code int) Response {
	if info, exists := constant.GetErrorInfo(code); exists {
		return Response{
			Code: code,
			Msg:  info.CN,
		}
	}
	return Response{
		Code: code,
		Msg:  "未知错误",
	}
}

// 带数据的错误响应
func ErrorWithData(code int, data interface{}) Response {
	if info, exists := constant.GetErrorInfo(code); exists {
		return Response{
			Code: code,
			Msg:  info.CN,
			Data: data,
		}
	}
	return Response{
		Code: code,
		Msg:  "未知错误",
		Data: data,
	}
}

// ErrorWithTrace 错误响应（带TraceID，默认中文）
func ErrorWithTrace(code int, traceID string) Response {
	info, _ := constant.GetErrorInfo(code)
	return Response{
		Code:    code,
		Msg:     info.CN,
		TraceID: traceID,
	}
}

// CustomError 自定义错误响应
func CustomError(code int, message string) Response {
	return Response{
		Code: code,
		Msg:  message,
	}
}

// CustomErrorWithTrace 自定义错误响应（带TraceID）
func CustomErrorWithTrace(code int, message string, traceID string) Response {
	return Response{
		Code:    code,
		Msg:     message,
		TraceID: traceID,
	}
}
