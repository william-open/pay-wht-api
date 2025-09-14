package service

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"
	"wht-order-api/internal/dao"
	"wht-order-api/internal/dto"
)

type SettlementService struct {
	mainDao  *dao.MainDao
	orderDao *dao.OrderDao
}

func NewSettlementService() *SettlementService {
	return &SettlementService{
		mainDao:  dao.NewMainDao(),
		orderDao: dao.NewOrderDao(),
	}
}

// 分片表名生成器：p_order_{YYYYMM}_p{orderID % 4}
func getSettleOrderTable(base string, orderID uint64, t time.Time) string {
	month := t.Format("200601")
	shard := orderID % 4
	return fmt.Sprintf("%s_%s_p%d", base, month, shard)
}

// Settlement 处理代收订单结算逻辑
func (s *SettlementService) Settlement(req dto.SettlementResult, mId string, orderId uint64) error {

	log.Printf("结算数据: %+v,商户: %v,订单号: %v", req, mId, orderId)
	// 1) 主库校验
	merchant, err := s.mainDao.GetMerchantId(mId)
	if err != nil || merchant == nil || merchant.Status != 1 {
		return errors.New("merchant invalid")
	}

	// 创建商户资金日志
	var moneyLog dto.MoneyLog
	moneyLog.Money = req.MerchantRecv
	moneyLog.UID = merchant.MerchantID
	moneyLog.OrderNo = strconv.FormatUint(orderId, 10)
	moneyLog.Type = 1
	moneyLog.Operator = merchant.NickName
	moneyLog.Description = "商户代收"
	moneyLog.Currency = req.Currency

	err = s.mainDao.CreateMoneyLog(moneyLog)
	if err != nil {
		return errors.New("create money log  invalid")
	}
	// 更新代理收益
	var agentMoney dto.AgentMoney
	agentMoney.OrderMoney = req.OrderAmount
	agentMoney.Money = req.AgentIncome
	agentMoney.MID = merchant.MerchantID
	agentMoney.AID = merchant.PId
	agentMoney.OrderNo = strconv.FormatUint(orderId, 10)
	agentMoney.Currency = req.Currency
	agentMoney.Remark = "商户代收收益"

	err = s.mainDao.CreateAgentMoneyLog(agentMoney)
	if err != nil {
		return errors.New("create agent money  invalid")
	}
	return nil
}
