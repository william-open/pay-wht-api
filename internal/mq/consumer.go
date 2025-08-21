package mq

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"wht-order-api/internal/dao"
	"wht-order-api/internal/dto"
	"wht-order-api/internal/service"

	"github.com/streadway/amqp"
	"wht-order-api/internal/dal"
	ordermodel "wht-order-api/internal/model/order"
	"wht-order-api/internal/utils"
)

// HyperfOrderMessage 匹配 Hyperf 发送的消息格式
type HyperfOrderMessage struct {
	MOrderID  string  `json:"mOrderId"`  // 商户订单号
	UpOrderID string  `json:"upOrderId"` // 平台流水号
	Amount    float64 `json:"amount"`    // 金额
	Status    string  `json:"status"`    // 状态
	Timestamp int64   `json:"timestamp"` // 时间戳
}

// NotifyMerchantPayload 通知下游商户端的回调通知信息
type NotifyMerchantPayload struct {
	TranFlow    string `json:"tran_flow"`
	PaySerialNo string `json:"pay_serial_no"`
	Status      string `json:"status"`
	Msg         string `json:"msg"`
	MerchantNo  string `json:"merchant_no"`
	Sign        string `json:"sign"`
	Amount      string `json:"amount"`
}

const (
	maxRetry     = 3
	exchangeName = "order_exchange"        // 匹配 Hyperf
	queueName    = "up.order.notify.queue" // 匹配 Hyperf
	routingKey   = "order.status"          // 匹配 Hyperf
)

func StartConsumers() {
	log.Printf("RabbitMQ consumer is starting for order status")

	if dal.RabbitCh == nil {
		log.Println("RabbitMQ channel not initialized")
		return
	}

	// 声明交换器
	err := dal.RabbitCh.ExchangeDeclare(
		exchangeName,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Printf("❌ Failed to declare exchange: %v", err)
		return
	}

	// 声明队列
	queue, err := dal.RabbitCh.QueueDeclare(
		queueName,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Printf("❌ Failed to declare queue: %v", err)
		return
	}

	// 绑定队列到交换器
	err = dal.RabbitCh.QueueBind(
		queue.Name,
		routingKey,
		exchangeName,
		false,
		nil,
	)
	if err != nil {
		log.Printf("❌ Failed to bind queue: %v", err)
		return
	}

	// 开始消费
	msgs, err := dal.RabbitCh.Consume(
		queueName,
		"",
		false, // 不自动确认
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Printf("❌ Failed to consume from queue: %v", err)
		return
	}

	log.Printf("✅ Successfully started RabbitMQ consumer on queue: %s", queueName)

	for d := range msgs {
		go handleOrderMessage(d)
	}
}

func handleOrderMessage(d amqp.Delivery) {
	var msg HyperfOrderMessage
	if err := json.Unmarshal(d.Body, &msg); err != nil {
		log.Printf("❌ Failed to unmarshal order message: %v", err)
		d.Nack(false, false)
		return
	}

	log.Printf("📨 Received order message: MOrderID=%s, Status=%s, Amount=%.2f",
		msg.MOrderID, msg.Status, msg.Amount)

	if err := processOrderNotification(msg); err != nil {
		log.Printf("❌ Failed to process order notification: %v", err)
		d.Nack(false, false)
		return
	}

	d.Ack(false)
	log.Printf("✅ Successfully processed order: %s", msg.MOrderID)
}

