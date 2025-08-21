package dao

import (
	"errors"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"log"
	"time"
	"wht-order-api/internal/dal"
	"wht-order-api/internal/dto"
	mainmodel "wht-order-api/internal/model/main"
)

type MainDao struct{}

func (r *MainDao) GetMerchant(mid string) (*mainmodel.Merchant, error) {
	var m mainmodel.Merchant
	if err := dal.MainDB.Where("app_id=?", mid).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *MainDao) GetMerchantId(mid string) (*mainmodel.Merchant, error) {
	var m mainmodel.Merchant
	if err := dal.MainDB.Where("m_id=?", mid).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *MainDao) GetChannel(cid uint64) (*mainmodel.Channel, error) {
	var ch mainmodel.Channel
	if err := dal.MainDB.Where("channel_id=?", cid).First(&ch).Error; err != nil {
		return nil, err
	}
	return &ch, nil
}

// 查询上游可用通道
func (r *MainDao) SelectPaymentChannel(merchantID uint, channelCode string, currency string) ([]dto.PaymentChannelVo, error) {
	var resp []dto.PaymentChannelVo
	query := dal.MainDB.Table("w_merchant_channel AS a").
		Select("c.title as up_channel_title,c.default_rate as up_channel_rate,c.add_rate as up_channel_fixed_fee,a.currency,a.sys_channel_id,a.up_channel_id,a.default_rate as m_default_rate,a.single_fee as m_single_fee,c.order_range,c.default_rate as up_default_rate,c.add_rate as up_single_fee,c.upstream_code,b.coding,b.title as sys_channel_title,c.weight,c.upstream_id,e.account AS up_account,e.receiving_key AS receiving_key,e.channel_code,f.country").
		Joins("JOIN w_pay_way AS b ON a.sys_channel_id = b.id").
		Joins("JOIN w_pay_upstream_product AS c ON a.up_channel_id = c.id").
		Joins("LEFT JOIN w_pay_upstream AS e ON c.upstream_id = e.id").
		Joins("LEFT JOIN w_currency_code AS f ON f.code = e.currency")
	query.Where("a.m_id =?", merchantID).
		Where("a.currency =?", currency).
		Where("a.status =?", 1).
		Where("b.status =?", 1).
		Where("c.status =?", 1).
		Where("b.coding =?", channelCode)
	query.Order("c.weight desc")
	if err := query.Find(&resp).Error; err != nil {
		return nil, err
	}
	return resp, nil
}

// GetAgentMerchant 查询代理商户
func (r *MainDao) GetAgentMerchant(param dto.QueryAgentMerchant) (*mainmodel.AgentMerchant, error) {
	var ch mainmodel.AgentMerchant
	if err := dal.MainDB.Where("a_id=?", param.AId).Where("m_id=?", param.MId).Where("sys_channel_id=?", param.SysChannelID).Where("up_channel_id=?", param.UpChannelID).First(&ch).Error; err != nil {
		return nil, err
	}
	return &ch, nil
}

// GetSysChannel 查询通道编码
func (r *MainDao) GetSysChannel(channelCode string) (dto.PayWayVo, error) {
	var ch dto.PayWayVo
	if err := dal.MainDB.Table("w_pay_way").Where("coding=?", channelCode).Where("status=?", 1).First(&ch).Error; err != nil {
		return ch, err
	}
	return ch, nil
}

// GetMerchantAccount 获取商户账户信息
func (r *MainDao) GetMerchantAccount(mId string) (dto.MerchantMoney, error) {
	var ch dto.MerchantMoney
	if err := dal.MainDB.Table("w_merchant_money").Where("uid=?", mId).First(&ch).Error; err != nil {
		return ch, err
	}
	return ch, nil
}

