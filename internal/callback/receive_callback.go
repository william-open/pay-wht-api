package callback

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
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

type ReceiveCallback struct {
	pub event.Publisher
}

func NewReceiveCallback(pub event.Publisher) *ReceiveCallback {
	return &ReceiveCallback{pub: pub}
}

const (
	receiveMaxRetry = 3
)

// HandleUpstreamCallback 处理上游代付回调
func (s *ReceiveCallback) HandleUpstreamCallback(msg *dto.ReceiveHyperfOrderMessage) error {
	// 转成 uint64
	mOrderIdNum, err := strconv.ParseUint(msg.MOrderID, 10, 64)
	if err != nil {
		return fmt.Errorf("[CALLBACK-RECEIVE] 平台对接上游的商户订单号,转换失败: %v", err)
	}
	txTable := shard.UpOrderShard.GetTable(mOrderIdNum, time.Now())

	var upOrder orderModel.UpstreamTx
	if err := dal.OrderDB.Table(txTable).Where("up_order_id = ?", mOrderIdNum).First(&upOrder).Error; err != nil {
		return fmt.Errorf("[CALLBACK-RECEIVE] tx order not found with MOrderID %v: %w", mOrderIdNum, err)
	}
	// 验证上游供应商IP
	if !verifyUpstreamWhitelist(upOrder.SupplierId, msg.UpIpAddress) {
		return fmt.Errorf("[CALLBACK-RECEIVE] the upstream supplier IP is not in the whitelist. MOrderID:%v, upstreamId:%v, ipAddress:%s", mOrderIdNum, upOrder.SupplierId, msg.UpIpAddress)
	}
	// 更新上游订单状态
	upOrder.Status = s.receiveGetUpStatusMessage(msg.Status)
	upOrder.UpOrderNo = msg.UpOrderID
	upOrder.NotifyTime = utils.PtrTime(time.Now())
	if err := dal.OrderDB.Table(txTable).Where("up_order_id = ?", mOrderIdNum).Updates(&upOrder).Error; err != nil {
		return fmt.Errorf("[CALLBACK-RECEIVE] update up_order not found with MOrderID %v: %w", mOrderIdNum, err)
	}
	// 判断一下交易金额是否正确
	if msg.Amount.Cmp(upOrder.Amount) != 0 {
		return fmt.Errorf("[CALLBACK-RECEIVE] incorrect transaction amount with MOrderID %v: callback amount:%v:up order amount:%v", mOrderIdNum, msg.Amount, upOrder.Amount)
	}
	// 根据商户订单号查找订单
	var order orderModel.MerchantOrder
	orderTable := shard.OrderShard.GetTable(upOrder.OrderID, time.Now())
	if err := dal.OrderDB.Table(orderTable).Where("order_id = ?", upOrder.OrderID).First(&order).Error; err != nil {
		return fmt.Errorf("[CALLBACK-RECEIVE] merchant order not found with MOrderID %v: %w", upOrder.OrderID, err)
	}

	// 如果商户订单status状态>1,表示已经收到上游回调处理
	if order.Status > 1 {
		return fmt.Errorf("[CALLBACK-RECEIVE] upstream callback merchant order,  have handled with MOrderID %v,order status is: %v", upOrder.OrderID, order.Status)
	}
	// 判断一下交易金额是否正确
	if msg.Amount.Cmp(order.Amount) != 0 {
		return fmt.Errorf("[CALLBACK-RECEIVE] incorrect transaction amount with MOrderID %v: callback amount:%v:order amount:%v", mOrderIdNum, msg.Amount, order.Amount)
	}
	order.Status = s.receiveGetUpStatusMessage(msg.Status)
	order.NotifyTime = utils.PtrTime(time.Now())
	if err := dal.OrderDB.Table(orderTable).Where("order_id = ?", upOrder.OrderID).Updates(&order).Error; err != nil {
		return fmt.Errorf("[CALLBACK-RECEIVE] update order not found with MOrderID %v: %w", upOrder.OrderID, err)
	}

	var mainDao *dao.MainDao
	mainDao = dao.NewMainDao()
	merchant, err := mainDao.GetMerchantId(upOrder.MerchantID)
	if err != nil || merchant == nil || merchant.Status != 1 {
		return fmt.Errorf("[CALLBACK-RECEIVE] merchant not found or inactive: %v", err)
	}

	// 如果订单成功就结算商户与代理分润
	if s.receiveConvertStatus(msg.Status) == "SUCCESS" {
		var settleService = settlement.NewSettlement()
		var settlementResult dto.SettlementResult
		settlementResult = dto.SettlementResult(order.SettleSnapshot)
		err := settleService.DoPaySettlement(settlementResult, strconv.FormatUint(merchant.MerchantID, 10), order.OrderID)
		if err != nil {
			return fmt.Errorf("[CALLBACK-RECEIVE] settlement  failed err: %v", err)
		}

		// 13) 异步处理统计数据
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
				OrderType:  "collect",
				Currency:   order.Currency,
				CreateTime: time.Now(),
			})
			if err != nil {
				return
			}
		}()

	}

	// 构建回调通知负载
	payload := dto.ReceiveNotifyMerchantPayload{
		TranFlow:    order.MOrderID,
		PaySerialNo: strconv.FormatUint(order.OrderID, 10),
		//Status:      convertStatus(msg.Status),
		Status:     msg.Status,
		Msg:        s.receiveGetStatusMessage(msg.Status),
		MerchantNo: merchant.AppId,
		Amount:     msg.Amount.String(),
	}
	payload.Sign = s.receiveGenerateSign(payload, merchant.ApiKey)

	// 执行通知，带重试
	var lastErr error
	for i := 1; i <= receiveMaxRetry; i++ {
		lastErr = s.receiveNotifyMerchant(order.NotifyURL, payload)
		if lastErr == nil {
			log.Printf("✅ [CALLBACK-RECEIVE]Successfully notified merchant for order: %s (try %d)", msg.MOrderID, i)
			return nil
		}
		log.Printf("⚠️ [CALLBACK-RECEIVE]Notify merchant failed (try %d/%d): %v", i, receiveMaxRetry, lastErr)
		time.Sleep(time.Duration(i*2) * time.Second)
	}
	return fmt.Errorf("[CALLBACK-RECEIVE]failed to notify merchant %v after %d retries: %v", payload.MerchantNo, receiveMaxRetry, lastErr)
}

