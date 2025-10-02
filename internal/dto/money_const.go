package dto

const (
	// 代收/代付
	MoneyLogTypeDeposit = 1 // 代收入账（加钱）
	MoneyLogTypePayout  = 2 // 代付出账（成功时减冻结）

	// 代理/平台收益
	MoneyLogTypeDepositComm = 11 // 代收收益
	MoneyLogTypePayoutComm  = 21 // 代付收益

	// 冻结/解冻资金
	MoneyLogTypeFreeze      = 60 // 冻结资金（下单时）
	MoneyLogTypeUnfreeze    = 61 // 解冻资金（失败退回）
	MoneyLogTypeUnfreezeDel = 62 // 删除冻结（失败扣掉冻结资金）

)
