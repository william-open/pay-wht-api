package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"wht-order-api/internal/config"
	"wht-order-api/internal/dto"
	orderModel "wht-order-api/internal/model/order"
	"wht-order-api/internal/notify"
	"wht-order-api/internal/settlement"
	"wht-order-api/internal/system"
	"wht-order-api/internal/utils"
)

// CallUpstreamReceiveService 调用 PHP 服务下单-代收 调用上游服务下单-代收（支持上下文超时控制）
func CallUpstreamReceiveService(ctx context.Context, req dto.UpstreamRequest) (string, string, string, error) {
	// 组装请求参数
	params := map[string]interface{}{
		"mchNo":       req.MchNo,
		"amount":      req.Amount,
		"currency":    req.Currency,
		"returnUrl":   req.RedirectUrl,
		"payType":     req.UpstreamCode, // 注意：这是上游通道编码
		"mchOrderId":  req.MchOrderId,
		"productInfo": req.ProductInfo,
		"apiKey":      req.ApiKey,
		"providerKey": req.ProviderKey,
		"accNo":       req.AccNo,
		"accName":     req.AccName,
		"payEmail":    req.PayEmail,
		"payPhone":    req.PayPhone,
		"bankCode":    req.BankCode,
		"bankName":    req.BankName,
		"payMethod":   req.PayMethod,
		"mode":        req.Mode,
		"clientIp":    req.ClientIp,
		"notifyUrl":   req.NotifyUrl, // 添加通知URL
		"submitUrl":   req.SubmitUrl, // 下单URL
		"queryUrl":    req.QueryUrl,  // 查单URL

	}

	upstreamUrl := config.C.Upstream.ReceiveApiUrl
	log.Printf("[Upstream-Receive] 请求地址: %s,请求参数: %+v", upstreamUrl, params)

	// ✅ 检测上游是否可访问（带超时）
	if err := utils.CheckUpstreamHealth(ctx, upstreamUrl); err != nil {
		log.Printf("[Upstream-Receive] 健康检查失败: %v", err)
		return "", "", "", fmt.Errorf("上游服务不可用: %v", err)
	}

	// ✅ 发起请求（带上下文和超时控制）
	resp, err := utils.HttpPostJsonWithContext(ctx, upstreamUrl, params)
	if err != nil {
		log.Printf("[Upstream-Receive] 请求失败: %v", err)
		return "", "", "", fmt.Errorf("请求上游失败: %v", err)
	}
	log.Printf("[Upstream-Receive] 响应原始数据: %s", resp)

	// ✅ 定义响应结构
	var response struct {
		Code utils.StringOrNumber `json:"code"` // 顶层code（无用）
		Msg  utils.FlexibleMsg    `json:"msg"`
		Data struct {
			Code      utils.StringOrNumber `json:"code"` // ✅ 实际判断的字段
			Msg       utils.FlexibleMsg    `json:"msg"`
			UpOrderNo string               `json:"up_order_no"`
			PayUrl    string               `json:"pay_url"`
			MOrderId  string               `json:"m_order_id"`
		} `json:"data"`
	}

	// ✅ JSON解析
	if respErr := json.Unmarshal([]byte(resp), &response); respErr != nil {
		log.Printf("[Upstream-Receive] JSON解析失败: %v, 原始响应: %s", respErr.Error(), resp)
		return "", "", "", fmt.Errorf("响应解析失败: %v", respErr.Error())
	}

	// ✅ 只认 data.code == "0" 成功
	if string(response.Code) != "0" {
		log.Printf("[Upstream-Receive] 上游返回错误: data.code=%s, data.msg=%s", response.Data.Code, response.Data.Msg)
		return "", "", "", fmt.Errorf("上游错误[%s]: %s", response.Data.Code, response.Data.Msg)
	}
	if response.Data.PayUrl == "" && !isValidURL(response.Data.PayUrl) {
		log.Printf("[Upstream-Receive] 上游返回错误: data.code=%s, data.msg=%s", response.Data.Code, response.Data.Msg)
		return "", "", "", fmt.Errorf("上游错误[%s]: %s", response.Data.Code, response.Data.Msg)
	}
	// ✅ 成功日志
	log.Printf("[Upstream-Receive] 收单下单成功, upOrderNo=%s, payUrl=%s, mOrderId=%s",
		response.Data.UpOrderNo, response.Data.PayUrl, response.Data.MOrderId)

	return response.Data.MOrderId, response.Data.UpOrderNo, response.Data.PayUrl, nil
}

// isSuccessCode 检查响应码是否为成功（支持字符串和数字类型）
func isSuccessCode(code interface{}) bool {
	switch v := code.(type) {
	case string:
		return v == "0" || v == "0000" || v == "success" || v == "SUCCESS"
	case int:
		return v == 0 || v == 200
	case float64:
		return v == 0 || v == 200
	default:
		return false
	}
}

// Go 内置的 net/url 包校验是否是合法 URL
func isValidURL(u string) bool {
	parsed, err := url.ParseRequestURI(u)
	return err == nil && (strings.HasPrefix(parsed.Scheme, "http"))
}

