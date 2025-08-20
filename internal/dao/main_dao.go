package dao

import (
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
		Select("a.currency,a.sys_channel_id,a.up_channel_id,a.default_rate as m_default_rate,a.single_fee as m_single_fee,c.order_range,c.default_rate as up_default_rate,c.add_rate as up_single_fee,c.upstream_code,b.coding,c.weight,c.upstream_id,e.account AS up_account,e.receiving_key AS receiving_key,e.channel_code").
		Joins("JOIN w_pay_way AS b ON a.sys_channel_id = b.id").
		Joins("JOIN w_pay_upstream_product AS c ON a.up_channel_id = c.id").
		Joins("LEFT JOIN w_pay_upstream AS e ON c.upstream_id = e.id")
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