func processOrderNotification(msg HyperfOrderMessage) error {
	// 转成 uint64
	mOrderIdNum, err := strconv.ParseUint(msg.MOrderID, 10, 64)
	if err != nil {
		return fmt.Errorf("平台对接上游的商户订单号,转换失败: %v", err)
	}
	txTable := getOrderTable("p_up_order", mOrderIdNum, time.Now())

	var upOrder ordermodel.UpstreamTx
	if err := dal.OrderDB.Table(txTable).Where("up_order_id = ?", mOrderIdNum).First(&upOrder).Error; err != nil {
		return fmt.Errorf("tx order not found with MOrderID %v: %w", mOrderIdNum, err)
	}

	// 更新上游订单状态
	upOrder.Status = getUpStatusMessage(msg.Status)
	upOrder.UpOrderNo = msg.UpOrderID
	upOrder.NotifyTime = time.Now()
	if err := dal.OrderDB.Table(txTable).Where("up_order_id = ?", mOrderIdNum).Updates(&upOrder).Error; err != nil {
		return fmt.Errorf("update up_order not found with MOrderID %v: %w", mOrderIdNum, err)
	}

	// 根据商户订单号查找订单
	var order ordermodel.MerchantOrder
	orderTable := getOrderTable("p_order", upOrder.OrderID, time.Now())
	if err := dal.OrderDB.Table(orderTable).Where("order_id = ?", upOrder.OrderID).First(&order).Error; err != nil {
		return fmt.Errorf("merchant order not found with MOrderID %v: %w", upOrder.OrderID, err)
	}

	order.Status = getUpStatusMessage(msg.Status)
	order.NotifyTime = time.Now()
	if err := dal.OrderDB.Table(orderTable).Where("order_id = ?", upOrder.OrderID).Updates(&order).Error; err != nil {
		return fmt.Errorf("update order not found with MOrderID %v: %w", upOrder.OrderID, err)
	}

	var mainDao *dao.MainDao
	merchant, err := mainDao.GetMerchantId(upOrder.MerchantID)
	if err != nil || merchant == nil || merchant.Status != 1 {
		return fmt.Errorf("merchant not found or inactive: %v", err)
	}

	// 如果订单成功就结算商户与代理分润
	if convertStatus(msg.Status) == "SUCCESS" {
		var settleService = &service.SettlementService{}
		var settlementResult dto.SettlementResult
		settlementResult = dto.SettlementResult(order.SettleSnapshot)
		err := settleService.Settlement(settlementResult, strconv.FormatUint(merchant.MerchantID, 10), order.OrderID)
		if err != nil {
			return fmt.Errorf("settlement  failed err: %v", err)
		}
	}

	// 构建回调通知负载
	payload := NotifyMerchantPayload{
		TranFlow:    order.MOrderID,
		PaySerialNo: strconv.FormatUint(order.OrderID, 10),
		//Status:      convertStatus(msg.Status),
		Status:     msg.Status,
		Msg:        getStatusMessage(msg.Status),
		MerchantNo: merchant.AppId,
		Amount:     fmt.Sprintf("%.2f", msg.Amount),
	}
	payload.Sign = generateSign(payload, merchant.ApiKey)

	// 执行通知，带重试
	var lastErr error
	for i := 1; i <= maxRetry; i++ {
		lastErr = notifyMerchant(order.NotifyURL, payload)
		if lastErr == nil {
			log.Printf("✅ Successfully notified merchant for order: %s (try %d)", msg.MOrderID, i)
			return nil
		}
		log.Printf("⚠️ Notify merchant failed (try %d/%d): %v", i, maxRetry, lastErr)
		time.Sleep(time.Duration(i*2) * time.Second)
	}
	return fmt.Errorf("failed to notify merchant %v after %d retries: %v", payload.MerchantNo, maxRetry, lastErr)
}

// 通知商户并检查响应
func notifyMerchant(url string, payload NotifyMerchantPayload) error {
	// 转 JSON
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}
	log.Printf("回调下游商户参数: %s", string(body))

	// 一定是 POST，并发送 JSON
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("notification failed with status: %d, body: %s", resp.StatusCode, string(respBody))
	}

	// 校验返回内容必须包含 ok 或 success
	respStr := strings.ToLower(strings.TrimSpace(string(respBody)))
	if respStr != "ok" && respStr != "success" {
		orderErr := updateMerchantOrder(payload.PaySerialNo, 2)
		if orderErr != nil {
			return fmt.Errorf("notify merchant order update data failed: %s", respStr)
		}
		return fmt.Errorf("merchant response invalid: %s", respStr)
	}
	orderErr := updateMerchantOrder(payload.PaySerialNo, 1)
	if orderErr != nil {
		return fmt.Errorf("notify merchant order update data failed: %s", orderErr)
	}
	log.Printf("通知下游商户成功,商户: %v 订单号: %v", payload.MerchantNo, payload.TranFlow)
	return nil
}

// 更新商户订单信息
func updateMerchantOrder(orderId string, status int8) error {

	id, err := strconv.ParseUint(orderId, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid id  with MOrderID %v: %w", orderId, err)
	}

	// 计算分表表名
	orderTable := getOrderTable("p_order", id, time.Now())
	// 这里必须有更新字段，例如更新状态、更新时间
	updateData := map[string]interface{}{
		"notify_status": status,
		"notify_time":   time.Now(),
		"update_time":   time.Now(),
	}
	if status == 1 { //支付成功时标识完成
		updateData["finish_time"] = time.Now()
		updateData["status"] = 1
	} else {
		updateData["status"] = 2
	}

	// 更新数据库
	if err := dal.OrderDB.Table(orderTable).Where("order_id = ?", id).Updates(updateData).Error; err != nil {
		return fmt.Errorf("notify update merchant order failed with MOrderID %v: %w", id, err)
	}

	return nil
}

func convertStatus(hyperfStatus string) string {
	switch hyperfStatus {
	case "0000":
		return "SUCCESS"
	case "0001":
		return "PENDING"
	case "0005":
		return "FAIL"
	default:
		return "UNKNOWN"
	}
}

func getStatusMessage(status string) string {
	switch status {
	case "0000":
		return "Approved 完成"
	case "0001":
		return "Pending 处理中"
	case "0005":
		return "Refunded 失败(并退款)"
	default:
		return "Unknown 状态"
	}
}

// 转化上游订单表状态
func getUpStatusMessage(status string) int8 {
	switch status {
	case "0000":
		return 1
	case "0001":
		return 0
	case "0005":
		return 2
	default:
		return -1
	}
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

// 分片表名生成器：p_order_{YYYYMM}_p{orderID % 4}
func getOrderTable(base string, orderID uint64, t time.Time) string {
	month := t.Format("200601")
	shard := orderID % 4
	return fmt.Sprintf("%s_%s_p%d", base, month, shard)
}
