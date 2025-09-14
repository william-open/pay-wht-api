package constant

import "fmt"

// Error 错误接口
type Error interface {
	error
	Code() int
	Message() string
	WithData(data interface{}) Error
}

// CustomError 自定义错误实现
type CustomError struct {
	code    int
	message string
	data    interface{}
}

func (e *CustomError) Error() string {
	return fmt.Sprintf("code: %d, message: %s", e.code, e.message)
}

func (e *CustomError) Code() int {
	return e.code
}

func (e *CustomError) Message() string {
	return e.message
}

func (e *CustomError) WithData(data interface{}) Error {
	e.data = data
	return e
}

// NewError 创建错误
func NewError(code int) Error {
	if info, exists := ErrorMessages[code]; exists {
		return &CustomError{code: code, message: info.CN}
	}
	return &CustomError{code: code, message: "未知错误"}
}

// GetErrorInfo 获取错误信息
func GetErrorInfo(code int) (ErrorInfo, bool) {
	info, exists := ErrorMessages[code]
	return info, exists
}
