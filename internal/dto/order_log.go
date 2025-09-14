package dto

import "time"

type AuditContextPayload struct {
	CreatedAt       time.Time
	StartTime       time.Time
	PlatformOrderID uint64
	MerchantNo      string
	TranFlow        string
	TraceID         string
	RequestBody     string
	ResponseBody    string
	Status          string
	ErrorMsg        string
	IP              string
	UserAgent       string
	ChannelCode     string
	RequestType     string
	LatencyMs       int64
}