// CreateMoneyLog 创建用户资金日志（带事务）
func (r *MainDao) CreateMoneyLog(moneyLog dto.MoneyLog) error {
	log.Printf("创建用户资金日志结算数据: %+v", moneyLog)
	return dal.MainDB.Transaction(func(tx *gorm.DB) error {
		// 检查是否已存在记录
		var existing mainmodel.MoneyLog
		err := tx.Table("w_money_log").
			Where("uid = ? AND order_no = ? AND type = ?", moneyLog.UID, moneyLog.OrderNo, moneyLog.Type).
			First(&existing).Error

		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			// 已存在，不重复插入
			return nil
		}

		// 获取商户账户信息
		var account mainmodel.MerchantMoney
		err = tx.Table("w_merchant_money").
			Where("uid = ? AND currency = ?", moneyLog.UID, moneyLog.Currency).
			First(&account).Error

		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// 创建资金日志
		newLog := mainmodel.MoneyLog{
			Currency:    moneyLog.Currency,
			UID:         moneyLog.UID,
			Money:       moneyLog.Money,
			OrderNo:     moneyLog.OrderNo,
			Type:        moneyLog.Type,
			Operator:    moneyLog.Operator,
			Description: moneyLog.Description,
			OldBalance:  account.Money,
			Balance:     account.Money.Add(moneyLog.Money),
			CreateTime:  time.Now(),
		}

		if err := tx.Table("w_money_log").Create(&newLog).Error; err != nil {
			return err
		}

		// 更新商户资金
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 创建新记录
			newAccount := mainmodel.MerchantMoney{
				UID:         moneyLog.UID,
				Currency:    moneyLog.Currency,
				Money:       moneyLog.Money,
				FreezeMoney: decimal.NewFromFloat(0),
				CreateTime:  time.Now(),
				UpdateTime:  time.Now(),
			}
			if err := tx.Table("w_merchant_money").Create(&newAccount).Error; err != nil {
				return err
			}
		} else {
			// 更新现有记录
			updateData := map[string]interface{}{
				"money":       account.Money.Add(moneyLog.Money),
				"update_time": time.Now(),
			}
			if err := tx.Table("w_merchant_money").
				Where("uid = ? AND currency = ?", moneyLog.UID, moneyLog.Currency).
				Updates(updateData).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// UpsertMerchantMoney 更新或插入商户资金（带事务）
func (r *MainDao) UpsertMerchantMoney(merchantMoney dto.MerchantMoney) error {
	return dal.MainDB.Transaction(func(tx *gorm.DB) error {
		var record mainmodel.MerchantMoney
		err := tx.Table("w_merchant_money").
			Where("uid = ? AND currency = ?", merchantMoney.UID, merchantMoney.Currency).
			First(&record).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 创建新记录
			newRecord := mainmodel.MerchantMoney{
				UID:         merchantMoney.UID,
				Currency:    merchantMoney.Currency,
				Money:       merchantMoney.Money,
				FreezeMoney: decimal.NewFromFloat(0),
				CreateTime:  time.Now(),
				UpdateTime:  time.Now(),
			}
			return tx.Table("w_merchant_money").Create(&newRecord).Error
		} else if err != nil {
			return err
		}

		// 更新余额
		record.Money = record.Money.Add(merchantMoney.Money)
		record.UpdateTime = time.Now()
		return tx.Table("w_merchant_money").Save(&record).Error
	})
}

// CreateAgentMoneyLog 创建代理收益资金日志（带事务）
func (r *MainDao) CreateAgentMoneyLog(agentMoney dto.AgentMoney) error {

	if agentMoney.AID <= 0 {
		return nil
	}

	return dal.MainDB.Transaction(func(tx *gorm.DB) error {
		// 检查是否已存在记录
		var existing mainmodel.AgentMoney
		err := tx.Table("w_agent_money").
			Where("a_id = ? AND order_no = ? AND type = ?", agentMoney.AID, agentMoney.OrderNo, agentMoney.Type).
			First(&existing).Error

		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			// 已存在，不重复插入
			return nil
		}
		zero := decimal.NewFromFloat(0)
		if agentMoney.Money.LessThanOrEqual(zero) {
			return nil
		}
		// 创建代理资金日志
		newLog := mainmodel.AgentMoney{
			Currency:   agentMoney.Currency,
			AID:        agentMoney.AID,
			MID:        agentMoney.MID,
			OrderNo:    agentMoney.OrderNo,
			Type:       agentMoney.Type,
			OrderMoney: agentMoney.OrderMoney,
			Remark:     agentMoney.Remark,
			Money:      agentMoney.Money,
			CreateTime: time.Now(),
		}

		if err := tx.Table("w_agent_money").Create(&newLog).Error; err != nil {
			return err
		}

		// 更新代理商户资金
		var agentAccount mainmodel.MerchantMoney
		err = tx.Table("w_merchant_money").
			Where("uid = ? AND currency = ?", agentMoney.AID, agentMoney.Currency).
			First(&agentAccount).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 创建新记录
			newAccount := mainmodel.MerchantMoney{
				UID:         agentMoney.AID,
				Currency:    agentMoney.Currency,
				Money:       agentMoney.Money,
				FreezeMoney: decimal.NewFromFloat(0),
				CreateTime:  time.Now(),
				UpdateTime:  time.Now(),
			}
			if err := tx.Table("w_merchant_money").Create(&newAccount).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			// 更新现有记录
			updateData := map[string]interface{}{
				"money":       agentAccount.Money.Add(agentMoney.Money),
				"update_time": time.Now(),
			}
			if err := tx.Table("w_merchant_money").
				Where("uid = ? AND currency = ?", agentMoney.AID, agentMoney.Currency).
				Updates(updateData).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// WithTransaction 执行事务操作
func (r *MainDao) WithTransaction(fn func(tx *gorm.DB) error) error {
	return dal.MainDB.Transaction(fn)
}
