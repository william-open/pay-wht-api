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
	// 1) 转换商户订单号
	mOrderIdNum, err := strconv.ParseUint(msg.MOrderID, 10, 64)
	if err != nil {
		return fmt.Errorf("[CALLBACK-PAYOUT] 商户订单号转换失败: %v", err)
	}

	// 2) 获取上游订单
	txTable := shard.UpOutOrderShard.GetTable(mOrderIdNum, time.Now())
	var upOrder orderModel.UpstreamTx
	if err := dal.OrderDB.Table(txTable).
		Where("up_order_id = ?", mOrderIdNum).
		First(&upOrder).Error; err != nil {
		return fmt.Errorf("[CALLBACK-PAYOUT] 上游订单不存在 MOrderID=%v: %w", mOrderIdNum, err)
	}

	// 3) 验证上游IP
	if !verifyUpstreamWhitelist(upOrder.SupplierId, msg.UpIpAddress) {
		return fmt.Errorf("[CALLBACK-PAYOUT] 上游IP不在白名单内, MOrderID=%v, upstreamId=%v, ip=%s",
			mOrderIdNum, upOrder.SupplierId, msg.UpIpAddress)
	}

	// 4) 更新上游订单状态
	newStatus := s.payoutGetUpStatusMessage(msg.Status)
	upOrder.Status = newStatus
	upOrder.UpOrderNo = msg.UpOrderID
	upOrder.NotifyTime = utils.PtrTime(time.Now())
	if err := dal.OrderDB.Table(txTable).
		Where("up_order_id = ?", mOrderIdNum).
		Updates(map[string]interface{}{
			"status":      upOrder.Status,
			"up_order_no": upOrder.UpOrderNo,
			"notify_time": upOrder.NotifyTime,
		}).Error; err != nil {
		return fmt.Errorf("[CALLBACK-PAYOUT] 更新上游订单失败 MOrderID=%v: %w", mOrderIdNum, err)
	}

	// 5) 获取商户订单
	orderTable := shard.OutOrderShard.GetTable(upOrder.OrderID, time.Now())
	var order orderModel.MerchantOrder
	if err := dal.OrderDB.Table(orderTable).
		Where("order_id = ?", upOrder.OrderID).
		First(&order).Error; err != nil {
		return fmt.Errorf("[CALLBACK-PAYOUT] 商户订单不存在 OrderID=%v: %w", upOrder.OrderID, err)
	}

	// 更新商户订单状态
	order.Status = newStatus
	order.NotifyTime = utils.PtrTime(time.Now())
	if err := dal.OrderDB.Table(orderTable).
		Where("order_id = ?", upOrder.OrderID).
		Updates(map[string]interface{}{
			"status":      order.Status,
			"notify_time": order.NotifyTime,
		}).Error; err != nil {
		return fmt.Errorf("[CALLBACK-PAYOUT] 更新商户订单失败 OrderID=%v: %w", upOrder.OrderID, err)
	}

	// 6) 校验商户
	mainDao := dao.NewMainDao()
	merchant, err := mainDao.GetMerchantId(upOrder.MerchantID)
	if err != nil || merchant == nil || merchant.Status != 1 {
		return fmt.Errorf("[CALLBACK-PAYOUT] 商户无效 merchantID=%v, err=%v", upOrder.MerchantID, err)
	}

	// 7) 结算逻辑
	settleService := settlement.NewSettlement()
	settlementResult := dto.SettlementResult(order.SettleSnapshot)

	statusText := s.payoutConvertStatus(msg.Status)
	isSuccess := statusText == "SUCCESS"

	if err := settleService.DoPayoutSettlement(settlementResult,
		strconv.FormatUint(merchant.MerchantID, 10),
		order.OrderID,
		isSuccess,
		order.Amount,
	); err != nil {
		return fmt.Errorf("[CALLBACK-PAYOUT] 结算失败 OrderID=%v, err=%w", order.OrderID, err)
	}

	// 8) 异步统计（仅成功时）
	if isSuccess {
		go func() {
			country, cErr := mainDao.GetCountry(order.Currency)
			if cErr != nil {
				log.Printf("获取国家信息异常: %v", cErr)
			}
			if err := s.pub.Publish("order_stat", &dto.OrderMessageMQ{
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
			}); err != nil {
				log.Printf("发布订单统计失败 OrderID=%v: %v", order.OrderID, err)
			}
		}()
	}

	// 9) 通知商户
	payload := dto.PayoutNotifyMerchantPayload{
		TranFlow:    order.MOrderID,
		PaySerialNo: strconv.FormatUint(order.OrderID, 10),
		Status:      msg.Status,
		Msg:         s.payoutGetStatusMessage(msg.Status),
		MerchantNo:  merchant.AppId,
		Amount:      msg.Amount.String(),
	}
	payload.Sign = s.payoutGenerateSign(payload, merchant.ApiKey)

	var lastErr error
	for i := 1; i <= payoutMaxRetry; i++ {
		lastErr = s.payoutNotifyMerchant(order.NotifyURL, payload)
		if lastErr == nil {
			log.Printf("✅ [CALLBACK-PAYOUT] 已成功通知商户 OrderID=%v (第%d次)", order.OrderID, i)
			return nil
		}
		log.Printf("⚠️ [CALLBACK-PAYOUT] 通知商户失败 OrderID=%v (第%d/%d次): %v",
			order.OrderID, i, payoutMaxRetry, lastErr)
		time.Sleep(time.Duration(i*2) * time.Second)
	}

	return fmt.Errorf("[CALLBACK-PAYOUT] 通知商户失败 merchant=%v, orderID=%v, retries=%d, lastErr=%v",
		payload.MerchantNo, order.OrderID, payoutMaxRetry, lastErr)
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
