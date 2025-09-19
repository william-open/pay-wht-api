package event

import (
	"log"
	"wht-order-api/internal/dto"
)

// PublishOrderStat 异步发布订单统计事件
func PublishOrderStat(msg *dto.OrderMessageMQ) {
	if msg == nil {
		log.Print("❌ [EVENT] 订单统计消息为空")
		return
	}
	go func() {
		if err := publish("order_stat", msg); err != nil {
			log.Printf("❌ [EVENT] 订单统计发布失败: %v", err)
		}
	}()
}
