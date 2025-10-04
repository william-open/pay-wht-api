package dto

type BankCodeMappingDto struct {
	ID               int    `json:"id"`               // 主键
	InterfaceId      int    `json:"interfaceId"`      // 接口ID
	Status           int8   `json:"status"`           // 状态: 0=停用, 1=正常
	Currency         string `json:"currency"`         // 货币
	InternalBankCode string `json:"internalBankCode"` // 平台银行编码
	UpstreamBankCode string `json:"upstreamBankCode"` // 上游银行编码
	UpstreamBankName string `json:"upstreamBankName"` // 上游银行名称
}

type BankCodeDto struct {
	ID       int    `json:"id"`       // 主键
	Status   int8   `json:"status"`   // 状态: 0=停用, 1=正常
	Currency string `json:"currency"` // 货币
	Code     string `json:"code"`     // 银行编码
	Name     string `json:"name"`     // 银行名称
}
