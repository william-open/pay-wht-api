package utils

import (
	"fmt"
	"wht-order-api/internal/dto"
)

// Mode:
//
//	"agent_from_platform" - 代理从平台收入里拿佣金（平台支付）
//	"agent_from_merchant" - 代理直接从商户结算扣款（商户支付）
func Calculate(orderAmount, merchantPct, merchantFixed, agentPct, agentFixed, upPct, upFixed float64, mode string) dto.SettlementResult {
	merchantPctAmt := orderAmount * merchantPct / 100.0
	agentPctAmt := orderAmount * agentPct / 100.0
	upPctAmt := orderAmount * upPct / 100.0

	merchantTotal := merchantPctAmt + merchantFixed
	agentTotal := agentPctAmt + agentFixed
	upTotal := upPctAmt + upFixed

	res := dto.SettlementResult{
		OrderAmount:      orderAmount,
		MerchantFee:      merchantPctAmt,
		MerchantFixed:    merchantFixed,
		MerchantTotalFee: merchantTotal,
		AgentFee:         agentPctAmt,
		AgentFixed:       agentFixed,
		AgentTotalFee:    agentTotal,
		UpFeePct:         upPctAmt,
		UpFixed:          upFixed,
		UpTotalFee:       upTotal,
		UpstreamCost:     orderAmount - upTotal, // 上游到账（或者上游实际保留对应）
	}

	switch mode {
	case "agent_from_platform":
		// 商户收到 order - merchantTotal
		res.MerchantRecv = orderAmount - merchantTotal
		res.AgentIncome = agentTotal
		// 平台净利 = merchantTotal - agentTotal - upTotal
		res.PlatformProfit = merchantTotal - agentTotal - upTotal
	case "agent_from_merchant":
		// 商户收到 order - merchantTotal - agentTotal
		res.MerchantRecv = orderAmount - merchantTotal - agentTotal
		res.AgentIncome = agentTotal
		// 平台净利 = merchantTotal - upTotal
		res.PlatformProfit = merchantTotal - upTotal
	default:
		// 默认采用 agent_from_platform
		res.MerchantRecv = orderAmount - merchantTotal
		res.AgentIncome = agentTotal
		res.PlatformProfit = merchantTotal - agentTotal - upTotal
	}

	return res
}

// quick demo
func Demo() {
	order := 100.0
	r1 := Calculate(order, 4.0, 5.0, 1.0, 1.0, 2.0, 2.0, "agent_from_platform")
	fmt.Printf("MODE A (agent from platform): %+v\n", r1)

	r2 := Calculate(order, 4.0, 5.0, 1.0, 1.0, 2.0, 2.0, "agent_from_merchant")
	fmt.Printf("MODE B (agent from merchant): %+v\n", r2)
}
