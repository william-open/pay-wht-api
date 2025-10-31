package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	"wht-order-api/internal/notify"
	"wht-order-api/internal/settlement"
	"wht-order-api/internal/shard"
	"wht-order-api/internal/system"
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
		notify.Notify(system.BotChatID, "warn", "代付回调商户",
			fmt.Sprintf("⚠️ order %v,  商户订单号转换失败: %v", msg.MOrderID, err), true)
		return fmt.Errorf("[CALLBACK-PAYOUT] 商户订单号转换失败: %v", err)
	}

	// 2) 获取上游订单
	txTable := shard.UpOutOrderShard.GetTable(mOrderIdNum, time.Now())
	var upOrder orderModel.UpstreamTx
	if err := dal.OrderDB.Table(txTable).
		Where("up_order_id = ?", mOrderIdNum).
		First(&upOrder).Error; err != nil {
		notify.Notify(system.BotChatID, "warn", "代付回调商户",
			fmt.Sprintf("⚠️ 上游订单不存在 MOrderID=%v: %v", mOrderIdNum, err), true)
		return fmt.Errorf("[CALLBACK-PAYOUT] 上游订单不存在 MOrderID=%v: %w", mOrderIdNum, err)
	}

	// 3) 验证上游IP
	if !verifyUpstreamWhitelist(upOrder.SupplierId, msg.UpIpAddress) {
		notifyMsg := fmt.Sprintf("[CALLBACK-PAYOUT] 上游IP不在白名单内, MOrderID=%v, upstreamId=%v, ip=%s",
			mOrderIdNum, upOrder.SupplierId, msg.UpIpAddress)
		// ⚠️ 每次失败后都发 Telegram
		notify.Notify(system.BotChatID, "warn", "[CALLBACK-PAYOUT]上游IP不在白名单内",
			notifyMsg, true)
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
		notify.Notify(system.BotChatID, "warn", "代付回调商户",
			fmt.Sprintf("⚠️ 更新上游订单失败 MOrderID=%v: %v", mOrderIdNum, err), true)
		return fmt.Errorf("[CALLBACK-PAYOUT] 更新上游订单失败 MOrderID=%v: %w", mOrderIdNum, err)
	}

	// 5) 获取商户订单
	orderTable := shard.OutOrderShard.GetTable(upOrder.OrderID, time.Now())
	var order orderModel.MerchantOrder
	if err := dal.OrderDB.Table(orderTable).
		Where("order_id = ?", upOrder.OrderID).
		First(&order).Error; err != nil {
		notify.Notify(system.BotChatID, "warn", "代付回调商户",
			fmt.Sprintf("⚠️  商户订单不存在 OrderID=%v: %v", upOrder.OrderID, err), true)
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
		notify.Notify(system.BotChatID, "warn", "代付回调商户",
			fmt.Sprintf("⚠️  更新商户订单失败 OrderID=%v: %v", upOrder.OrderID, err), true)
		return fmt.Errorf("[CALLBACK-PAYOUT] 更新商户订单失败 OrderID=%v: %w", upOrder.OrderID, err)
	}

	// 6) 校验商户
	mainDao := dao.NewMainDao()
	merchant, err := mainDao.GetMerchantId(upOrder.MerchantID)
	if err != nil || merchant == nil || merchant.Status != 1 {
		notify.Notify(system.BotChatID, "warn", "代付回调商户",
			fmt.Sprintf("⚠️  商户无效 merchantID=%v, err=%v", upOrder.MerchantID, err), true)
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
		notifyMsg := fmt.Sprintf("结算失败 OrderID=%v, err=%w", order.OrderID, err)
		notify.Notify(system.BotChatID, "warn", "[CALLBACK-PAYOUT]",
			notifyMsg, true)
		return fmt.Errorf("[CALLBACK-PAYOUT] 结算失败 OrderID=%v, err=%w", order.OrderID, err)
	}

	// 8) 异步统计（仅成功时）
	if isSuccess {
		go func() {
			country, cErr := mainDao.GetCountry(order.Currency)
			if cErr != nil {
				notify.Notify(system.BotChatID, "warn", "代付回调商户",
					fmt.Sprintf("⚠️  获取国家信息异常: %v", cErr), true)
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
				notify.Notify(system.BotChatID, "warn", "代付回调商户",
					fmt.Sprintf("⚠️  发布订单统计失败 OrderID=%v: %v", order.OrderID, err), true)
				log.Printf("发布订单统计失败 OrderID=%v: %v", order.OrderID, err)
			}
		}()
	}
	//代付订单失败不直接给商户推送消息
	if statusText == "FAIL" {
		notifyMsg := fmt.Sprintf("⚠️ [CALLBACK-PAYOUT] 订单代付失败，不自动进行商户推送，进入人工流程,OrderID= %v,UpOrderId= %v,MOrderId= %v, status= %s", order.OrderID, order.UpOrderID, order.MOrderID, statusText)
		log.Printf(notifyMsg)
		notify.Notify(system.BotChatID, "warn", "[CALLBACK-PAYOUT]",
			notifyMsg, true)
		return nil
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
			notify.Notify(system.BotChatID, "warn", "代付回调商户",
				fmt.Sprintf("⚠️   已成功通知商户 OrderID=%v (第%d次)", order.OrderID, i), true)
			log.Printf("✅ [CALLBACK-PAYOUT] 已成功通知商户 OrderID=%v (第%d次)", order.OrderID, i)
			return nil
		}
		log.Printf("⚠️ [CALLBACK-PAYOUT] 通知商户失败 OrderID=%v (第%d/%d次): %v",
			order.OrderID, i, payoutMaxRetry, lastErr)
		notify.Notify(system.BotChatID, "warn", "代付回调商户",
			fmt.Sprintf("⚠️   通知商户失败 OrderID=%v (第%d/%d次): %v",
				order.OrderID, i, payoutMaxRetry, lastErr), true)
		time.Sleep(time.Duration(i*2) * time.Second)
	}

	notify.Notify(system.BotChatID, "warn", "代付回调商户",
		fmt.Sprintf("⚠️  通知商户失败 merchant=%v, orderID=%v, retries=%d, lastErr=%v", payload.MerchantNo, order.OrderID, payoutMaxRetry, lastErr), true)

	return fmt.Errorf("[CALLBACK-PAYOUT] 通知商户失败 merchant=%v, orderID=%v, retries=%d, lastErr=%v",
		payload.MerchantNo, order.OrderID, payoutMaxRetry, lastErr)
}

