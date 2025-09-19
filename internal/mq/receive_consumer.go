package mq

import (
	"encoding/json"
	"github.com/streadway/amqp"
	"log"
	"wht-order-api/internal/callback"
	"wht-order-api/internal/dto"
)

func StartReceiveConsumer() {
	StartConsumer("receive", receiveHandleOrderMessage)
}

func receiveHandleOrderMessage(d amqp.Delivery) {
	var msg dto.ReceiveHyperfOrderMessage
	if err := json.Unmarshal(d.Body, &msg); err != nil {
		log.Printf("❌ [CALLBACK-RECEIVE] Failed to unmarshal order message: %v", err)
		d.Nack(false, false)
		return
	}

	log.Printf("📨 [CALLBACK-RECEIVE] Received order message: MOrderID=%s, Status=%s, Amount=%.2f",
		msg.MOrderID, msg.Status, msg.Amount)

	if err := callback.NewReceiveCallback().HandleUpstreamCallback(&msg); err != nil {
		log.Printf("❌ [CALLBACK-RECEIVE] Failed to process order notification: %v", err)
		d.Nack(false, false)
		return
	}

	d.Ack(false)
	log.Printf("✅ [CALLBACK-RECEIVE] Successfully processed order: %s", msg.MOrderID)
}
