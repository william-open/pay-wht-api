package dto

import (
	"github.com/shopspring/decimal"
	"time"
)

type OrderMessageMQ struct {
	OrderID    string
	MerchantID uint64
	CountryID  int64
	ChannelID  int64
	SupplierID int64
	Amount     decimal.Decimal
	Profit     decimal.Decimal
	Cost       decimal.Decimal
	Fee        decimal.Decimal
	Status     int
	OrderType  string
	Currency   string
	CreateTime time.Time
}

type OrderCreatedEvent struct {
	OrderID       uint64 `json:"order_id"`
	MerchantID    uint64 `json:"merchant_id"`
	MerchantOrdNo string `json:"merchant_ord_no"`
	Amount        string `json:"amount"`
	Currency      string `json:"currency"`
	PayMethod     string `json:"pay_method"`
	CreatedAt     int64  `json:"created_at"`
}
