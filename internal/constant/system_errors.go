package constant

// 系统级错误码 (1xxx)

// 系统级错误码
const (
	CodeSuccess            = 0    // 操作成功，请求已成功处理并返回预期结果
	CodeSystemError        = 1000 // 系统内部错误，服务器遇到意外情况无法完成请求
	CodeDatabaseError      = 1001 // 数据库操作失败，包括连接失败、查询错误、事务异常等
	CodeRedisError         = 1002 // Redis缓存服务错误，包括连接失败、读写超时、内存不足等
	CodeInternalError      = 1003 // 内部服务错误，业务逻辑处理过程中出现的未预期异常
	CodeServiceUnavailable = 1004 // 服务暂时不可用，可能由于维护、升级或过载等原因
	CodeTimeout            = 1005 // 请求处理超时，在规定时间内未完成处理
	CodeRateLimit          = 1006 // 请求频率超过限制，请降低请求频率后重试
	CodeCircuitBreak       = 1007 // 服务熔断器触发，暂时停止服务以防止系统雪崩
)

// 参数错误码
const (
	CodeInvalidParams     = 1100 // 参数格式错误，请求参数不符合预期格式或规范
	CodeMissingParams     = 1101 // 缺少必要参数，请求中缺失必须提供的参数字段
	CodeParamsFormatError = 1102 // 参数格式错误，参数值格式不正确（如日期格式、数字格式等）
	CodeParamsTypeError   = 1103 // 参数类型错误，参数值类型与预期类型不匹配
	CodeParamsRangeError  = 1104 // 参数范围错误，参数值超出允许的范围或界限
	CodeDuplicateRequest  = 1105 // 重复请求检测，相同请求在短时间内被重复提交
)

// 认证授权错误码
const (
	CodeUnauthorized     = 1200 // 未授权访问，请求缺少有效的身份认证信息
	CodeTokenExpired     = 1201 // Token已过期，访问令牌超过有效期限需要重新获取
	CodeTokenInvalid     = 1202 // Token无效，访问令牌格式错误或已被撤销
	CodeSignatureError   = 1203 // 签名验证失败，请求签名与计算签名不匹配
	CodeAccessDenied     = 1204 // 访问权限不足，当前身份没有执行该操作的权限
	CodeIPNotWhitelisted = 1205 // IP不在白名单内，请求来源IP未被授权访问该服务
	CodeMerchantDisabled = 1206 // 商户账号已被禁用，可能由于违规操作或安全原因
)
