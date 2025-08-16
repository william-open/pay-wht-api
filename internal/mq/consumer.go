package mq

import (
	"encoding/json"
	"log"
	"time"
	"wht-order-api/internal/dal"
	ordermodel "wht-order-api/internal/model/order"
	"wht-order-api/internal/shard"

	"github.com/streadway/amqp"
	"gorm.io/gorm"
)

type ChannelCallbackMsg struct {
	OrderID     uint64 `json:"order_id"`
	ChannelTxID string `json:"channel_tx_id"`
	Status      string `json:"status"`
	Amount      string `json:"amount"`
	Currency    string `json:"currency"`
	Ts          int64  `json:"ts"`
	Raw         any    `json:"raw"`
}

func StartConsumers() {
	if dal.RabbitCh == nil {
		return
	}
	msgs, err := dal.RabbitCh.Consume("channel_callback", "", false, false, false, false, nil)
	if err != nil {
		log.Printf("consume channel_callback failed: %v", err)
		return
	}
	for d := range msgs {
		go handleCallback(d)
	}
}

func handleCallback(d amqp.Delivery) {
	var msg ChannelCallbackMsg
	if err := json.Unmarshal(d.Body, &msg); err != nil {
		log.Printf("callback unmarshal err: %v", err)
		d.Nack(false, false)
		return
	}
	ts := time.Unix(msg.Ts, 0)
	orderTable := shard.Table("merchant_order", ts, msg.OrderID)
	upTxTable := shard.Table("up_tx", ts, msg.OrderID)

	err := dal.OrderDB.Transaction(func(tx *gorm.DB) error {
		up := ordermodel.UpTx{
			TxID:        uint64(time.Now().UnixNano()),
			OrderID:     msg.OrderID,
			ChannelTxID: &msg.ChannelTxID,
			Type:        1,
			Amount:      msg.Amount,
			Currency:    msg.Currency,
			Status: func() int8 {
				if msg.Status == "SUCCESS" {
					return 1
				}
				return 2
			}(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := tx.Table(upTxTable).Create(&up).Error; err != nil {
			log.Printf("create up_tx err: %v", err)
		}
		newStatus := 1
		if msg.Status != "SUCCESS" {
			newStatus = 2
		}
		if err := tx.Table(orderTable).Where("order_id = ? AND status <> ?", msg.OrderID, 1).
			Updates(map[string]any{"status": newStatus, "channel_ord_no": msg.ChannelTxID, "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		log.Printf("process callback failed: %v", err)
		d.Nack(false, true)
		return
	}
	d.Ack(false)
}
