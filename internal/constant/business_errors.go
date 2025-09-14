package constant

// 业务级错误码 (2xxx)

// 商户相关错误码
const (
	CodeMerchantNotFound     = 2000 // 商户不存在或未找到，请检查商户编号是否正确
	CodeMerchantBalanceLow   = 2001 // 商户余额不足，请充值后再进行交易
	CodeMerchantRateInvalid  = 2002 // 商户费率配置无效，请联系管理员检查费率设置
	CodeMerchantLimitReached = 2003 // 商户交易限额已满，请明日再试或联系客服调整限额
	CodeMerchantKeyInvalid   = 2004 // 商户密钥无效或已过期，请重新生成API密钥
)

// 订单相关错误码
const (
	CodeOrderNotFound      = 2100 // 订单不存在，请检查订单号是否正确
	CodeOrderAlreadyExist  = 2101 // 订单已存在，请勿重复创建订单
	CodeOrderStatusInvalid = 2102 // 订单状态无效，无法进行当前操作
	CodeOrderAmountInvalid = 2103 // 订单金额无效，请检查金额格式和范围
	CodeOrderExpired       = 2104 // 订单已过期，请重新创建订单
	CodeOrderPaid          = 2105 // 订单已支付，请勿重复支付
	CodeOrderRefunded      = 2106 // 订单已退款，无法再进行支付操作
	CodeOrderClosed        = 2107 // 订单已关闭，无法进行任何操作
)

// 支付通道相关错误码
const (
	CodeChannelNotFound     = 2200 // 支付通道不存在，请检查通道编码是否正确
	CodeChannelDisabled     = 2201 // 支付通道已禁用，暂时无法使用该通道
	CodeChannelBusy         = 2202 // 支付通道繁忙，请稍后重试或选择其他通道
	CodeChannelMaintenance  = 2203 // 支付通道维护中，预计恢复时间请关注公告
	CodeChannelRateInvalid  = 2204 // 通道费率配置错误，请联系技术支持
	CodeChannelLimitReached = 2205 // 通道交易限额已满，请选择其他支付通道
	CodeChannelUnavailable  = 2206 // 支付通道暂时不可用，请稍后重试
)

// 支付相关错误码
const (
	CodePaymentFailed        = 2300 // 支付失败，请检查支付信息后重试
	CodePaymentProcessing    = 2301 // 支付处理中，请勿重复提交
	CodePaymentTimeout       = 2302 // 支付超时，请检查网络后重试
	CodePaymentAmountError   = 2303 // 支付金额错误，请核对金额是否正确
	CodePaymentCurrencyError = 2304 // 支付币种错误，请选择正确的币种
	CodePaymentMethodError   = 2305 // 支付方式错误，请选择支持的支付方式
)

// 风控相关错误码
const (
	CodeRiskRejected      = 2400 // 风控系统拒绝交易，请联系客服
	CodeRiskSuspicious    = 2401 // 交易行为可疑，请完成身份验证
	CodeRiskBlacklist     = 2402 // 您的账户存在风险，暂时无法交易
	CodeRiskHighFrequency = 2403 // 交易频率过高，请稍后重试
	CodeRiskAmountLimit   = 2404 // 交易金额超过限制，请调整金额
	CodeRiskGeoBlocked    = 2405 // 您所在的地区暂不支持该交易
)

// 结算相关错误码
const (
	CodeSettlementFailed     = 2500 // 结算失败，请检查结算信息
	CodeSettlementProcessing = 2501 // 结算处理中，请勿重复提交
	CodeSettlementBalanceLow = 2502 // 结算余额不足，请先充值
	CodeSettlementLimit      = 2503 // 结算金额超过单笔限额
	CodeSettlementTimeLimit  = 2504 // 不在结算时间范围内
	CodeSettlementBankError  = 2505 // 银行结算系统异常
)

// 退款相关错误码
const (
	CodeRefundFailed       = 2600 // 退款失败，请检查退款信息
	CodeRefundProcessing   = 2601 // 退款处理中，请勿重复提交
	CodeRefundAmountError  = 2602 // 退款金额超过可退金额
	CodeRefundTimeLimit    = 2603 // 已超过退款时间限制
	CodeRefundOrderInvalid = 2604 // 订单状态不支持退款
	CodeRefundChannelError = 2605 // 退款通道异常，请稍后重试
)

// 通知相关错误码
const (
	CodeNotifyFailed      = 2700 // 通知发送失败，请检查通知配置
	CodeNotifyTimeout     = 2701 // 通知超时，请检查网络连接
	CodeNotifySignError   = 2702 // 通知签名验证失败
	CodeNotifyFormatError = 2703 // 通知格式错误，请检查数据格式
	CodeNotifyRepeat      = 2704 // 重复通知，已处理过该通知
)

// 对账相关错误码
const (
	CodeReconFileError    = 2800 // 对账文件格式错误或无法解析
	CodeReconDataMismatch = 2801 // 对账数据不一致，请人工核对
	CodeReconDownloadFail = 2802 // 对账文件下载失败
	CodeReconProcessFail  = 2803 // 对账处理失败，请重试
	CodeReconNoData       = 2804 // 暂无对账数据
)

// 配置相关错误码
const (
	CodeConfigNotFound   = 2900 // 配置信息不存在
	CodeConfigInvalid    = 2901 // 配置信息无效或格式错误
	CodeConfigUpdateFail = 2902 // 配置更新失败
	CodeConfigReadOnly   = 2903 // 配置为只读，无法修改
)

// 汇率相关错误码
const (
	CodeRateNotFound   = 2910 // 汇率信息不存在
	CodeRateExpired    = 2911 // 汇率已过期，请更新汇率
	CodeRateInvalid    = 2912 // 汇率无效或格式错误
	CodeRateUpdateFail = 2913 // 汇率更新失败
)

// 账户相关错误码
const (
	CodeAccountNotFound         = 2920 // 账户不存在
	CodeAccountFrozen           = 2921 // 账户已冻结，无法操作
	CodeAccountBalanceLow       = 2922 // 账户余额不足
	CodeAccountPasswordError    = 2923 // 账户密码错误
	CodeAccountPermissionDenied = 2924 // 账户权限不足
)
