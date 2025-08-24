package dao

import (
	"errors"
	"gorm.io/gorm"
	"wht-order-api/internal/dal"
	ordermodel "wht-order-api/internal/model/order"
)

type IndexTableDao struct{}

// 代付索引表
func (r *IndexTableDao) GetByOutIndexTable(table, mOrderId string, mId uint64) (*ordermodel.PayoutOrderIndexM, error) {
	var m ordermodel.PayoutOrderIndexM
	err := dal.OrderDB.Table(table).Where("m_order_id = ?", mOrderId).Where("m_id = ?", mId).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &m, err
}

// 代收索引表
func (r *IndexTableDao) GetByIndexTable(table, mOrderId string, mId uint64) (*ordermodel.ReceiveOrderIndexM, error) {
	var m ordermodel.ReceiveOrderIndexM
	err := dal.OrderDB.Table(table).Where("m_order_id = ?", mOrderId).Where("m_id = ?", mId).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &m, err
}
