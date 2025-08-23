package dto

import "github.com/shopspring/decimal"

// SettlementResult 结算数据
type SettlementResult struct {
	OrderAmount      decimal.Decimal `json:"orderAmount"`
	MerchantFee      decimal.Decimal `json:"merchantFee"`
	MerchantFixed    decimal.Decimal `json:"merchantFixed"`
	MerchantTotalFee decimal.Decimal `json:"merchantTotalFee"`
	AgentFee         decimal.Decimal `json:"agentFee"`
	AgentFixed       decimal.Decimal `json:"agentFixed"`
	AgentTotalFee    decimal.Decimal `json:"agentTotalFee"`
	UpFeePct         decimal.Decimal `json:"upFeePct"`
	UpFixed          decimal.Decimal `json:"upFixed"`
	UpTotalFee       decimal.Decimal `json:"upTotalFee"`
	Currency         string          `json:"currency"` // 货币符号

	// outputs
	MerchantRecv   decimal.Decimal `json:"merchantRecv"`   // 商户最终到账（取决于模式）
	UpstreamCost   decimal.Decimal `json:"upstreamCost"`   // 上游成本/上游实际保留
	AgentIncome    decimal.Decimal `json:"agentIncome"`    // 代理收入
	PlatformProfit decimal.Decimal `json:"platformProfit"` // 平台净利润

}
