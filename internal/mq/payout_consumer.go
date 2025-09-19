package mq

import (
	"encoding/json"
	"github.com/streadway/amqp"
	"log"
	"wht-order-api/internal/callback"
	"wht-order-api/internal/dto"
)

// StartPayoutConsumer 代付消息队列
func StartPayoutConsumer() {
	log.Printf("[CALLBACK-PAYOUT] RabbitMQ payout consumer is starting for order status")

	StartConsumer("payout", payoutHandleOrderMessage)
}

func payoutHandleOrderMessage(d amqp.Delivery) {
	var msg dto.PayoutHyperfOrderMessage
	if err := json.Unmarshal(d.Body, &msg); err != nil {
		log.Printf("❌ [CALLBACK-PAYOUT] 消息解析失败: %v", err)
		d.Nack(false, false)
		return
	}

	log.Printf("📨 [CALLBACK-PAYOUT] 收到代付回调: MOrderID=%s, Status=%s", msg.MOrderID, msg.Status)

	err := callback.NewPayoutCallback().HandleUpstreamCallback(&msg)
	if err != nil {
		log.Printf("❌ [CALLBACK-PAYOUT] 回调处理失败: %v", err)
		d.Nack(false, false)
		return
	}

	d.Ack(false)
	log.Printf("✅ [CALLBACK-PAYOUT]回调处理完成: %s", msg.MOrderID)
}
