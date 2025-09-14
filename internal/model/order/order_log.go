package ordermodel

import "time"

type MerchantOrderLog struct {
	ID              uint64    `gorm:"primaryKey"`
	PlatformOrderID uint64    `gorm:"column:platform_order_id"`
	MerchantNo      string    `gorm:"column:merchant_no"`
	TranFlow        string    `gorm:"column:tran_flow"`
	TraceID         string    `gorm:"column:trace_id"`
	RequestBody     string    `gorm:"column:request_body"`
	ResponseBody    string    `gorm:"column:response_body"`
	Status          string    `gorm:"column:status"`
	ErrorMsg        string    `gorm:"column:error_msg"`
	IP              string    `gorm:"column:ip"`
	UserAgent       string    `gorm:"column:user_agent"`
	ChannelCode     string    `gorm:"column:channel_code"`
	LatencyMs       int64     `gorm:"column:latency_ms"`
	CreatedAt       time.Time `gorm:"column:created_at"`
}
