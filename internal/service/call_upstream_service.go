package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"
	"wht-order-api/internal/config"
	"wht-order-api/internal/dto"
	orderModel "wht-order-api/internal/model/order"
	"wht-order-api/internal/notify"
	"wht-order-api/internal/settlement"
	"wht-order-api/internal/system"
	"wht-order-api/internal/utils"
)

// ===============================
// 公共工具方法
// ===============================

// isSuccessCode 判断 code 是否为成功状态
func isSuccessCode(code interface{}) bool {
	switch v := code.(type) {
	case string:
		return v == "0" || v == "0000" || strings.EqualFold(v, "success")
	case int:
		return v == 0 || v == 200
	case float64:
		return v == 0 || v == 200
	default:
		return false
	}
}

// isValidURL 检查 URL 是否有效
func isValidURL(u string) bool {
	parsed, err := url.ParseRequestURI(u)
	return err == nil && (strings.HasPrefix(parsed.Scheme, "http"))
}

// 安全日志过滤函数
func safeParams(params map[string]interface{}) map[string]interface{} {
	clone := make(map[string]interface{})
	for k, v := range params {
		if strings.Contains(strings.ToLower(k), "key") || strings.Contains(strings.ToLower(k), "acc") {
			clone[k] = "***"
		} else {
			clone[k] = v
		}
	}
	return clone
}

// ===============================
// 代收调用
// ===============================

func CallUpstreamReceiveService(ctx context.Context, req dto.UpstreamRequest) (string, string, string, error) {
	upstreamUrl := config.C.Upstream.ReceiveApiUrl
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	params := map[string]interface{}{
		"mchNo": req.MchNo, "amount": req.Amount, "currency": req.Currency,
		"returnUrl": req.RedirectUrl, "payType": req.UpstreamCode, "mchOrderId": req.MchOrderId,
		"productInfo": req.ProductInfo, "apiKey": req.ApiKey, "providerKey": req.ProviderKey,
		"accNo": req.AccNo, "accName": req.AccName, "payEmail": req.PayEmail, "payPhone": req.PayPhone,
		"bankCode": req.BankCode, "bankName": req.BankName, "payMethod": req.PayMethod,
		"identityType": req.IdentityType, "identityNum": req.IdentityNum, "mode": req.Mode,
		"clientIp": req.ClientIp, "notifyUrl": req.NotifyUrl, "submitUrl": req.SubmitUrl, "queryUrl": req.QueryUrl,
	}

	log.Printf("[Upstream-Receive] 请求地址: %s, 参数: %+v", upstreamUrl, safeParams(params))

	if err := utils.CheckUpstreamHealth(ctx, upstreamUrl); err != nil {
		notify.Notify(system.BotChatID, "warn", "代收上游健康检查失败",
			fmt.Sprintf("上游接口不可用: %s", upstreamUrl), true)
		return "", "", "", errors.New("上游服务暂时不可用，请稍后重试")
	}

	resp, err := utils.HttpPostJsonWithContext(ctx, upstreamUrl, params)
	if err != nil {
		log.Printf("[Upstream-Receive] 请求失败: %v", err)
		notify.Notify(system.BotChatID, "warn", "代收请求失败", fmt.Sprintf("上游接口: %s", upstreamUrl), true)
		return "", "", "", errors.New("请求上游失败，请稍后重试")
	}
	log.Printf("[Upstream-Receive] 响应: %s", resp)

	var response struct {
		Code utils.StringOrNumber `json:"code"`
		Msg  utils.FlexibleMsg    `json:"msg"`
		Data struct {
			Code      utils.StringOrNumber `json:"code"`
			Msg       utils.FlexibleMsg    `json:"msg"`
			UpOrderNo string               `json:"up_order_no"`
			PayUrl    string               `json:"pay_url"`
			MOrderId  string               `json:"m_order_id"`
		} `json:"data"`
	}

	if err := json.Unmarshal([]byte(resp), &response); err != nil {
		log.Printf("[Upstream-Receive] JSON解析失败: %v", err)
		return "", "", "", errors.New("上游响应异常，请联系管理员")
	}

	if !isSuccessCode(string(response.Data.Code)) {
		notify.Notify(system.BotChatID, "warn", "代收下单失败",
			fmt.Sprintf("上游返回错误: %v, %v", response.Data.Code, response.Data.Msg), true)
		return "", "", "", fmt.Errorf("交易失败: %s", response.Data.Msg)
	}

	if !isValidURL(response.Data.PayUrl) {
		return "", "", "", errors.New("无效的支付链接")
	}

	log.Printf("[Upstream-Receive] 成功: upOrderNo=%s, payUrl=%s", response.Data.UpOrderNo, response.Data.PayUrl)
	return response.Data.MOrderId, response.Data.UpOrderNo, response.Data.PayUrl, nil
}

