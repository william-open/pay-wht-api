package dao

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
	"log"
	"wht-order-api/internal/dal"
	ordermodel "wht-order-api/internal/model/order"
)

type IndexTableDao struct {
	DB *gorm.DB
}

// 工厂方法：默认使用 dal.OrderDB
func NewIndexTableDao() *IndexTableDao {
	if dal.OrderDB == nil {
		log.Panic("[FATAL] dal.OrderDB is nil - database not initialized")
	}
	return &IndexTableDao{DB: dal.OrderDB}
}

// 支持传入自定义 DB（比如 txDB）
func NewIndexTableDaoWithDB(db *gorm.DB) *IndexTableDao {
	if db == nil {
		log.Panic("[FATAL] db cannot be nil")
	}
	return &IndexTableDao{DB: db}
}

// 安全检查方法
func (r *IndexTableDao) checkDB() error {
	if r == nil {
		return errors.New("IndexTableDao is nil")
	}
	if r.DB == nil {
		return errors.New("DB connection is nil")
	}
	return nil
}

// 代付索引表
func (r *IndexTableDao) GetByOutIndexTable(table, mOrderId string, mId uint64) (*ordermodel.PayoutOrderIndexM, error) {
	if err := r.checkDB(); err != nil {
		return nil, fmt.Errorf("get by out index table failed: %w", err)
	}

	var m ordermodel.PayoutOrderIndexM
	err := r.DB.Table(table).Where("m_order_id = ?", mOrderId).Where("m_id = ?", mId).First(&m).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	return &m, nil
}

// 代收索引表
func (r *IndexTableDao) GetByIndexTable(table, mOrderId string, mId uint64) (*ordermodel.ReceiveOrderIndexM, error) {
	if err := r.checkDB(); err != nil {
		return nil, fmt.Errorf("get by index table failed: %w", err)
	}

	var m ordermodel.ReceiveOrderIndexM
	err := r.DB.Table(table).Where("m_order_id = ?", mOrderId).Where("m_id = ?", mId).First(&m).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	return &m, nil
}

// 插入代付索引表
func (r *IndexTableDao) InsertPayoutIndex(table string, index *ordermodel.PayoutOrderIndexM) error {
	if err := r.checkDB(); err != nil {
		return fmt.Errorf("insert payout index failed: %w", err)
	}
	return r.DB.Table(table).Create(index).Error
}

// 插入代收索引表
func (r *IndexTableDao) InsertReceiveIndex(table string, index *ordermodel.ReceiveOrderIndexM) error {
	if err := r.checkDB(); err != nil {
		return fmt.Errorf("insert receive index failed: %w", err)
	}
	return r.DB.Table(table).Create(index).Error
}

// 根据订单ID查询代付索引
func (r *IndexTableDao) GetPayoutIndexByOrderId(table string, orderId uint64) (*ordermodel.PayoutOrderIndexM, error) {
	if err := r.checkDB(); err != nil {
		return nil, fmt.Errorf("get payout index by order id failed: %w", err)
	}

	var m ordermodel.PayoutOrderIndexM
	err := r.DB.Table(table).Where("order_id = ?", orderId).First(&m).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	return &m, nil
}

// 根据订单ID查询代收索引
func (r *IndexTableDao) GetReceiveIndexByOrderId(table string, orderId uint64) (*ordermodel.ReceiveOrderIndexM, error) {
	if err := r.checkDB(); err != nil {
		return nil, fmt.Errorf("get receive index by order id failed: %w", err)
	}

	var m ordermodel.ReceiveOrderIndexM
	err := r.DB.Table(table).Where("order_id = ?", orderId).First(&m).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	return &m, nil
}
