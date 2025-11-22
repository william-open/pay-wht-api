package callback

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/shopspring/decimal"
	"io"
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
		notifyMsg := fmt.Sprintf("交易订单号: %v,转换失败: %+v", mOrderIdNum, err)
		notify.Notify(system.BotChatID, "warn", "代收回调异常",
			notifyMsg, true)
		return fmt.Errorf(notifyMsg)
	}
	txTable := shard.UpOrderShard.GetTable(mOrderIdNum, time.Now())

	var upOrder orderModel.UpstreamTx
	if err := dal.OrderDB.Table(txTable).Where("up_order_id = ?", mOrderIdNum).First(&upOrder).Error; err != nil {
		notifyMsg := fmt.Sprintf("系统未找到交易订单号，交易订单号: %v,平台订单号: %v,错误: %+v", mOrderIdNum, upOrder.OrderID, err)
		notify.Notify(system.BotChatID, "warn", "代收回调异常", notifyMsg, true)
		return fmt.Errorf(notifyMsg)
	}
	// 验证上游供应商IP
	if !verifyUpstreamWhitelist(upOrder.SupplierId, msg.UpIpAddress) {
		title := "[代收回调] 上游IP不在白名单内"
		notifyMsg := fmt.Sprintf(
			"*供应商ID:* `%v`\n"+
				"*交易订单号:* `%v`\n"+
				"*平台订单号:* `%v`\n"+
				"*回调IP:* `%s`\n"+
				"*回调状态:* `%s`\n",
			upOrder.SupplierId, mOrderIdNum, upOrder.OrderID, msg.UpIpAddress, s.receiveConvertStatus(msg.Status),
		)
		notify.Notify(system.BotChatID, "warn", title,
			notifyMsg, true)
		return fmt.Errorf(notifyMsg)
	}
	// 判断一下交易金额是否正确
	if msg.Amount.Cmp(upOrder.Amount) != 0 {
		notifyMsg := fmt.Sprintf("回调交易金额与订单金额不符,交易订单号: %v,平台订单号: %v,上游回调金额: %v,交易金额:%v", mOrderIdNum, upOrder.OrderID, msg.Amount, upOrder.Amount)
		notify.Notify(system.BotChatID, "warn", "代收回调商户",
			notifyMsg, true)
		return fmt.Errorf(notifyMsg)
	}
	// 更新上游订单状态
	upOrder.Status = s.receiveGetUpStatusMessage(msg.Status)
	upOrder.UpOrderNo = msg.UpOrderID
	upOrder.NotifyTime = utils.PtrTime(time.Now())
	if err := dal.OrderDB.Table(txTable).Where("up_order_id = ?", mOrderIdNum).Updates(&upOrder).Error; err != nil {
		notifyMsg := fmt.Sprintf("更新订单交易信息失败,交易订单号: %v,平台订单号: %v,错误: %v", mOrderIdNum, upOrder.OrderID, err)
		notify.Notify(system.BotChatID, "warn", "代收回调异常",
			notifyMsg, true)
		return fmt.Errorf(notifyMsg)
	}

	// 根据商户订单号查找订单
	var order orderModel.MerchantOrder
	orderTable := shard.OrderShard.GetTable(upOrder.OrderID, time.Now())
	if err := dal.OrderDB.Table(orderTable).Where("order_id = ?", upOrder.OrderID).First(&order).Error; err != nil {
		notifyMsg := fmt.Sprintf("平台订单号未找到,交易订单号: %v,平台订单号: %v,错误: %v", mOrderIdNum, upOrder.OrderID, err)
		notify.Notify(system.BotChatID, "warn", "代收回调异常",
			notifyMsg, true)
		return fmt.Errorf(notifyMsg)
	}

	// 如果商户订单status状态>1,表示已经收到上游回调处理
	if order.Status > 1 {
		notifyMsg := fmt.Sprintf("订单状态不是待处理或者未支付状态, 不能进行其他操作流程，进入人工核查阶段。交易订单号: %v,平台订单号: %v,订单状态: %v", mOrderIdNum, upOrder.OrderID, s.receiveConvertStatus(utils.ConvertOrderStatus(order.Status)))
		notify.Notify(system.BotChatID, "warn", "代收回调重复",
			notifyMsg, true)
		return fmt.Errorf(notifyMsg)
	}
	// 判断一下交易金额是否正确
	if msg.Amount.Cmp(order.Amount) != 0 {
		notifyMsg := fmt.Sprintf("回调交易金额与订单金额不符,交易订单号: %v,平台订单号: %v,上游回调金额: %v,交易金额:%v", mOrderIdNum, upOrder.OrderID, msg.Amount, upOrder.Amount)
		notify.Notify(system.BotChatID, "warn", "代收回调商户",
			notifyMsg, true)
		return fmt.Errorf(notifyMsg)
	}
	order.Status = s.receiveGetUpStatusMessage(msg.Status)
	order.NotifyTime = utils.PtrTime(time.Now())
	if err := dal.OrderDB.Table(orderTable).Where("order_id = ?", upOrder.OrderID).Updates(&order).Error; err != nil {
		notifyMsg := fmt.Sprintf("更新订单信息失败,交易订单号: %v,平台订单号: %v,错误: %v", mOrderIdNum, upOrder.OrderID, err)
		notify.Notify(system.BotChatID, "warn", "代收回调商户",
			notifyMsg, true)
		return fmt.Errorf(notifyMsg)
	}

	var mainDao *dao.MainDao
	mainDao = dao.NewMainDao()
	merchant, err := mainDao.GetMerchantId(upOrder.MerchantID)
	if err != nil || merchant == nil || merchant.Status != 1 {
		notifyMsg := fmt.Sprintf("商户没有找到，商户不存在或者未启动,交易订单号: %v,平台订单号: %v,错误: %v", mOrderIdNum, upOrder.OrderID, err)
		notify.Notify(system.BotChatID, "warn", "代收回调商户",
			notifyMsg, true)
		return fmt.Errorf(notifyMsg)
	}

	// 如果订单成功就结算商户与代理分润
	if s.receiveConvertStatus(msg.Status) == "SUCCESS" {
		var settleService = settlement.NewSettlement()
		var settlementResult dto.SettlementResult
		settlementResult = dto.SettlementResult(order.SettleSnapshot)
		err := settleService.DoPaySettlement(settlementResult, strconv.FormatUint(merchant.MerchantID, 10), order.OrderID, order.MOrderID)
		if err != nil {
			notifyMsg := fmt.Sprintf("结算失败\n交易订单号: %v\n平台订单号: %v\n商户订单号: %v\n错误: %v", mOrderIdNum, order.OrderID, order.MOrderID, err)
			notify.Notify(system.BotChatID, "warn", "代收回调商户",
				notifyMsg, true)
			return fmt.Errorf(notifyMsg)
		}

		// 13) 异步处理统计数据
		go func() {
			country, cErr := mainDao.GetCountry(order.Currency)
			if cErr != nil {
				notifyMsg := fmt.Sprintf("订单统计失败,交易订单号: %v,平台订单号:%v,商户订单号:%v,获取国家信息异常: %v", mOrderIdNum, order.OrderID, order.MOrderID, cErr)
				notify.Notify(system.BotChatID, "warn", "代收回调商户",
					notifyMsg, true)
				log.Printf(notifyMsg)
			}
			err := s.pub.Publish("order_stat", &dto.OrderMessageMQ{
				OrderID:       strconv.FormatUint(order.OrderID, 10),
				MerchantID:    order.MID,
				CountryID:     country.ID,
				ChannelID:     order.ChannelID,
				SupplierID:    order.SupplierID,
				Amount:        decimal.Zero,
				SuccessAmount: order.Amount,
				Profit:        *order.Profit,
				Cost:          *order.Cost,
				Fee:           order.Fees,
				Status:        2,
				OrderType:     "collect",
				Currency:      order.Currency,
				CreateTime:    time.Now(),
			})
			if err != nil {
				notifyMsg := fmt.Sprintf("订单统计失败,交易订单号: %v,平台订单号:%v,商户订单号:%v,推送到队列异常: %v", mOrderIdNum, order.OrderID, order.MOrderID, err)
				notify.Notify(system.BotChatID, "warn", "代收回调商户",
					notifyMsg, true)
				return
			}
		}()

	}

	// 构建回调通知负载
	payload := dto.ReceiveNotifyMerchantPayload{
		TranFlow:    order.MOrderID,
		PaySerialNo: strconv.FormatUint(order.OrderID, 10),
		Status:      msg.Status,
		Msg:         s.receiveGetStatusMessage(msg.Status),
		MerchantNo:  merchant.AppId,
		Amount:      msg.Amount.String(),
	}
	payload.Sign = s.receiveGenerateSign(payload, merchant.ApiKey)

	// 执行通知，带重试
	var lastErr error
	for i := 1; i <= receiveMaxRetry; i++ {
		lastErr = s.receiveNotifyMerchant(merchant.NickName, order.NotifyURL, payload)
		if lastErr == nil {
			log.Printf("✅ [代收回调]成功通知商户信息,商户号: %v,商户名称: %v,交易订单号: %v,平台订单号: %v,商户订单号: %v, 通知次数: %d", merchant.AppId, merchant.NickName, mOrderIdNum, order.OrderID, order.MOrderID, i)
			return nil
		}
		notifyMsg := fmt.Sprintf("[代收回调]失败通知商户信息\n商户号: %v\n商户名称: %v\n交易订单号: %v\n平台订单号: %v\n商户订单号: %v\n(通知次数: %d/%d)\n错误: %v", merchant.AppId, merchant.NickName, mOrderIdNum, order.OrderID, order.MOrderID, i, receiveMaxRetry, lastErr)
		notify.Notify(system.BotChatID, "warn", "代收回调商户",
			notifyMsg, true)
		log.Printf(notifyMsg)
		time.Sleep(time.Duration(i*2) * time.Second)
	}
	notifyMsg := fmt.Sprintf("[代收回调]失败通知商户信息\n商户号: %v\n商户名称: %v\n交易订单号: %v\n平台订单号: %v\n商户订单号: %v\n(通知次数: %d)\n错误: %v", merchant.AppId, merchant.NickName, mOrderIdNum, order.OrderID, order.MOrderID, receiveMaxRetry, lastErr)
	notify.Notify(system.BotChatID, "warn", "代收回调商户",
		notifyMsg, true)
	return fmt.Errorf(notifyMsg)
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

