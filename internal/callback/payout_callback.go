package callback

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
	"wht-order-api/internal/dal"
	"wht-order-api/internal/dao"
	"wht-order-api/internal/dto"
	"wht-order-api/internal/event"
	orderModel "wht-order-api/internal/model/order"
	"wht-order-api/internal/settlement"
	"wht-order-api/internal/shard"
	"wht-order-api/internal/utils"
)

type PayoutCallback struct {
	pub event.Publisher
}

func NewPayoutCallback(pub event.Publisher) *PayoutCallback {

	return &PayoutCallback{pub: pub}
}

const (
	payoutMaxRetry = 3
)

// HandleUpstreamCallback 处理上游代付回调
func (s *PayoutCallback) HandleUpstreamCallback(msg *dto.PayoutHyperfOrderMessage) error {
	// 原 payoutProcessOrderNotification 的逻辑全部迁移到这里
	// 转成 uint64
	mOrderIdNum, err := strconv.ParseUint(msg.MOrderID, 10, 64)
	if err != nil {
		return fmt.Errorf("[CALLBACK-PAYOUT] 平台对接上游的商户订单号,转换失败: %v", err)
	}
	txTable := shard.UpOutOrderShard.GetTable(mOrderIdNum, time.Now())

	var upOrder orderModel.UpstreamTx
	if err := dal.OrderDB.Table(txTable).Where("up_order_id = ?", mOrderIdNum).First(&upOrder).Error; err != nil {
		return fmt.Errorf("[CALLBACK-PAYOUT] tx order not found with MOrderID %v: %w", mOrderIdNum, err)
	}
	// 验证上游供应商IP
	if !verifyUpstreamWhitelist(upOrder.SupplierId, msg.UpIpAddress) {
		return fmt.Errorf("[CALLBACK-PAYOUT] the upstream supplier IP is not in the whitelist. MOrderID:%v, upstreamId:%v, ipAddress:%s", mOrderIdNum, upOrder.SupplierId, msg.UpIpAddress)
	}
	// 更新上游订单状态
	upOrder.Status = s.payoutGetUpStatusMessage(msg.Status)
	upOrder.UpOrderNo = msg.UpOrderID
	upOrder.NotifyTime = utils.PtrTime(time.Now())
	if err := dal.OrderDB.Table(txTable).Where("up_order_id = ?", mOrderIdNum).Updates(&upOrder).Error; err != nil {
		return fmt.Errorf("[CALLBACK-PAYOUT] update up_out_order not found with MOrderID %v: %w", mOrderIdNum, err)
	}

	// 根据商户订单号查找订单
	var order orderModel.MerchantOrder
	orderTable := shard.OutOrderShard.GetTable(upOrder.OrderID, time.Now())
	if err := dal.OrderDB.Table(orderTable).Where("order_id = ?", upOrder.OrderID).First(&order).Error; err != nil {
		return fmt.Errorf("[CALLBACK-PAYOUT] merchant order not found with MOrderID %v: %w", upOrder.OrderID, err)
	}

	order.Status = s.payoutGetUpStatusMessage(msg.Status)
	order.NotifyTime = utils.PtrTime(time.Now())
	if err := dal.OrderDB.Table(orderTable).Where("order_id = ?", upOrder.OrderID).Updates(&order).Error; err != nil {
		return fmt.Errorf("[CALLBACK-PAYOUT] update order not found with MOrderID %v: %w", upOrder.OrderID, err)
	}

	var mainDao *dao.MainDao
	mainDao = dao.NewMainDao()
	merchant, err := mainDao.GetMerchantId(upOrder.MerchantID)
	if err != nil || merchant == nil || merchant.Status != 1 {
		return fmt.Errorf("[CALLBACK-PAYOUT] merchant not found or inactive: %v", err)
	}

	// 如果订单成功就结算商户与代理分润
	if s.payoutConvertStatus(msg.Status) == "SUCCESS" {
		var settleService = settlement.NewSettlement()
		var settlementResult dto.SettlementResult
		settlementResult = dto.SettlementResult(order.SettleSnapshot)
		err := settleService.DoSettlement(settlementResult, strconv.FormatUint(merchant.MerchantID, 10), order.OrderID)
		if err != nil {
			return fmt.Errorf("[CALLBACK-PAYOUT] settlement  failed err: %v", err)
		}

		// 14) 异步处理统计数据
		go func() {
			country, cErr := mainDao.GetCountry(order.Currency)
			if cErr != nil {
				log.Printf("获取国家信息异常: %v", cErr)
			}
			err := s.pub.Publish("order_stat", &dto.OrderMessageMQ{
				OrderID:    strconv.FormatUint(order.OrderID, 10),
				MerchantID: order.MID,
				CountryID:  country.ID,
				ChannelID:  order.ChannelID,
				SupplierID: order.SupplierID,
				Amount:     order.Amount,
				Profit:     *order.Profit,
				Cost:       *order.Cost,
				Fee:        order.Fees,
				Status:     2,
				OrderType:  "payout",
				Currency:   order.Currency,
				CreateTime: time.Now(),
			})
			if err != nil {
				return
			}
		}()
	}

	// 构建回调通知负载
	payload := dto.PayoutNotifyMerchantPayload{
		TranFlow:    order.MOrderID,
		PaySerialNo: strconv.FormatUint(order.OrderID, 10),
		//Status:      convertStatus(msg.Status),
		Status:     msg.Status,
		Msg:        s.payoutGetStatusMessage(msg.Status),
		MerchantNo: merchant.AppId,
		Amount:     msg.Amount.String(),
	}
	payload.Sign = s.payoutGenerateSign(payload, merchant.ApiKey)

	// 执行通知，带重试
	var lastErr error
	for i := 1; i <= payoutMaxRetry; i++ {
		lastErr = s.payoutNotifyMerchant(order.NotifyURL, payload)
		if lastErr == nil {
			log.Printf("✅ [CALLBACK-PAYOUT] Successfully notified merchant for order: %s (try %d)", msg.MOrderID, i)
			return nil
		}
		log.Printf("⚠️ [CALLBACK-PAYOUT] Notify merchant failed (try %d/%d): %v", i, payoutMaxRetry, lastErr)
		time.Sleep(time.Duration(i*2) * time.Second)
	}
	return fmt.Errorf("[CALLBACK-PAYOUT] failed to notify merchant %v after %d retries: %v", payload.MerchantNo, payoutMaxRetry, lastErr)
	// 包括：订单状态更新、结算、商户通知
}