// verifyUpstreamWhitelist 校验上游供应商IP白名单
func (s *ReceiveCallback) verifyUpstreamWhitelist(upstreamId uint64, ipAddress string) bool {
	var mainDao *dao.MainDao
	mainDao = dao.NewMainDao()
	upstream, err := mainDao.GetUpstreamWhitelist(upstreamId)
	if err != nil || upstream == nil || upstream.Status != 1 {
		return false
	}
	// 构建白名单集合
	allowed := make(map[string]struct{})
	ipList := strings.Split(upstream.IpWhitelist, ",")
	for _, ip := range ipList {
		ip = strings.TrimSpace(ip)
		if net.ParseIP(ip) != nil {
			allowed[ip] = struct{}{}
		}
	}

	// 验证请求 IP 是否允许
	if _, ok := allowed[ipAddress]; !ok {
		return false
	}

	return true
}

// 通知商户并检查响应
func (s *ReceiveCallback) receiveNotifyMerchant(url string, payload dto.ReceiveNotifyMerchantPayload) error {
	// 转 JSON
	body, err := json.Marshal(payload)
	if err != nil {
		notify.Notify(
			system.BotChatID,
			"warn",
			"[回调商户] 调用失败",
			fmt.Sprintf(
				"商户号: %s\n订单号: %s\n回调状态: Failed\n回调地址: %s\n错误描述: %s\n请求参数: %s\n响应参数: %s",
				payload.MerchantNo,
				payload.TranFlow,
				url,
				err.Error(),
				utils.MapToJSON(payload),
				utils.MapToJSON(body),
			),
			true,
		)
		return fmt.Errorf("[CALLBACK-RECEIVE] failed to marshal payload: %w", err)
	}
	log.Printf("[CALLBACK-RECEIVE] >>回调下游商户参数: %s", string(body))

	// 一定是 POST，并发送 JSON
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		notify.Notify(
			system.BotChatID,
			"warn",
			"[回调商户] 调用失败",
			fmt.Sprintf(
				"商户号: %s\n订单号: %s\n回调状态: Failed\n回调地址: %s\n错误描述: %s\n请求参数: %s\n响应参数: %+v",
				payload.MerchantNo,
				payload.TranFlow,
				url,
				err.Error(),
				utils.MapToJSON(payload),
				resp,
			),
			true,
		)
		return fmt.Errorf("[CALLBACK-RECEIVE] failed to send notification: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		notify.Notify(
			system.BotChatID,
			"warn",
			"[回调商户] 调用失败",
			fmt.Sprintf(
				"商户号: %s\n订单号: %s\n回调状态: Failed\n回调地址: %s\nHTTP状态: %+v\n请求参数: %s\n响应参数: %s",
				payload.MerchantNo,
				payload.TranFlow,
				url,
				resp.StatusCode,
				utils.MapToJSON(payload),
				string(respBody),
			),
			true,
		)
		return fmt.Errorf("[CALLBACK-RECEIVE] notification failed with status: %d, body: %s", resp.StatusCode, string(respBody))
	}

	// 校验返回内容必须包含 ok 或 success
	respStr := strings.ToLower(strings.TrimSpace(string(respBody)))
	if respStr != "ok" && respStr != "success" {
		orderErr := s.receiveUpdateMerchantOrder(payload.PaySerialNo, 2, payload.Status)
		if orderErr != nil {
			notify.Notify(
				system.BotChatID,
				"warn",
				"[回调商户] 调用失败",
				fmt.Sprintf(
					"商户号: %s\n订单号: %s\n回调状态: Failed\n回调地址: %s\n错误描述: %+v\n请求参数: %s\n响应参数: %s",
					payload.MerchantNo,
					payload.TranFlow,
					url,
					resp.StatusCode,
					utils.MapToJSON(payload),
					respStr,
				),
				true,
			)
			return fmt.Errorf("[CALLBACK-RECEIVE] notify merchant order update data failed: %s", respStr)
		}
		notify.Notify(
			system.BotChatID,
			"warn",
			"[回调商户] 调用失败",
			fmt.Sprintf(
				"商户号: %s\n订单号: %s\n回调状态: Failed\n回调地址: %s\n错误描述: %+v\n请求参数: %s\n响应参数: %s",
				payload.MerchantNo,
				payload.TranFlow,
				url,
				resp.StatusCode,
				utils.MapToJSON(payload),
				respStr,
			),
			true,
		)
		return fmt.Errorf("[CALLBACK-RECEIVE] merchant response invalid: %s", respStr)
	}
	orderErr := s.receiveUpdateMerchantOrder(payload.PaySerialNo, 1, payload.Status)
	if orderErr != nil {
		return fmt.Errorf("[CALLBACK-RECEIVE] notify merchant order update data failed: %s", orderErr)
	}
	notify.Notify(
		system.BotChatID,
		"info",
		"[回调商户] 调用成功",
		fmt.Sprintf(
			"商户号: %s\n订单号: %s\n回调状态: Success\n回调地址: %s\n请求参数: %s\n响应参数: %s",
			payload.MerchantNo,
			payload.TranFlow,
			url,
			utils.MapToJSON(payload),
			respStr,
		),
		true,
	)
	log.Printf("[CALLBACK-RECEIVE] 通知下游商户成功,商户: %v 订单号: %v", payload.MerchantNo, payload.TranFlow)
	return nil
}

