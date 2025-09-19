package dto

import "github.com/shopspring/decimal"

// ReceiveHyperfOrderMessage 匹配 Hyperf 发送的消息格式
type ReceiveHyperfOrderMessage struct {
	MOrderID    string          `json:"mOrderId"`    // 商户订单号
	UpOrderID   string          `json:"upOrderId"`   // 平台流水号
	Amount      decimal.Decimal `json:"amount"`      // 金额
	Status      string          `json:"status"`      // 状态
	UpIpAddress string          `json:"upIpAddress"` // 上游供应商回调IP(不是PHP服务IP)
	Timestamp   int64           `json:"timestamp"`   // 时间戳
}

// ReceiveNotifyMerchantPayload 通知下游商户端的回调通知信息
type ReceiveNotifyMerchantPayload struct {
	TranFlow    string `json:"tran_flow"`
	PaySerialNo string `json:"pay_serial_no"`
	Status      string `json:"status"`
	Msg         string `json:"msg"`
	MerchantNo  string `json:"merchant_no"`
	Sign        string `json:"sign"`
	Amount      string `json:"amount"`
}
