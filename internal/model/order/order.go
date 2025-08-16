package ordermodel

import "time"

type MerchantOrder struct {
	OrderID       uint64    `gorm:"column:order_id;primaryKey"`
	MerchantID    uint64    `gorm:"column:merchant_id"`
	MerchantOrdNo string    `gorm:"column:merchant_ord_no"`
	Amount        string    `gorm:"column:amount"`
	Currency      string    `gorm:"column:currency"`
	PayMethod     string    `gorm:"column:pay_method"`
	Status        int8      `gorm:"column:status"`
	NotifyURL     string    `gorm:"column:notify_url"`
	ChannelID     *uint64   `gorm:"column:channel_id"`
	ChannelOrdNo  *string   `gorm:"column:channel_ord_no"`
	Ext           any       `gorm:"column:ext"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}
