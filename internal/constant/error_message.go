package constant

// ErrorInfo 错误信息结构
type ErrorInfo struct {
	CN string `json:"cn"` // 中文错误信息
	EN string `json:"en"` // 英文错误信息
}

// ErrorMessages 错误信息映射
var ErrorMessages = map[int]ErrorInfo{
	// 系统错误
	CodeSuccess:       {"操作成功", "Success"},
	CodeSystemError:   {"系统错误", "System error"},
	CodeDatabaseError: {"数据库错误", "Database error"},

	// 上游错误
	CodeUpstreamError: {"上游通道错误", "Upstream channel error"},

	// 银行错误
	CodeBankRejected: {"银行拒绝交易", "Bank rejected"},
	// 商户相关错误
	CodeMerchantNotFound:     {"商户不存在", "Merchant not found"},
	CodeMerchantBalanceLow:   {"商户余额不足", "Merchant balance insufficient"},
	CodeMerchantRateInvalid:  {"商户费率配置无效", "Merchant rate invalid"},
	CodeMerchantLimitReached: {"商户交易限额已满", "Merchant limit reached"},
	CodeMerchantKeyInvalid:   {"商户密钥无效", "Merchant key invalid"},

	// 订单相关错误
	CodeOrderNotFound:      {"订单不存在", "Order not found"},
	CodeOrderAlreadyExist:  {"订单已存在", "Order already exists"},
	CodeOrderStatusInvalid: {"订单状态无效", "Order status invalid"},
	CodeOrderAmountInvalid: {"订单金额无效", "Order amount invalid"},
	CodeOrderExpired:       {"订单已过期", "Order expired"},
	CodeOrderPaid:          {"订单已支付", "Order already paid"},
	CodeOrderRefunded:      {"订单已退款", "Order already refunded"},
	CodeOrderClosed:        {"订单已关闭", "Order closed"},

	// 支付通道相关错误
	CodeChannelNotFound:     {"支付通道不存在", "Channel not found"},
	CodeChannelDisabled:     {"支付通道已禁用", "Channel disabled"},
	CodeChannelBusy:         {"支付通道繁忙", "Channel busy"},
	CodeChannelMaintenance:  {"支付通道维护中", "Channel under maintenance"},
	CodeChannelRateInvalid:  {"通道费率配置错误", "Channel rate invalid"},
	CodeChannelLimitReached: {"通道交易限额已满", "Channel limit reached"},
	CodeChannelUnavailable:  {"支付通道暂时不可用", "Channel unavailable"},

	// 支付相关错误
	CodePaymentFailed:        {"支付失败", "Payment failed"},
	CodePaymentProcessing:    {"支付处理中", "Payment processing"},
	CodePaymentTimeout:       {"支付超时", "Payment timeout"},
	CodePaymentAmountError:   {"支付金额错误", "Payment amount error"},
	CodePaymentCurrencyError: {"支付币种错误", "Payment currency error"},
	CodePaymentMethodError:   {"支付方式错误", "Payment method error"},

	// 风控相关错误
	CodeRiskRejected:      {"风控系统拒绝交易", "Risk control rejected"},
	CodeRiskSuspicious:    {"交易行为可疑", "Suspicious transaction"},
	CodeRiskBlacklist:     {"账户存在风险", "Blacklisted user"},
	CodeRiskHighFrequency: {"交易频率过高", "High frequency transaction"},
	CodeRiskAmountLimit:   {"交易金额超过限制", "Amount limit exceeded"},
	CodeRiskGeoBlocked:    {"地区暂不支持", "Geographic restriction"},
}
