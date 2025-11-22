package utils

import (
	"github.com/shopspring/decimal"
	"wht-order-api/internal/dto"
)

// Calculate Mode:
//
//	"agent_from_platform" - 代理从平台收入里拿佣金（平台支付）
//	"agent_from_merchant" - 代理直接从商户扣佣（商户支付）
//
// 本版本增强：支持 payChannelProduct.HasMinFee(int8) & 保底手续费 minFee
func Calculate(
	orderAmount decimal.Decimal,
	merchantPct decimal.Decimal,
	merchantFixed decimal.Decimal,
	agentPct decimal.Decimal,
	agentFixed decimal.Decimal,
	upPct decimal.Decimal,
	upFixed decimal.Decimal,
	mode string,
	currency string,

	// 新增：保底手续费 & 开关（字段来自数据库：has_min_fee）
	minFee decimal.Decimal,
	hasMinFee int8, // ← int8 类型
) dto.SettlementResult {

	// ======= 费率计算 =======
	merchantPctAmt := orderAmount.Mul(merchantPct).Div(decimal.NewFromFloat(100.0))
	agentPctAmt := orderAmount.Mul(agentPct).Div(decimal.NewFromFloat(100.0))
	upPctAmt := orderAmount.Mul(upPct).Div(decimal.NewFromFloat(100.0))

	// ======= 合计 =======
	merchantTotal := merchantPctAmt.Add(merchantFixed)
	agentTotal := agentPctAmt.Add(agentFixed)
	upTotal := upPctAmt.Add(upFixed)

	// ======= 核心新增：保底手续费逻辑 =======
	// hasMinFee == 1 → 启用；其它情况都不启用
	if hasMinFee == 1 {
		if merchantTotal.LessThan(minFee) {
			merchantTotal = minFee
		}
	}

	// ======= 构建基础数据 =======
	res := dto.SettlementResult{
		OrderAmount: orderAmount,

		MerchantFee:      merchantPctAmt,
		MerchantFixed:    merchantFixed,
		MerchantTotalFee: merchantTotal,

		AgentFee:      agentPctAmt,
		AgentFixed:    agentFixed,
		AgentTotalFee: agentTotal,

		UpFeePct:     upPctAmt,
		UpFixed:      upFixed,
		UpTotalFee:   upTotal,
		UpstreamCost: orderAmount.Sub(upTotal),

		Currency: currency,
	}

	// ======= 结算模式 =======
	switch mode {

	case "agent_from_platform":
		// 商户到账 = 订单金额 - 商户手续费
		res.MerchantRecv = MaxDecimal(orderAmount.Sub(merchantTotal), decimal.Zero)

		// 代理佣金
		res.AgentIncome = agentTotal

		// 平台利润 = 商户费用 - 代理佣金 - 上游成本
		res.PlatformProfit = MaxDecimal(merchantTotal.Sub(agentTotal).Sub(upTotal), decimal.Zero)

	case "agent_from_merchant":
		// 商户到账 = 订单金额 - 商户费用 - 代理佣金
		res.MerchantRecv = MaxDecimal(orderAmount.Sub(merchantTotal).Sub(agentTotal), decimal.Zero)

		res.AgentIncome = agentTotal

		// 平台利润 = 商户费用 - 上游成本
		res.PlatformProfit = MaxDecimal(merchantTotal.Sub(upTotal), decimal.Zero)

	default:
		// 默认采用 agent_from_platform
		res.MerchantRecv = MaxDecimal(orderAmount.Sub(merchantTotal), decimal.Zero)
		res.AgentIncome = agentTotal
		res.PlatformProfit = MaxDecimal(merchantTotal.Sub(agentTotal).Sub(upTotal), decimal.Zero)
	}

	return res
}

// MaxDecimal 返回最大值（防止负数）
func MaxDecimal(a, b decimal.Decimal) decimal.Decimal {
	if a.Cmp(b) >= 0 {
		return a
	}
	return b
}