// decodeIfBase64 自动检测并解码 Base64 响应
func decodeIfBase64(s string) string {
	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return s
	}
	return string(data)
}

// notifyMerchantCallback 通用通知封装
func notifyMerchantCallback(level, title string, payload dto.ReceiveNotifyMerchantPayload, url, desc, req, resp string, merchantTitle string) {
	text := fmt.Sprintf(
		"商户名称: %s\n商户号: %s\n订单号: %s\n回调地址: %s\n描述: %s\n请求参数:\n%s\n响应参数:\n%s",
		merchantTitle,
		payload.MerchantNo,
		payload.TranFlow,
		url,
		desc,
		req,
		resp,
	)
	notify.Notify(system.BotChatID, level, title, text, true)
}

// 通知商户并检查响应
// ReceiveCallback 通知商户并检查响应
func (s *ReceiveCallback) receiveNotifyMerchant(merchantTitle string, url string, payload dto.ReceiveNotifyMerchantPayload) error {
	// 构建 JSON
	body, err := json.Marshal(payload)
	if err != nil {
		notifyMerchantCallback("warn", "[代收回调商户] 序列化失败", payload, url, err.Error(), utils.MapToJSON(payload), "", merchantTitle)
		return fmt.Errorf("[代收回调] failed to marshal payload: %w", err)
	}

	log.Printf("[代收回调] >>商户名称: %v,商户号: %v,通知地址: %v,回调下游商户参数: %s", merchantTitle, payload.MerchantNo, url, string(body))

	// 带超时的 HTTP 客户端
	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewBuffer(body))
	if err != nil {
		notifyMerchantCallback("warn", "[代收回调商户] 创建请求失败", payload, url, err.Error(), string(body), "", merchantTitle)
		return fmt.Errorf("[代收回调] new request error: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		notifyMerchantCallback("warn", "[代收回调商户] 请求失败", payload, url, err.Error(), string(body), "", merchantTitle)
		return fmt.Errorf("[代收回调] send error: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)

	respBody, _ := io.ReadAll(resp.Body)
	respStr := strings.ToLower(strings.TrimSpace(string(respBody)))
	respStr = decodeIfBase64(respStr)
	if respStr == "" {
		respStr = "empty response"
	}

	if resp.StatusCode != http.StatusOK {
		notifyMerchantCallback("warn", "[代收回调商户] HTTP状态异常", payload, url,
			fmt.Sprintf("HTTP状态: %d", resp.StatusCode),
			string(body), respStr, merchantTitle)
		return fmt.Errorf("[代收回调] merchant returned %d: %s", resp.StatusCode, respStr)
	}

	// 判断响应内容
	if respStr != "ok" && respStr != "success" {
		orderErr := s.receiveUpdateMerchantOrder(payload.PaySerialNo, 2, payload.Status)
		if orderErr != nil {
			notifyMerchantCallback("warn", "[代收回调商户] 订单状态更新失败", payload, url,
				orderErr.Error(), string(body), respStr, merchantTitle)
			return fmt.Errorf("[代收回调] merchant response invalid: %s", respStr)
		}
		notifyMerchantCallback("warn", "[代收回调商户] 响应无效", payload, url,
			fmt.Sprintf("返回内容无效: %s", respStr),
			string(body), respStr, merchantTitle)
		return fmt.Errorf("[代收回调] invalid merchant response: %s", respStr)
	}

	// 回调成功
	if err := s.receiveUpdateMerchantOrder(payload.PaySerialNo, 1, payload.Status); err != nil {
		notifyMerchantCallback("warn", "[代收回调商户] 状态更新异常", payload, url, err.Error(), string(body), respStr, merchantTitle)
		return fmt.Errorf("[代收回调] update merchant order failed: %v", err)
	}

	log.Printf("[代收回调] ✅ 通知下游商户成功, 商户号: %v, 订单号: %v", payload.MerchantNo, payload.TranFlow)
	return nil
}

// 更新商户订单信息
func (s *ReceiveCallback) receiveUpdateMerchantOrder(orderId string, notifyStatus int8, orderStatus string) error {

	id, err := strconv.ParseUint(orderId, 10, 64)
	if err != nil {
		return fmt.Errorf("[代收回调] invalid id  with MOrderID %v: %w", orderId, err)
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
		return fmt.Errorf("[代收回调] notify update merchant order failed with MOrderID %v: %w", id, err)
	}

	return nil
}

func (s *ReceiveCallback) receiveConvertStatus(hyperFStatus string) string {
	switch hyperFStatus {
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

// 状态值转化成文字说明
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
		"merchant_no":   p.MerchantNo,
	}
	return utils.GenerateSign(signStr, apiKey)
}
