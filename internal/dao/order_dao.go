package dao

import (
	"errors"

	"gorm.io/gorm"
	"wht-order-api/internal/dal"
	ordermodel "wht-order-api/internal/model/order"
)

type OrderDao struct{}

func (r *OrderDao) Insert(table string, o *ordermodel.MerchantOrder) error {
	return dal.OrderDB.Table(table).Create(o).Error
}

func (r *OrderDao) GetByID(table string, id uint64) (*ordermodel.MerchantOrder, error) {
	var m ordermodel.MerchantOrder
	err := dal.OrderDB.Table(table).Where("order_id = ?", id).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &m, err
}

func (r *OrderDao) GetByMerchantNo(table string, mid uint64, mNo string) (*ordermodel.MerchantOrder, error) {
	var m ordermodel.MerchantOrder
	err := dal.OrderDB.Table(table).Where("m_id=? AND m_order_id=?", mid, mNo).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &m, err
}

func (r *OrderDao) ListInTables(tables []string, kw string, status *int8, limit, offset int) ([]ordermodel.MerchantOrder, int64, error) {
	var out []ordermodel.MerchantOrder
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

		var tmp []ordermodel.MerchantOrder
		if err := q.Limit(limit).Offset(offset).Find(&tmp).Error; err != nil {
			return nil, 0, err
		}
		out = append(out, tmp...)
	}
	return out, total, nil
}

func (r *OrderDao) InsertTx(table string, o *ordermodel.UpstreamTx) error {
	return dal.OrderDB.Table(table).Create(o).Error
}
