package logger

import (
	"log"
	"wht-order-api/internal/dal"
	"wht-order-api/internal/dto"
	orderModel "wht-order-api/internal/model/order"
	"wht-order-api/internal/shard"
)

// WriteOrderLog 写入请求日志
func WriteOrderLog(payload *dto.AuditContextPayload) {
	if payload == nil {
		log.Printf("[AuditLogger] payload 为空，跳过写入")
		return
	}
	var table string
	switch payload.RequestType {
	case "receive":
		table = shard.OrderLogShard.GetTable(payload.PlatformOrderID, payload.CreatedAt)
	case "payout":
		table = shard.OutOrderLogShard.GetTable(payload.PlatformOrderID, payload.CreatedAt)
	default:
		log.Printf("[AuditLogger] payload request_type[%v]，不支持", payload.RequestType)
		return

	}

	if table == "" {
		log.Printf("[AuditLogger] 表名为空，PlatformOrderID=%d, CreatedAt=%v", payload.PlatformOrderID, payload.CreatedAt)
		return
	}
	logEntry := orderModel.MerchantOrderLog{
		PlatformOrderID: payload.PlatformOrderID,
		MerchantNo:      payload.MerchantNo,
		TranFlow:        payload.TranFlow,
		TraceID:         payload.TraceID,
		RequestBody:     payload.RequestBody,
		ResponseBody:    payload.ResponseBody,
		Status:          payload.Status,
		ErrorMsg:        payload.ErrorMsg,
		IP:              payload.IP,
		UserAgent:       payload.UserAgent,
		ChannelCode:     payload.ChannelCode, // ✅ 字段名统一
		LatencyMs:       payload.LatencyMs,
		CreatedAt:       payload.CreatedAt,
	}

	go func(entry orderModel.MerchantOrderLog, tableName string) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[AuditLogger] goroutine panic: trace_id=%s, err=%v", entry.TraceID, r)
			}
		}()

		if err := dal.OrderDB.Table(tableName).Create(&entry).Error; err != nil {
			log.Printf("[AuditLogger] 写入失败: table=%s, trace_id=%s, err=%v", tableName, entry.TraceID, err)
		}
	}(logEntry, table)
}
