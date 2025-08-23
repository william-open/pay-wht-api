package utils

import (
	"github.com/shopspring/decimal"
	"wht-order-api/internal/dto"
)

// Mode:
//
//	"agent_from_platform" - 代理从平台收入里拿佣金（平台支付）
//	"agent_from_merchant" - 代理直接从商户结算扣款（商户支付）
func Calculate(orderAmount, merchantPct, merchantFixed, agentPct, agentFixed, upPct, upFixed decimal.Decimal, mode string, currency string) dto.SettlementResult {
	merchantPctAmt := orderAmount.Mul(merchantPct).Div(decimal.NewFromFloat(100.0))
	agentPctAmt := orderAmount.Mul(agentPct).Div(decimal.NewFromFloat(100.0))
	upPctAmt := orderAmount.Mul(upPct).Div(decimal.NewFromFloat(100.0))

	merchantTotal := merchantPctAmt.Add(merchantFixed)
	agentTotal := agentPctAmt.Add(agentFixed)
	upTotal := upPctAmt.Add(upFixed)

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
		UpstreamCost:     orderAmount.Sub(upTotal), // 上游到账（或者上游实际保留对应）
		Currency:         currency,
	}

	switch mode {
	case "agent_from_platform":
		// 商户收到 order - merchantTotal
		res.MerchantRecv = MaxDecimal(orderAmount.Sub(merchantTotal), decimal.Zero)
		res.AgentIncome = agentTotal
		// 平台净利 = merchantTotal - agentTotal - upTotal
		res.PlatformProfit = MaxDecimal(merchantTotal.Sub(agentTotal).Sub(upTotal), decimal.Zero)
	case "agent_from_merchant":
		// 商户收到 order - merchantTotal - agentTotal
		res.MerchantRecv = MaxDecimal(orderAmount.Sub(merchantTotal).Sub(agentTotal), decimal.Zero)
		res.AgentIncome = agentTotal
		// 平台净利 = merchantTotal - upTotal
		res.PlatformProfit = MaxDecimal(merchantTotal.Sub(upTotal), decimal.Zero)
	default:
		// 默认采用 agent_from_platform
		res.MerchantRecv = MaxDecimal(orderAmount.Sub(merchantTotal), decimal.Zero)
		res.AgentIncome = agentTotal
		res.PlatformProfit = MaxDecimal(merchantTotal.Sub(agentTotal).Sub(upTotal), decimal.Zero)
	}

	return res
}

// 比较两个数值，返回最大值
func MaxDecimal(a, b decimal.Decimal) decimal.Decimal {

	if a.Cmp(b) >= 0 {
		return a
	}
	return b
}
