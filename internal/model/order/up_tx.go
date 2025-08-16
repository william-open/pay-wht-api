package ordermodel

import "time"

type UpTx struct {
	TxID        uint64    `gorm:"column:tx_id;primaryKey"`
	OrderID     uint64    `gorm:"column:order_id"`
	MerchantID  uint64    `gorm:"column:merchant_id"`
	ChannelID   uint64    `gorm:"column:channel_id"`
	ChannelTxID *string   `gorm:"column:channel_tx_id"`
	Type        int8      `gorm:"column:type"`
	Amount      string    `gorm:"column:amount"`
	Currency    string    `gorm:"column:currency"`
	Status      int8      `gorm:"column:status"`
	Detail      any       `gorm:"column:detail"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}
