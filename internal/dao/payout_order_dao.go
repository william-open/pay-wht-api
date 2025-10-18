package dao

import (
	"errors"
	"fmt"
	"log"
	"wht-order-api/internal/dto"

	"gorm.io/gorm"
	"wht-order-api/internal/dal"
	ordermodel "wht-order-api/internal/model/order"
)

type PayoutOrderDao struct {
	DB *gorm.DB
}

// 工厂方法：默认使用 dal.OrderDB
func NewPayoutOrderDao() *PayoutOrderDao {
	if dal.OrderDB == nil {
		log.Panic("[FATAL] dal.OrderDB is nil - database not initialized")
	}
	return &PayoutOrderDao{DB: dal.OrderDB}
}

// 支持传入自定义 DB（比如 txDB）
func NewPayoutOrderDaoWithDB(db *gorm.DB) *PayoutOrderDao {
	if db == nil {
		log.Panic("[FATAL] db cannot be nil")
	}
	return &PayoutOrderDao{DB: db}
}

// 安全检查方法
func (r *PayoutOrderDao) checkDB() error {
	if r == nil {
		return errors.New("PayoutOrderDao is nil")
	}
	if r.DB == nil {
		return errors.New("DB connection is nil")
	}
	return nil
}

func (r *PayoutOrderDao) Insert(table string, o *ordermodel.MerchantPayOutOrderM) error {
	if err := r.checkDB(); err != nil {
		return fmt.Errorf("insert failed: %w", err)
	}
	return r.DB.Table(table).Create(o).Error
}

func (r *PayoutOrderDao) UpdateByWhere(table string, where map[string]interface{}, data map[string]interface{}) error {
	if err := r.checkDB(); err != nil {
		return fmt.Errorf("update payout order failed: %w", err)
	}
	return r.DB.Table(table).Where(where).Updates(data).Error
}

func (r *PayoutOrderDao) GetByID(table string, id uint64) (*ordermodel.MerchantPayOutOrderM, error) {
	if err := r.checkDB(); err != nil {
		return nil, fmt.Errorf("get by id failed: %w", err)
	}

	var m ordermodel.MerchantPayOutOrderM
	err := r.DB.Table(table).Where("order_id = ?", id).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	return &m, nil
}

func (r *PayoutOrderDao) GetByMerchantNo(table string, mid uint64, mNo string) (*ordermodel.MerchantPayOutOrderM, error) {
	if err := r.checkDB(); err != nil {
		return nil, fmt.Errorf("get by merchant no failed: %w", err)
	}

	var m ordermodel.MerchantPayOutOrderM
	err := r.DB.Table(table).Where("m_id = ? AND m_order_id = ?", mid, mNo).First(&m).Error

	log.Printf("[DEBUG] 代付订单查询: table=%s, mid=%d, mNo=%s, found=%v, error=%v",
		table, mid, mNo, m.OrderID != 0, err)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	return &m, nil
}

func (r *PayoutOrderDao) ListInTables(tables []string, kw string, status *int8, limit, offset int) ([]ordermodel.MerchantPayOutOrderM, int64, error) {
	if err := r.checkDB(); err != nil {
		return nil, 0, fmt.Errorf("list in tables failed: %w", err)
	}

	var out []ordermodel.MerchantPayOutOrderM
	var total int64

	for _, t := range tables {
		q := r.DB.Table(t)
		if kw != "" {
			q = q.Where("merchant_ord_no LIKE ?", "%"+kw+"%")
		}
		if status != nil {
			q = q.Where("status = ?", *status)
		}

		var cnt int64
		if err := q.Count(&cnt).Error; err != nil {
			return nil, 0, fmt.Errorf("count failed for table %s: %w", t, err)
		}
		total += cnt

		var tmp []ordermodel.MerchantPayOutOrderM
		if err := q.Limit(limit).Offset(offset).Find(&tmp).Error; err != nil {
			return nil, 0, fmt.Errorf("find failed for table %s: %w", t, err)
		}
		out = append(out, tmp...)
	}
	return out, total, nil
}

func (r *PayoutOrderDao) InsertTx(table string, o *ordermodel.PayoutUpstreamTxM) error {
	if err := r.checkDB(); err != nil {
		return fmt.Errorf("insert tx failed: %w", err)
	}
	return r.DB.Table(table).Create(o).Error
}

func (r *PayoutOrderDao) UpdateUpTx(table string, o dto.UpdateUpTxVo) error {
	if err := r.checkDB(); err != nil {
		return fmt.Errorf("update up tx failed: %w", err)
	}
	return r.DB.Table(table).Where("up_order_id = ?", o.UpOrderId).Updates(o).Error
}

// 更新代付订单表
func (r *PayoutOrderDao) UpdateOrder(table string, o dto.UpdateOrderVo) error {
	if err := r.checkDB(); err != nil {
		return fmt.Errorf("update payout order failed: %w", err)
	}
	return r.DB.Table(table).Where("order_id = ?", o.OrderId).Updates(o).Error
}

// 代付订单表索引表
func (r *PayoutOrderDao) InsertPayoutOrderIndexTable(table string, o *ordermodel.PayoutOrderIndexM) error {
	if err := r.checkDB(); err != nil {
		return fmt.Errorf("insert index failed: %w", err)
	}
	return r.DB.Table(table).Create(o).Error
}

// 根据 order_id 查询订单
func (r *PayoutOrderDao) GetByOrderId(table string, orderId uint64) (*ordermodel.MerchantPayOutOrderM, error) {
	if err := r.checkDB(); err != nil {
		return nil, fmt.Errorf("get by order id failed: %w", err)
	}

	var m ordermodel.MerchantPayOutOrderM
	err := r.DB.Table(table).Where("order_id = ?", orderId).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	return &m, nil
}

// 根据 order_id 查询上游交易
func (r *PayoutOrderDao) GetTxByOrderId(table string, orderId uint64) (*ordermodel.PayoutUpstreamTxM, error) {
	if err := r.checkDB(); err != nil {
		return nil, fmt.Errorf("get tx by order id failed: %w", err)
	}

	var m ordermodel.PayoutUpstreamTxM
	err := r.DB.Table(table).Where("order_id = ?", orderId).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	return &m, nil
}

// 根据上游订单ID查询交易
func (r *PayoutOrderDao) GetTxByUpOrderId(table string, upOrderId uint64) (*ordermodel.PayoutUpstreamTxM, error) {
	if err := r.checkDB(); err != nil {
		return nil, fmt.Errorf("get tx by up order id failed: %w", err)
	}

	var m ordermodel.PayoutUpstreamTxM
	err := r.DB.Table(table).Where("up_order_id = ?", upOrderId).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	return &m, nil
}