// notifyPayoutCallback 通知 Telegram 封装
func notifyPayoutCallback(level, title string, payload dto.PayoutNotifyMerchantPayload, url, desc, req, resp string) {
	text := fmt.Sprintf(
		"商户号: %s\n订单号: %s\n回调地址: %s\n描述: %s\n请求参数: %s\n响应参数: %s",
		payload.MerchantNo,
		payload.TranFlow,
		url,
		desc,
		req,
		resp,
	)
	notify.Notify(system.BotChatID, level, title, text, true)
}

// payoutNotifyMerchant 通知商户并检查响应
func (s *PayoutCallback) payoutNotifyMerchant(url string, payload dto.PayoutNotifyMerchantPayload) error {
	// 转 JSON
	body, err := json.Marshal(payload)
	if err != nil {
		notifyPayoutCallback("warn", "[回调商户-代付] 序列化失败", payload, url, err.Error(), utils.MapToJSON(payload), "")
		return fmt.Errorf("[CALLBACK-PAYOUT] failed to marshal payload: %w", err)
	}

	log.Printf("[CALLBACK-PAYOUT] >>回调下游商户参数: %s", string(body))

	// 带超时的 HTTP 客户端
	client := &http.Client{Timeout: 8 * time.Second}
	req, _ := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		notifyPayoutCallback("warn", "[回调商户-代付] 请求失败", payload, url, err.Error(), string(body), "")
		return fmt.Errorf("[CALLBACK-PAYOUT] send error: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	respStr := strings.ToLower(strings.TrimSpace(string(respBody)))
	respStr = decodeIfBase64(respStr)
	if respStr == "" {
		respStr = "empty response"
	}

	if resp.StatusCode != http.StatusOK {
		notifyPayoutCallback("warn", "[回调商户-代付] HTTP状态异常", payload, url,
			fmt.Sprintf("HTTP状态: %d", resp.StatusCode),
			string(body), respStr)
		return fmt.Errorf("[CALLBACK-PAYOUT] merchant returned %d: %s", resp.StatusCode, respStr)
	}

	// 判断响应内容
	if respStr != "ok" && respStr != "success" {
		orderErr := s.payoutUpdateMerchantOrder(payload.PaySerialNo, 2, payload.Status)
		if orderErr != nil {
			notifyPayoutCallback("warn", "[回调商户-代付] 订单状态更新失败", payload, url, orderErr.Error(), string(body), respStr)
			return fmt.Errorf("[CALLBACK-PAYOUT] merchant response invalid: %s", respStr)
		}
		notifyPayoutCallback("warn", "[回调商户-代付] 响应无效", payload, url,
			fmt.Sprintf("返回内容无效: %s", respStr),
			string(body), respStr)
		return fmt.Errorf("[CALLBACK-PAYOUT] invalid merchant response: %s", respStr)
	}

	// ✅ 回调成功
	if err := s.payoutUpdateMerchantOrder(payload.PaySerialNo, 1, payload.Status); err != nil {
		notifyPayoutCallback("warn", "[回调商户-代付] 状态更新异常", payload, url, err.Error(), string(body), respStr)
		return fmt.Errorf("[CALLBACK-PAYOUT] update merchant order failed: %v", err)
	}

	notifyPayoutCallback("info", "[回调商户-代付] 调用成功", payload, url,
		"回调状态: Success", string(body), respStr)

	log.Printf("[CALLBACK-PAYOUT] ✅ 通知下游商户成功, 商户: %v, 订单号: %v", payload.MerchantNo, payload.TranFlow)
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
