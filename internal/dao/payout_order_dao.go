package dao

import (
	"errors"
	"wht-order-api/internal/dto"

	"gorm.io/gorm"
	"wht-order-api/internal/dal"
	ordermodel "wht-order-api/internal/model/order"
)

type PayoutOrderDao struct{}

func (r *PayoutOrderDao) Insert(table string, o *ordermodel.MerchantPayOutOrderM) error {
	return dal.OrderDB.Table(table).Create(o).Error
}

func (r *PayoutOrderDao) GetByID(table string, id uint64) (*ordermodel.MerchantPayOutOrderM, error) {
	var m ordermodel.MerchantPayOutOrderM
	err := dal.OrderDB.Table(table).Where("order_id = ?", id).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &m, err
}

func (r *PayoutOrderDao) GetByMerchantNo(table string, mid uint64, mNo string) (*ordermodel.MerchantPayOutOrderM, error) {
	var m ordermodel.MerchantPayOutOrderM
	err := dal.OrderDB.Table(table).Where("m_id=? AND m_order_id=?", mid, mNo).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &m, err
}

func (r *PayoutOrderDao) ListInTables(tables []string, kw string, status *int8, limit, offset int) ([]ordermodel.MerchantPayOutOrderM, int64, error) {
	var out []ordermodel.MerchantPayOutOrderM
	var total int64
	for _, t := range tables {
		q := dal.OrderDB.Table(t)
		if kw != "" {
			q = q.Where("merchant_ord_no LIKE ?", "%"+kw+"%")
		}
		if status != nil {
			q = q.Where("status = ?", *status)
		}
		var cnt int64
		if err := q.Count(&cnt).Error; err != nil {
			return nil, 0, err
		}
		total += cnt

		var tmp []ordermodel.MerchantPayOutOrderM
		if err := q.Limit(limit).Offset(offset).Find(&tmp).Error; err != nil {
			return nil, 0, err
		}
		out = append(out, tmp...)
	}
	return out, total, nil
}

func (r *PayoutOrderDao) InsertTx(table string, o *ordermodel.UpstreamTx) error {
	return dal.OrderDB.Table(table).Create(o).Error
}

func (r *PayoutOrderDao) UpdateUpTx(table string, o dto.UpdateUpTxVo) error {
	return dal.OrderDB.Table(table).Where("up_order_id = ?", o.UpOrderId).Updates(o).Error
}

// 更新代付订单表
func (r *PayoutOrderDao) UpdateOrder(table string, o dto.UpdateOrderVo) error {
	return dal.OrderDB.Table(table).Where("order_id = ?", o.OrderId).Updates(o).Error
}

// 代付订单表索引表
func (r *PayoutOrderDao) InsertReceiveOrderIndexTable(table string, o *ordermodel.ReceiveOrderIndexModel) error {
	return dal.OrderDB.Table(table).Create(o).Error
}
