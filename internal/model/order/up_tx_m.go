package ordermodel

import (
	"github.com/shopspring/decimal"
	"time"
)

type UpstreamTx struct {
	UpOrderId  uint64          `gorm:"column:up_order_id;primaryKey"`
	OrderID    uint64          `gorm:"column:order_id"`
	MerchantID string          `gorm:"column:m_id"`
	SupplierId uint64          `gorm:"column:supplier_id"`
	UpOrderNo  string          `gorm:"column:up_order_no"`
	Amount     decimal.Decimal `gorm:"column:amount"`     //订单金额
	PayAmount  decimal.Decimal `gorm:"column:pay_amount"` //实际支付金额
	Currency   string          `gorm:"column:currency"`
	Status     int8            `gorm:"column:status"`
	CreateTime *time.Time      `gorm:"column:create_time"`
	NotifyTime *time.Time      `gorm:"column:notify_time"`
	UpdateTime *time.Time      `gorm:"column:update_time"`
}