// CallUpstreamPayoutService 调用 PHP 服务下单 -代付
// CallUpstreamPayoutService 调用上游服务下单-代付（支持上下文超时控制）
func CallUpstreamPayoutService(ctx context.Context, req dto.UpstreamRequest, merchantId uint64, order *orderModel.MerchantPayOutOrderM) (string, string, string, error) {
	// 组装请求参数
	params := map[string]interface{}{
		"mchNo":        req.MchNo,
		"amount":       req.Amount,
		"currency":     req.Currency,
		"returnUrl":    req.RedirectUrl,
		"payType":      req.UpstreamCode, // 使用通道的上游编码
		"mchOrderId":   req.MchOrderId,
		"productInfo":  req.ProductInfo,
		"apiKey":       req.ApiKey,
		"providerKey":  req.ProviderKey,
		"accNo":        req.AccNo,
		"accName":      req.AccName,
		"payEmail":     req.PayEmail,
		"payPhone":     req.PayPhone,
		"bankCode":     req.BankCode,
		"bankName":     req.BankName,
		"payMethod":    req.PayMethod,
		"identityType": req.IdentityType,
		"identityNum":  req.IdentityNum,
		"mode":         req.Mode,
		"notifyUrl":    req.NotifyUrl, // 添加通知URL
		"submitUrl":    req.SubmitUrl, // 下单URL
		"queryUrl":     req.QueryUrl,  // 查单URL
	}

	upstreamUrl := config.C.Upstream.PayoutApiUrl
	log.Printf("[Upstream-Payout] 请求地址: %s", upstreamUrl)
	log.Printf("[Upstream-Payout] 请求参数: %+v", params)

	// ✅ 检测上游是否可访问（带超时）
	if err := utils.CheckUpstreamHealth(ctx, upstreamUrl); err != nil {
		log.Printf("[Upstream-Payout] 健康检查失败: %v", err)
		return "", "", "", fmt.Errorf("上游服务不可用: %v", err)
	}

	// ✅ 发起请求（带上下文和超时控制）
	resp, err := utils.HttpPostJsonWithContext(ctx, upstreamUrl, params)
	if err != nil {
		log.Printf("[Upstream-Payout] 请求失败: %v", err)
		notify.Notify(system.BotChatID, "warn", "代付下单失败", fmt.Sprintf("[代付]上游商户: %s, 调用上游编码[%s]失败:%s,请求参数:%s", req.MchNo, req.UpstreamCode, err, utils.MapToJSON(req)), true)
		return "", "", "", fmt.Errorf("请求上游失败: %v", err)
	}
	log.Printf("[Upstream-Payout] 响应原始数据: %s", resp)

	// ✅ 解析响应
	var response struct {
		Code utils.StringOrNumber `json:"code"` // 使用interface{}因为上游可能返回字符串或数字
		Msg  utils.FlexibleMsg    `json:"msg"`
		Data struct {
			UpOrderNo string               `json:"up_order_no"`
			PayUrl    string               `json:"pay_url"`
			MOrderId  string               `json:"m_order_id"`
			Status    string               `json:"status"`   // 代付可能有状态返回
			Fee       string               `json:"fee"`      // 代付手续费
			TradeNo   string               `json:"trade_no"` // 交易号
			Code      utils.StringOrNumber `json:"code"`     // code编码
			Msg       utils.FlexibleMsg    `json:"msg"`      // 上游返回错误信息
		} `json:"data"`
	}

	if err := json.Unmarshal([]byte(resp), &response); err != nil {
		log.Printf("[Upstream-Payout] JSON解析失败: %v, 原始响应: %s", err, resp)
		return "", "", "", fmt.Errorf("响应解析失败: %v", err)
	}

	// 检查响应码（支持字符串和数字类型）
	if !isSuccessCode(response.Code) {
		log.Printf("[Upstream-Payout] 上游返回错误: code=%v, msg=%s", response.Code, response.Msg)
		rollbackErr := rollbackPayoutAmount(strconv.FormatUint(merchantId, 10), order, false)
		if rollbackErr != nil {
			return "", "", "", rollbackErr
		}
		return "", "", "", fmt.Errorf("上游错误[%v]: %s", response.Code, response.Msg)
	}
	// ✅ 只认 data.code == "0" 成功
	log.Printf("[Upstream-Payout]上游供应商返回code: %s", response.Data.Code)
	if string(response.Code) != "0" || string(response.Data.Code) != "0" {
		log.Printf("[Upstream-Payout] 上游返回错误: code=%v, msg=%s", response.Code, response.Msg)
		rollbackErr := rollbackPayoutAmount(strconv.FormatUint(merchantId, 10), order, false)
		if rollbackErr != nil {
			return "", "", "", rollbackErr
		}
		return "", "", "", fmt.Errorf("上游错误[%v]: %v", response.Code, response.Data.Msg)
	}

	log.Printf("[Upstream-Payout] 代付下单成功, upOrderNo=%s, mOrderId=%s, status=%s",
		response.Data.UpOrderNo, response.Data.MOrderId, response.Data.Status)

	return response.Data.MOrderId, response.Data.UpOrderNo, response.Data.PayUrl, nil
}

// 代付失败，回滚资金
func rollbackPayoutAmount(merchantId string, order *orderModel.MerchantPayOutOrderM, isSuccess bool) error {
	settleService := settlement.NewSettlement()
	settlementResult := dto.SettlementResult(order.SettleSnapshot)
	if err := settleService.DoPayoutSettlement(settlementResult,
		merchantId,
		order.OrderID,
		isSuccess,
		order.Amount,
	); err != nil {
		return fmt.Errorf("[ROLLBACK-PAYOUT] 回滚失败 OrderID=%v, err=%w", order.OrderID, err)
	}
	return nil
}
