package dao

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
	"log"
	"wht-order-api/internal/dal"
	"wht-order-api/internal/dto"
	ordermodel "wht-order-api/internal/model/order"
)

type OrderDao struct {
	DB *gorm.DB
}

// 工厂方法：默认使用 dal.OrderDB
func NewOrderDao() *OrderDao {
	if dal.OrderDB == nil {
		log.Panic("[FATAL] dal.OrderDB is nil - database not initialized")
	}
	return &OrderDao{DB: dal.OrderDB}
}

// 支持传入自定义 DB（比如 txDB）
func NewOrderDaoWithDB(db *gorm.DB) *OrderDao {
	if db == nil {
		log.Panic("[FATAL] db cannot be nil")
	}
	return &OrderDao{DB: db}
}

// 安全检查方法
func (r *OrderDao) checkDB() error {
	if r == nil {
		return errors.New("OrderDao is nil")
	}
	if r.DB == nil {
		return errors.New("DB connection is nil")
	}
	return nil
}

// 插入订单
func (r *OrderDao) Insert(table string, o *ordermodel.MerchantOrder) error {
	if err := r.checkDB(); err != nil {
		return fmt.Errorf("insert order failed: %w", err)
	}
	return r.DB.Table(table).Create(o).Error
}

// 根据 order_id 获取订单
func (r *OrderDao) GetByID(table string, id uint64) (*ordermodel.MerchantOrder, error) {
	if err := r.checkDB(); err != nil {
		return nil, fmt.Errorf("get by id failed: %w", err)
	}

	var m ordermodel.MerchantOrder
	err := r.DB.Table(table).Where("order_id = ?", id).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	return &m, nil
}

// 根据商户号 + 商户订单号获取订单
func (r *OrderDao) GetByMerchantNo(table string, mid uint64, mNo string) (*ordermodel.MerchantOrder, error) {
	if err := r.checkDB(); err != nil {
		return nil, fmt.Errorf("get by merchant no failed: %w", err)
	}

	var m ordermodel.MerchantOrder
	err := r.DB.Table(table).Where("m_id = ? AND m_order_id = ?", mid, mNo).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	return &m, nil
}

// 跨多表查询订单
func (r *OrderDao) ListInTables(tables []string, kw string, status *int8, limit, offset int) ([]ordermodel.MerchantOrder, int64, error) {
	if err := r.checkDB(); err != nil {
		return nil, 0, fmt.Errorf("list in tables failed: %w", err)
	}

	var out []ordermodel.MerchantOrder
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

		// ⚠️ 分页逻辑不准确（跨表时偏移重复），建议调用方传入单表分页
		var tmp []ordermodel.MerchantOrder
		if err := q.Limit(limit).Offset(offset).Find(&tmp).Error; err != nil {
			return nil, 0, fmt.Errorf("find failed for table %s: %w", t, err)
		}
		out = append(out, tmp...)
	}

	return out, total, nil
}

// 插入上游交易
func (r *OrderDao) InsertTx(table string, o *ordermodel.UpstreamTx) error {
	if err := r.checkDB(); err != nil {
		return fmt.Errorf("insert tx failed: %w", err)
	}
	return r.DB.Table(table).Create(o).Error
}

// 更新上游交易
func (r *OrderDao) UpdateUpTx(table string, o dto.UpdateUpTxVo) error {
	if err := r.checkDB(); err != nil {
		return fmt.Errorf("update up tx failed: %w", err)
	}
	return r.DB.Table(table).Where("up_order_id = ?", o.UpOrderId).Updates(o).Error
}

// 更新订单
func (r *OrderDao) UpdateOrder(table string, o dto.UpdateOrderVo) error {
	if err := r.checkDB(); err != nil {
		return fmt.Errorf("update order failed: %w", err)
	}
	return r.DB.Table(table).Where("order_id = ?", o.OrderId).Updates(o).Error
}

// 插入代收订单索引表
func (r *OrderDao) InsertReceiveOrderIndexTable(table string, o *ordermodel.ReceiveOrderIndexM) error {
	if err := r.checkDB(); err != nil {
		return fmt.Errorf("insert receive order index failed: %w", err)
	}
	return r.DB.Table(table).Create(o).Error
}

// 根据 order_id 查询订单
func (r *OrderDao) GetByOrderId(table string, orderId uint64) (*ordermodel.MerchantOrder, error) {
	if err := r.checkDB(); err != nil {
		return nil, fmt.Errorf("get by order id failed: %w", err)
	}

	var m ordermodel.MerchantOrder
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
func (r *OrderDao) GetTxByOrderId(table string, orderId uint64) (*ordermodel.UpstreamTx, error) {
	if err := r.checkDB(); err != nil {
		return nil, fmt.Errorf("get tx by order id failed: %w", err)
	}

	var m ordermodel.UpstreamTx
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
func (r *OrderDao) GetTxByUpOrderId(table string, upOrderId uint64) (*ordermodel.UpstreamTx, error) {
	if err := r.checkDB(); err != nil {
		return nil, fmt.Errorf("get tx by up order id failed: %w", err)
	}

	var m ordermodel.UpstreamTx
	err := r.DB.Table(table).Where("up_order_id = ?", upOrderId).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	return &m, nil
}

// 根据状态查询订单
func (r *OrderDao) GetByStatus(table string, status int8) ([]ordermodel.MerchantOrder, error) {
	if err := r.checkDB(); err != nil {
		return nil, fmt.Errorf("get by status failed: %w", err)
	}

	var orders []ordermodel.MerchantOrder
	err := r.DB.Table(table).Where("status = ?", status).Find(&orders).Error
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	return orders, nil
}

// 批量更新订单状态
func (r *OrderDao) BatchUpdateStatus(table string, orderIds []uint64, status int8) error {
	if err := r.checkDB(); err != nil {
		return fmt.Errorf("batch update status failed: %w", err)
	}

	if len(orderIds) == 0 {
		return nil
	}

	return r.DB.Table(table).
		Where("order_id IN (?)", orderIds).
		Update("status", status).Error
}

// 获取订单数量统计
func (r *OrderDao) GetOrderCount(table string, mid uint64, status *int8) (int64, error) {
	if err := r.checkDB(); err != nil {
		return 0, fmt.Errorf("get order count failed: %w", err)
	}

	query := r.DB.Table(table).Where("m_id = ?", mid)
	if status != nil {
		query = query.Where("status = ?", *status)
	}

	var count int64
	err := query.Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("count failed: %w", err)
	}
	return count, nil
}
