package dto

// SettlementResult 结算数据
type SettlementResult struct {
	OrderAmount      float64 `json:"orderAmount"`
	MerchantFee      float64 `json:"merchantFee"`
	MerchantFixed    float64 `json:"merchantFixed"`
	MerchantTotalFee float64 `json:"merchantTotalFee"`
	AgentFee         float64 `json:"agentFee"`
	AgentFixed       float64 `json:"agentFixed"`
	AgentTotalFee    float64 `json:"agentTotalFee"`
	UpFeePct         float64 `json:"upFeePct"`
	UpFixed          float64 `json:"upFixed"`
	UpTotalFee       float64 `json:"upTotalFee"`
	Currency         string  `json:"currency"` // 货币符号

	// outputs
	MerchantRecv   float64 `json:"merchantRecv"`   // 商户最终到账（取决于模式）
	UpstreamCost   float64 `json:"upstreamCost"`   // 上游成本/上游实际保留
	AgentIncome    float64 `json:"agentIncome"`    // 代理收入
	PlatformProfit float64 `json:"platformProfit"` // 平台净利润

}
