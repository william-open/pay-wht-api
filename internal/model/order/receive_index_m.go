package ordermodel

import "time"

type ReceiveOrderIndexM struct {
	ID                uint64    `gorm:"primaryKey;autoIncrement;column:id;primaryKey" json:"id"`                                      // 主键ID
	MID               uint64    `gorm:"column:m_id;not null;index:uniq_merchant_order,unique" json:"mId"`                             // 商户ID
	MOrderID          string    `gorm:"column:m_order_id;type:varchar(64);not null;index:uniq_merchant_order,unique" json:"mOrderId"` // 商户侧订单号
	OrderID           uint64    `gorm:"column:order_id;not null;index:idx_order_id" json:"orderId"`                                   // 平台订单ID
	OrderTableName    string    `gorm:"column:order_table_name;type:varchar(50)" json:"orderTableName"`                               // 订单表
	OrderLogTableName string    `gorm:"column:order_log_table_name;type:varchar(40);not null" json:"orderLogTableName"`               // 订单日志表
	CreateTime        time.Time `gorm:"column:create_time;not null;default:CURRENT_TIMESTAMP;index:idx_created_at" json:"createTime"` // 创建时间
}