// 通知商户并检查响应
func (s *PayoutCallback) payoutNotifyMerchant(url string, payload dto.PayoutNotifyMerchantPayload) error {
	// 转 JSON
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("[CALLBACK-PAYOUT] failed to marshal payload: %w", err)
	}
	log.Printf("[CALLBACK-PAYOUT] >>回调下游商户参数: %s", string(body))

	// 一定是 POST，并发送 JSON
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("[CALLBACK-PAYOUT] failed to send notification: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("[CALLBACK-PAYOUT] notification failed with status: %d, body: %s", resp.StatusCode, string(respBody))
	}

	// 校验返回内容必须包含 ok 或 success
	respStr := strings.ToLower(strings.TrimSpace(string(respBody)))
	if respStr != "ok" && respStr != "success" {
		orderErr := s.payoutUpdateMerchantOrder(payload.PaySerialNo, 2, payload.Status)
		if orderErr != nil {
			return fmt.Errorf("[CALLBACK-PAYOUT] notify merchant order update data failed: %s", respStr)
		}
		return fmt.Errorf("[CALLBACK-PAYOUT] merchant response invalid: %s", respStr)
	}
	orderErr := s.payoutUpdateMerchantOrder(payload.PaySerialNo, 1, payload.Status)
	if orderErr != nil {
		return fmt.Errorf("[CALLBACK-PAYOUT] notify merchant order update data failed: %s", orderErr)
	}
	log.Printf("[CALLBACK-PAYOUT] 通知下游商户成功,商户: %v 订单号: %v", payload.MerchantNo, payload.TranFlow)
	return nil
}

// 更新商户订单信息
func (s *PayoutCallback) payoutUpdateMerchantOrder(orderId string, notifyStatus int8, orderStatus string) error {

	id, err := strconv.ParseUint(orderId, 10, 64)
	if err != nil {
		return fmt.Errorf("[CALLBACK-PAYOUT] invalid id  with MOrderID %v: %w", orderId, err)
	}

	// 计算分表表名
	orderTable := shard.OutOrderShard.GetTable(id, time.Now())
	// 这里必须有更新字段，例如更新状态、更新时间
	updateData := map[string]interface{}{
		"notify_status": notifyStatus,
		"notify_time":   time.Now(),
		"update_time":   time.Now(),
	}
	if s.payoutGetUpStatusMessage(orderStatus) == 2 { //支付成功时标识完成
		updateData["finish_time"] = time.Now()
		updateData["status"] = 2
	} else {
		updateData["status"] = s.payoutGetUpStatusMessage(orderStatus)
	}

	// 更新数据库
	if err := dal.OrderDB.Table(orderTable).Where("order_id = ?", id).Updates(updateData).Error; err != nil {
		return fmt.Errorf("[CALLBACK-PAYOUT] notify update merchant order failed with MOrderID %v: %w", id, err)
	}

	return nil
}

func (s *PayoutCallback) payoutConvertStatus(hyperfStatus string) string {
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

func (s *PayoutCallback) payoutGetStatusMessage(status string) string {
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
func (s *PayoutCallback) payoutGetUpStatusMessage(status string) int8 {
	switch status {
	case "0000":
		return 2
	case "0001":
		return 1
	case "0005":
		return 3
	default:
		return -1
	}
}

// 生成签名
func (s *PayoutCallback) payoutGenerateSign(p dto.PayoutNotifyMerchantPayload, apiKey string) string {
	signStr := map[string]string{
		"status":        p.Status,
		"msg":           p.Msg,
		"tran_flow":     p.TranFlow,
		"pay_serial_no": p.PaySerialNo,
		"amount":        p.Amount,
	}
	return utils.GenerateSign(signStr, apiKey)
}
