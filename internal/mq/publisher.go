package mq

import (
	"encoding/json"
	"log"
	"wht-order-api/internal/dal"

	"github.com/streadway/amqp"
)

type OrderCreatedEvent struct {
	OrderID       uint64 `json:"order_id"`
	MerchantID    uint64 `json:"merchant_id"`
	MerchantOrdNo string `json:"merchant_ord_no"`
	Amount        string `json:"amount"`
	Currency      string `json:"currency"`
	PayMethod     string `json:"pay_method"`
	CreatedAt     int64  `json:"created_at"`
}

func PublishOrderCreated(evt OrderCreatedEvent) error {
	if dal.RabbitCh == nil {
		return nil
	}
	b, _ := json.Marshal(evt)
	err := dal.RabbitCh.Publish(
		"order_events",
		"order.created",
		false, false,
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         b,
		},
	)
	if err != nil {
		log.Printf("publish order.created failed: %v", err)
	}
	return err
}
