package mq

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
	"wht-order-api/internal/dao"

	"github.com/streadway/amqp"
	"wht-order-api/internal/dal"
	ordermodel "wht-order-api/internal/model/order"
	"wht-order-api/internal/shard"
	"wht-order-api/internal/utils"
)

type ChannelCallbackMsg struct {
	OrderID     uint64      `json:"order_id"`
	MerchantID  uint64      `json:"merchant_id"`
	ChannelTxID string      `json:"channel_tx_id"`
	Status      string      `json:"status"`
	Amount      string      `json:"amount"`
	Currency    string      `json:"currency"`
	Ts          int64       `json:"ts"`
	Raw         interface{} `json:"raw"`
	RetryCount  int         `json:"retry_count"`
}

type NotifyMerchantPayload struct {
	TranFlow    string `json:"tranFlow"`
	PaySerialNo string `json:"paySerialNo"`
	Status      string `json:"status"`
	Msg         string `json:"msg"`
	MerchantNo  string `json:"merchantNo"`
	Sign        string `json:"sign"`
	Amount      string `json:"amount"`
}

const maxRetry = 3

func StartConsumers() {
	if dal.RabbitCh == nil {
		log.Println("RabbitMQ channel not initialized")
		return
	}
	msgs, err := dal.RabbitCh.Consume("channel_callback", "", false, false, false, false, nil)
	if err != nil {
		log.Printf("❌ consume channel_callback failed: %v", err)
		return
	}
	for d := range msgs {
		go handleCallback(d)
	}
}

func handleCallback(d amqp.Delivery) {
	var msg ChannelCallbackMsg
	if err := json.Unmarshal(d.Body, &msg); err != nil {
		log.Printf("❌ callback unmarshal err: %v", err)
		d.Nack(false, false)
		return
	}

	if err := notifyMerchant(msg); err != nil {
		log.Printf("❌ notify merchant failed: %v", err)

		if msg.RetryCount < maxRetry {
			msg.RetryCount++
			retryBody, _ := json.Marshal(msg)
			_ = dal.RabbitCh.Publish(
				"", "channel_callback", false, false,
				amqp.Publishing{
					ContentType: "application/json",
					Body:        retryBody,
				},
			)
			log.Printf("🔁 retrying notify for order %d (attempt %d)", msg.OrderID, msg.RetryCount)
		} else {
			log.Printf("🚨 max retry reached for order %d", msg.OrderID)
		}

		d.Nack(false, false)
		return
	}

	d.Ack(false)
}

func notifyMerchant(msg ChannelCallbackMsg) error {
	ts := time.Unix(msg.Ts, 0)
	orderTable := shard.Table("p_order", ts, msg.OrderID)

	var order ordermodel.MerchantOrder
	if err := dal.OrderDB.Table(orderTable).Where("order_id = ?", msg.OrderID).First(&order).Error; err != nil {
		return fmt.Errorf("order not found: %w", err)
	}
	var mainDao *dao.MainDao
	merchant, err := mainDao.GetMerchant(string(order.MID))
	if err != nil || merchant == nil || merchant.Status != 1 {
		return fmt.Errorf("merchant not found: %v", err)
	}

	payload := NotifyMerchantPayload{
		TranFlow:    order.MOrderID,
		PaySerialNo: msg.ChannelTxID,
		Status:      msg.Status,
		Msg:         statusMsg(msg.Status),
		MerchantNo:  string(order.MID),
		Amount:      msg.Amount,
	}

	payload.Sign = generateSign(payload, merchant.ApiKey)

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", order.NotifyURL, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return fmt.Errorf("notify failed: %v", err)
	}

	log.Printf("✅ notify success for order %d", order.OrderID)
	return nil
}

func generateSign(p NotifyMerchantPayload, apiKey string) string {

	signStr := map[string]string{
		"status":        p.Status,
		"msg":           p.Msg,
		"tran_flow":     p.TranFlow,
		"pay_serial_no": p.PaySerialNo,
		"amount":        p.Amount,
	}
	return utils.GenerateSign(signStr, apiKey)
}

func statusMsg(status string) string {
	switch status {
	case "SUCCESS":
		return "0000"
	case "PENDING":
		return "0001"
	case "FAIL":
		return "0005"
	default:
		return "0006"
	}
}