// 更新商户订单信息
func (s *ReceiveCallback) receiveUpdateMerchantOrder(orderId string, notifyStatus int8, orderStatus string) error {

	id, err := strconv.ParseUint(orderId, 10, 64)
	if err != nil {
		return fmt.Errorf("[CALLBACK-RECEIVE] invalid id  with MOrderID %v: %w", orderId, err)
	}

	// 计算分表表名
	orderTable := shard.OrderShard.GetTable(id, time.Now())
	// 这里必须有更新字段，例如更新状态、更新时间
	updateData := map[string]interface{}{
		"notify_status": notifyStatus,
		"notify_time":   time.Now(),
		"update_time":   time.Now(),
	}
	if s.receiveGetUpStatusMessage(orderStatus) == 2 { //支付成功时标识完成
		updateData["finish_time"] = time.Now()
		updateData["status"] = 2
	} else {
		updateData["status"] = s.receiveGetUpStatusMessage(orderStatus)
	}

	// 更新数据库
	if err := dal.OrderDB.Table(orderTable).Where("order_id = ?", id).Updates(updateData).Error; err != nil {
		return fmt.Errorf("[CALLBACK-RECEIVE] notify update merchant order failed with MOrderID %v: %w", id, err)
	}

	return nil
}

func (s *ReceiveCallback) receiveConvertStatus(hyperfStatus string) string {
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

func (s *ReceiveCallback) receiveGetStatusMessage(status string) string {
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
func (s *ReceiveCallback) receiveGetUpStatusMessage(status string) int8 {
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
func (s *ReceiveCallback) receiveGenerateSign(p dto.ReceiveNotifyMerchantPayload, apiKey string) string {
	signStr := map[string]string{
		"status":        p.Status,
		"msg":           p.Msg,
		"tran_flow":     p.TranFlow,
		"pay_serial_no": p.PaySerialNo,
		"amount":        p.Amount,
	}
	return utils.GenerateSign(signStr, apiKey)
}