// ===============================
// 代付调用
// ===============================

func CallUpstreamPayoutService(ctx context.Context, req dto.UpstreamRequest, merchantId uint64, order *orderModel.MerchantPayOutOrderM) (string, string, string, error) {
	upstreamUrl := config.C.Upstream.PayoutApiUrl
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	params := map[string]interface{}{
		"mchNo": req.MchNo, "amount": req.Amount, "currency": req.Currency, "returnUrl": req.RedirectUrl,
		"payType": req.UpstreamCode, "mchOrderId": req.MchOrderId, "productInfo": req.ProductInfo,
		"apiKey": req.ApiKey, "providerKey": req.ProviderKey, "accNo": req.AccNo, "accName": req.AccName,
		"payEmail": req.PayEmail, "payPhone": req.PayPhone, "bankCode": req.BankCode, "bankName": req.BankName,
		"payMethod": req.PayMethod, "identityType": req.IdentityType, "identityNum": req.IdentityNum,
		"mode": req.Mode, "notifyUrl": req.NotifyUrl, "submitUrl": req.SubmitUrl, "queryUrl": req.QueryUrl,
		"clientIp": req.ClientIp, "accountType": req.AccountType, "cciNo": req.CciNo, "address": req.Address,
	}

	log.Printf("[Upstream-Payout] 请求地址: %s, 参数: %+v", upstreamUrl, safeParams(params))

	if err := utils.CheckUpstreamHealth(ctx, upstreamUrl); err != nil {
		notify.Notify(system.BotChatID, "warn", "代付上游健康检查失败",
			fmt.Sprintf("接口: %s", upstreamUrl), true)
		return "", "", "", errors.New("上游服务暂时不可用，请稍后重试")
	}

	resp, err := utils.HttpPostJsonWithContext(ctx, upstreamUrl, params)
	if err != nil {
		log.Printf("[Upstream-Payout] 请求失败: %v", err)
		notify.Notify(system.BotChatID, "warn", "代付请求失败", fmt.Sprintf("上游接口: %s", upstreamUrl), true)
		return "", "", "", errors.New("上游请求异常，请稍后重试")
	}
	log.Printf("[Upstream-Payout] 响应: %s", resp)

	var response struct {
		Code utils.StringOrNumber `json:"code"`
		Msg  utils.FlexibleMsg    `json:"msg"`
		Data struct {
			UpOrderNo string               `json:"up_order_no"`
			PayUrl    string               `json:"pay_url"`
			MOrderId  string               `json:"m_order_id"`
			Status    string               `json:"status"`
			Fee       string               `json:"fee"`
			TradeNo   string               `json:"trade_no"`
			Code      utils.StringOrNumber `json:"code"`
			Msg       utils.FlexibleMsg    `json:"msg"`
		} `json:"data"`
	}

	if err := json.Unmarshal([]byte(resp), &response); err != nil {
		log.Printf("[Upstream-Payout] JSON解析失败: %v", err)
		return "", "", "", errors.New("上游响应异常，请联系管理员")
	}

	if !isSuccessCode(string(response.Code)) || !isSuccessCode(string(response.Data.Code)) {
		log.Printf("[Upstream-Payout] 上游返回错误: code=%v, msg=%s", response.Code, response.Msg)
		if rollbackErr := rollbackPayoutAmount(strconv.FormatUint(merchantId, 10), order, false); rollbackErr != nil {
			log.Printf("[ROLLBACK-PAYOUT] 回滚失败: %v", rollbackErr)
		}
		return "", "", "", fmt.Errorf("交易失败: %v", response.Data.Msg)
	}

	log.Printf("[Upstream-Payout] 成功: upOrderNo=%s, mOrderId=%s, status=%s", response.Data.UpOrderNo, response.Data.MOrderId, response.Data.Status)
	return response.Data.MOrderId, response.Data.UpOrderNo, response.Data.PayUrl, nil
}

// ===============================
// 回滚资金
// ===============================

func rollbackPayoutAmount(merchantId string, order *orderModel.MerchantPayOutOrderM, isSuccess bool) error {
	settleService := settlement.NewSettlement()
	settlementResult := dto.SettlementResult(order.SettleSnapshot)
	if err := settleService.DoPayoutSettlement(settlementResult, merchantId, order.OrderID, isSuccess, order.Amount); err != nil {
		return fmt.Errorf("回滚资金失败，请联系管理员")
	}
	return nil
}
