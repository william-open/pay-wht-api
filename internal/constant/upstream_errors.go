package constant

// 上游通道错误码 (3xxx) - 用于表示与上游支付通道相关的错误
const (
	// CodeUpstreamError 上游通道通用错误
	// 适用场景：上游通道返回未知错误、系统内部异常等
	// 示例：上游通道系统繁忙、内部处理失败
	CodeUpstreamError = 3000

	// CodeUpstreamTimeout 上游通道请求超时
	// 适用场景：调用上游通道接口超时未响应
	// 示例：HTTP请求超时、上游通道响应超时
	CodeUpstreamTimeout = 3001

	// CodeUpstreamRejected 上游通道拒绝交易
	// 适用场景：上游通道明确拒绝处理该笔交易
	// 示例：金额超限、商户号无效、通道暂停服务
	CodeUpstreamRejected = 3002

	// CodeUpstreamBalanceInsufficient 上游通道余额不足
	// 适用场景：上游通道账户余额不足以完成交易
	// 示例：通道商户号余额不足、额度已用完
	CodeUpstreamBalanceInsufficient = 3003

	// CodeUpstreamInvalidAccount 上游通道账户异常
	// 适用场景：上游通道账户状态异常或信息错误
	// 示例：账户被冻结、API密钥无效、商户号不存在
	CodeUpstreamInvalidAccount = 3004

	// CodeUpstreamNetworkError 上游通道网络异常
	// 适用场景：与上游通道网络连接异常
	// 示例：DNS解析失败、连接被拒绝、SSL证书错误
	CodeUpstreamNetworkError = 3005

	// CodeUpstreamDataFormatError 上游通道数据格式错误
	// 适用场景：请求数据格式不符合上游通道要求
	// 示例：字段缺失、数据类型错误、编码格式不支持
	CodeUpstreamDataFormatError = 3006

	// CodeUpstreamSignError 上游通道签名错误
	// 适用场景：签名验证失败或生成错误
	// 示例：签名算法不一致、密钥不匹配、签名过期
	CodeUpstreamSignError = 3007

	// CodeUpstreamRateLimit 上游通道频率限制
	// 适用场景：请求频率超过上游通道限制
	// 示例：API调用频率超限、同一订单重复提交
	CodeUpstreamRateLimit = 3008

	// CodeUpstreamMaintenance 上游通道维护中
	// 适用场景：上游通道系统维护或升级
	// 示例：通道临时维护、系统升级暂停服务
	CodeUpstreamMaintenance = 3009

	// CodeUpstreamRiskControl 上游通道风控拦截
	// 适用场景：交易被上游风控系统拦截
	// 示例：可疑交易、IP地址异常、交易模式风险
	CodeUpstreamRiskControl = 3010

	// CodeUpstreamCurrencyUnsupported 上游通道不支持该币种
	// 适用场景：请求的币种上游通道不支持
	// 示例：请求USD但通道只支持CNY
	CodeUpstreamCurrencyUnsupported = 3011

	// CodeUpstreamAmountLimit 上游通道金额限制
	// 适用场景：交易金额超出上游通道限制范围
	// 示例：单笔金额超限、单日累计金额超限
	CodeUpstreamAmountLimit = 3012

	// CodeUpstreamBankProcessing 上游通道银行处理中
	// 适用场景：交易已提交银行处理，结果未知
	// 示例：银行系统处理延迟、需要人工审核
	CodeUpstreamBankProcessing = 3013

	// CodeUpstreamDuplicateOrder 上游通道订单重复
	// 适用场景：上游通道检测到重复订单号
	// 示例：同一商户订单号重复提交
	CodeUpstreamDuplicateOrder = 3014

	// CodeUpstreamChannelClosed 上游通道已关闭
	// 适用场景：该支付通道已停止服务
	// 示例：通道合作终止、通道已下线
	CodeUpstreamChannelClosed = 3015
)
