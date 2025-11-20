package settlement

import (
	"fmt"
	"github.com/shopspring/decimal"
	"log"
	"strconv"
	"wht-order-api/internal/dao"
	"wht-order-api/internal/dto"
)

type Settlement struct {
	mainDao  *dao.MainDao
	orderDao *dao.OrderDao
}

func NewSettlement() *Settlement {
	return &Settlement{
		mainDao:  dao.NewMainDao(),
		orderDao: dao.NewOrderDao(),
	}
}

// DoPaySettlement 处理代收订单结算逻辑
func (s *Settlement) DoPaySettlement(req dto.SettlementResult, mId string, orderId uint64, mOrderId string) error {
	orderNo := strconv.FormatUint(orderId, 10)

	log.Printf("[SETTLEMENT] 开始结算: 商户=%v, 订单号=%v, 数据=%+v", mId, orderNo, req)

	// 1) 校验商户合法性
	merchant, err := s.mainDao.GetMerchantId(mId)
	if err != nil {
		return fmt.Errorf("[SETTLEMENT] 获取商户失败: %w", err)
	}
	if merchant == nil || merchant.Status != 1 {
		return fmt.Errorf("[SETTLEMENT] 商户无效, merchantID=%v", mId)
	}

	// 2) 商户资金日志 & 账户更新
	moneyLog := dto.MoneyLog{
		UID:         merchant.MerchantID,
		Money:       req.MerchantRecv,
		OrderNo:     orderNo,
		MOrderNo:    mOrderId,
		Type:        dto.MoneyLogTypeDeposit, // ✅ 常量定义
		Operator:    merchant.NickName,
		Description: "商户代收",
		Currency:    req.Currency,
		CreateBy:    merchant.NickName,
	}

	if err := s.mainDao.CreateMoneyLog(moneyLog); err != nil {
		return fmt.Errorf("[SETTLEMENT] 商户资金结算失败, merchantID=%v, orderNo=%v, err=%w", merchant.MerchantID, orderNo, err)
	}

	// 3) 代理收益 & 账户更新
	agentMoney := dto.AgentMoney{
		AID:        merchant.PId,
		MID:        merchant.MerchantID,
		OrderNo:    orderNo,
		MOrderNo:   mOrderId,
		OrderMoney: req.OrderAmount,
		Money:      req.AgentIncome,
		Currency:   req.Currency,
		Type:       dto.MoneyLogTypeDeposit, // ✅ 常量定义
		Remark:     "商户代收收益",
	}

	if err := s.mainDao.CreateAgentMoneyLog(agentMoney, dto.MoneyLogTypeDepositComm, "代理代收佣金"); err != nil {
		return fmt.Errorf("[SETTLEMENT] 代理资金结算失败, agentID=%v, orderNo=%v, err=%w", merchant.PId, orderNo, err)
	}

	log.Printf("[SETTLEMENT] 结算完成: 商户=%v, 代理=%v, 订单号=%v", merchant.MerchantID, merchant.PId, orderNo)
	return nil
}

// DoPayoutSettlement 处理代付订单结算逻辑
// status = true 表示代付成功，false 表示代付失败
func (s *Settlement) DoPayoutSettlement(req dto.SettlementResult, mId string, orderId uint64, mOrderNo string, status bool, orderAmount decimal.Decimal) error {
	orderNo := strconv.FormatUint(orderId, 10)

	log.Printf("[SETTLEMENT] 开始代付结算: 商户=%v, 订单号=%v, 商户费用=%v,代理佣金=%v,货币=%s, 状态=%v, 数据=%+v",
		mId, orderNo, req.MerchantTotalFee, req.AgentTotalFee, req.Currency, status, req)

	// 1) 校验商户合法性
	merchant, err := s.mainDao.GetMerchantId(mId)
	if err != nil {
		return fmt.Errorf("[SETTLEMENT] 获取商户失败: merchantID=%v, err=%w", mId, err)
	}
	if merchant == nil || merchant.Status != 1 {
		return fmt.Errorf("[SETTLEMENT] 商户无效, merchantID=%v", mId)
	}

	// 2) 商户资金日志 & 账户更新
	if handleErr := s.mainDao.HandlePayoutCallback(
		merchant.MerchantID,
		req.Currency,
		orderNo,
		mOrderNo,
		req.MerchantTotalFee,
		req.AgentTotalFee,
		status,
		orderAmount,
		merchant.NickName,
	); handleErr != nil {
		return fmt.Errorf("[SETTLEMENT][payout] 商户资金结算失败\n 商户ID:%v\n平台订单号:%v\n商户手续费金额:%v\n代理手续费金额:%v\n错误:%w",
			merchant.MerchantID, orderNo, req.MerchantTotalFee, req.AgentTotalFee, handleErr)
	}

	// 3) 成功时才处理代理收益
	if status && req.AgentIncome.GreaterThan(decimal.Zero) {
		agentMoney := dto.AgentMoney{
			AID:        merchant.PId,
			MID:        merchant.MerchantID,
			OrderNo:    orderNo,
			MOrderNo:   mOrderNo,
			OrderMoney: req.OrderAmount,
			Money:      req.AgentIncome,
			Currency:   req.Currency,
			Remark:     "商户代付收益",
		}

		if err := s.mainDao.CreateAgentMoneyLog(agentMoney, dto.MoneyLogTypePayoutComm, "代理代付佣金"); err != nil {
			return fmt.Errorf("[SETTLEMENT] 代理资金结算失败, agentID=%v, orderNo=%v, 金额=%v, err=%w",
				merchant.PId, orderNo, req.AgentIncome, err)
		}
	}

	log.Printf("[SETTLEMENT] 代付结算完成: 商户=%v(金额=%v %s), 代理=%v(收益=%v %s), 订单号=%v, 状态=%v",
		merchant.MerchantID, req.MerchantRecv, req.Currency,
		merchant.PId, req.AgentIncome, req.Currency,
		orderNo, status)

	return nil
}
