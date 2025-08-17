package dto

import "time"

type CreateOrderReq struct {
	MerchantID    uint64         `json:"merchant_id" binding:"required"`
	MerchantOrdNo string         `json:"merchant_ord_no" binding:"required,max=128"`
	Amount        string         `json:"amount" binding:"required"`
	Currency      string         `json:"currency" binding:"required,len=3"`
	PayMethod     string         `json:"pay_method" binding:"required"`
	ChannelID     uint64         `json:"channel_id" binding:"required"`
	NotifyURL     string         `json:"notify_url" binding:"omitempty,url"`
	Ext           map[string]any `json:"ext"`
}

type CreateOrderResp struct {
	OrderID  string      `json:"order_id"`
	Status   string      `json:"status"`
	PayData  interface{} `json:"pay_data,omitempty"`
	ExpireIn int         `json:"expire_in"`
}

type OrderVO struct {
	OrderID       uint64    `json:"order_id"`
	MerchantID    uint64    `json:"merchant_id"`
	MerchantOrdNo string    `json:"merchant_ord_no"`
	Amount        string    `json:"amount"`
	Currency      string    `json:"currency"`
	PayMethod     string    `json:"pay_method"`
	Status        int8      `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}
