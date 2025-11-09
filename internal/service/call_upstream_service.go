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
	"wht-order-api/internal/utils"
)

// CallUpstreamReceiveService 调用上游服务下单 - 代收
func CallUpstreamReceiveService(ctx context.Context, req dto.UpstreamRequest) (string, string, string, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, config.C.Upstream.Timeout.Receive)
	defer cancel()

	params := map[string]interface{}{
		"mchNo":             req.MchNo,
		"amount":            req.Amount,
		"currency":          req.Currency,
		"returnUrl":         req.RedirectUrl,
		"payType":           req.UpstreamCode,  // 注意：这是上游通道编码
		"upstreamTitle":     req.UpstreamTitle, // 上游供应商名称
		"mchOrderId":        req.MchOrderId,
		"productInfo":       req.ProductInfo,
		"apiKey":            req.ApiKey,
		"providerKey":       req.ProviderKey,
		"accNo":             req.AccNo,
		"accName":           req.AccName,
		"payEmail":          req.PayEmail,
		"payPhone":          req.PayPhone,
		"bankCode":          req.BankCode,
		"bankName":          req.BankName,
		"payMethod":         req.PayMethod,
		"identityType":      req.IdentityType,
		"identityNum":       req.IdentityNum,
		"mode":              req.Mode,
		"clientIp":          req.ClientIp,
		"notifyUrl":         req.NotifyUrl,         // 添加通知URL
		"submitUrl":         req.SubmitUrl,         // 下单URL
		"queryUrl":          req.QueryUrl,          // 查单URL
		"downstreamOrderNo": req.DownstreamOrderNo, //下游商户订单号
	}

	upstreamUrl := config.C.Upstream.ReceiveApiUrl
	log.Printf("[Upstream-Receive] 请求地址: %s, 请求参数: %+v", upstreamUrl, params)

	// ✅ 健康检查
	if err := utils.CheckUpstreamHealth(ctxTimeout, upstreamUrl); err != nil {
		log.Printf("[Upstream-Receive] 健康检查失败: %v", err)
		notify.NotifyUpstreamAlert("warn", "代收上游不可用", upstreamUrl, req, nil, map[string]string{
			"错误": err.Error(),
		})
		return "", "", "", fmt.Errorf("上游服务不可用")
	}

	// ✅ 带重试逻辑
	var resp string
	err := utils.DoWithRetry(ctxTimeout, config.C.Upstream.Retry.Times, config.C.Upstream.Retry.Interval, func() error {
		r, err := utils.HttpPostJsonWithContext(ctxTimeout, upstreamUrl, params)
		if err != nil {
			return err
		}
		resp = r
		return nil
	})
	if err != nil {
		log.Printf("[Upstream-Receive] 请求失败(重试后仍失败): %v", err)
		notify.NotifyUpstreamAlert("error", "代收上游请求失败(重试后仍失败)", upstreamUrl, req, resp, map[string]string{
			"错误":   err.Error(),
			"重试次数": strconv.Itoa(config.C.Upstream.Retry.Times),
		})
		return "", "", "", fmt.Errorf("请求上游失败")
	}

	log.Printf("[Upstream-Receive] 响应原始数据: %s", resp)

	// ✅ JSON解析
	var response struct {
		Code utils.StringOrNumber `json:"code"` // 顶层code（无用）
		Msg  utils.FlexibleMsg    `json:"msg"`
		Data struct {
			Code      utils.StringOrNumber `json:"code"` //实际判断的字段
			Msg       utils.FlexibleMsg    `json:"msg"`
			UpOrderNo string               `json:"up_order_no"`
			PayUrl    string               `json:"pay_url"`
			MOrderId  string               `json:"m_order_id"`
		} `json:"data"`
	}

	if respErr := json.Unmarshal([]byte(resp), &response); respErr != nil {
		log.Printf("[Upstream-Receive] JSON解析失败: %v", respErr)
		notify.NotifyUpstreamAlert("error", "代收上游响应解析失败", upstreamUrl, req, resp, map[string]string{
			"错误": respErr.Error(),
		})
		return "", "", "", fmt.Errorf("上游响应解析失败")
	}

	// ✅ 判断响应成功
	if !isSuccessCode(string(response.Code)) || string(response.Data.Code) != "0" {
		log.Printf("[Upstream-Receive] 上游返回错误: data.code=%s, data.msg=%s", response.Data.Code, response.Data.Msg)
		notify.NotifyUpstreamAlert("warn", "代收上游交易错误", upstreamUrl, req, response, map[string]string{
			"上游Code": string(response.Data.Code),
			"上游Msg":  fmt.Sprintf("%v", response.Data.Msg),
		})
		return "", "", "", fmt.Errorf("交易失败")
	}

	if response.Data.PayUrl == "" || !isValidURL(response.Data.PayUrl) {
		log.Printf("[Upstream-Receive] 上游返回错误: payUrl 无效")
		notify.NotifyUpstreamAlert("warn", "代收上游返回无效支付链接", upstreamUrl, req, response, nil)
		return "", "", "", fmt.Errorf("交易失败")
	}

	log.Printf("[Upstream-Receive] 收单下单成功, upOrderNo=%s, payUrl=%s, mOrderId=%s",
		response.Data.UpOrderNo, response.Data.PayUrl, response.Data.MOrderId)

	return response.Data.MOrderId, response.Data.UpOrderNo, response.Data.PayUrl, nil
}

// CallUpstreamPayoutService 调用上游服务下单 - 代付
func CallUpstreamPayoutService(ctx context.Context, req dto.UpstreamRequest, merchantId uint64, order *orderModel.MerchantPayOutOrderM) (string, string, string, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, config.C.Upstream.Timeout.Payout)
	defer cancel()

	params := map[string]interface{}{
		"mchNo":             req.MchNo,
		"amount":            req.Amount,
		"currency":          req.Currency,
		"returnUrl":         req.RedirectUrl,
		"payType":           req.UpstreamCode,  // 使用通道的上游编码
		"upstreamTitle":     req.UpstreamTitle, // 上游供应商名称
		"mchOrderId":        req.MchOrderId,
		"productInfo":       req.ProductInfo,
		"apiKey":            req.ApiKey,
		"providerKey":       req.ProviderKey,
		"accNo":             req.AccNo,
		"accName":           req.AccName,
		"payEmail":          req.PayEmail,
		"payPhone":          req.PayPhone,
		"bankCode":          req.BankCode,
		"bankName":          req.BankName,
		"payMethod":         req.PayMethod,
		"identityType":      req.IdentityType,
		"identityNum":       req.IdentityNum,
		"mode":              req.Mode,
		"notifyUrl":         req.NotifyUrl,
		"submitUrl":         req.SubmitUrl,
		"queryUrl":          req.QueryUrl,
		"clientIp":          req.ClientIp,
		"accountType":       req.AccountType,       //账户类型
		"cciNo":             req.CciNo,             //银行间账户号
		"address":           req.Address,           //客户地址
		"downstreamOrderNo": req.DownstreamOrderNo, //下游商户订单号
	}

	upstreamUrl := config.C.Upstream.PayoutApiUrl
	log.Printf("[Upstream-Payout] 请求地址: %s", upstreamUrl)
	log.Printf("[Upstream-Payout] 请求参数: %+v", params)

	// ✅ 健康检查
	if err := utils.CheckUpstreamHealth(ctxTimeout, upstreamUrl); err != nil {
		log.Printf("[Upstream-Payout] 健康检查失败: %v", err)
		notify.NotifyUpstreamAlert("warn", "代付上游不可用", upstreamUrl, req, nil, map[string]string{
			"错误": err.Error(),
		})
		return "", "", "", fmt.Errorf("上游服务不可用")
	}

	// ✅ 带重试逻辑
	var resp string
	err := utils.DoWithRetry(ctxTimeout, config.C.Upstream.Retry.Times, config.C.Upstream.Retry.Interval, func() error {
		r, e := utils.HttpPostJsonWithContext(ctxTimeout, upstreamUrl, params)
		if e != nil {
			return e
		}
		resp = r
		return nil
	})
	if err != nil {
		log.Printf("[Upstream-Payout] 请求失败(重试后仍失败): %v", err)
		notify.NotifyUpstreamAlert("error", "代付上游请求失败(重试后仍失败)", upstreamUrl, req, resp, map[string]string{
			"错误":   err.Error(),
			"重试次数": strconv.Itoa(config.C.Upstream.Retry.Times),
		})
		return "", "", "", fmt.Errorf("请求上游失败")
	}

	log.Printf("[Upstream-Payout] 响应原始数据: %s", resp)

	// ✅ JSON解析
	var response struct {
		Code utils.StringOrNumber `json:"code"` //顶层code（无用）
		Msg  utils.FlexibleMsg    `json:"msg"`
		Data struct {
			UpOrderNo string               `json:"up_order_no"`
			PayUrl    string               `json:"pay_url"`
			MOrderId  string               `json:"m_order_id"`
			Status    string               `json:"status"`   // 代付可能有状态返回
			Fee       string               `json:"fee"`      // 代付手续费
			TradeNo   string               `json:"trade_no"` // 交易号
			Code      utils.StringOrNumber `json:"code"`     // code编码 实际判断的字段
			Msg       utils.FlexibleMsg    `json:"msg"`      // 上游返回错误信息
		} `json:"data"`
	}

	if err := json.Unmarshal([]byte(resp), &response); err != nil {
		log.Printf("[Upstream-Payout] JSON解析失败: %v", err)
		notify.NotifyUpstreamAlert("error", "代付上游响应解析失败", upstreamUrl, req, resp, map[string]string{
			"错误": err.Error(),
		})
		return "", "", "", fmt.Errorf("上游响应解析失败")
	}

	// ✅ 响应检查
	if !isSuccessCode(string(response.Code)) || string(response.Data.Code) != "0" {
		log.Printf("[Upstream-Payout] 上游返回错误: code=%v, msg=%s", response.Code, response.Msg)

		return "", "", "", fmt.Errorf("交易失败")
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
		order.MOrderID,
		isSuccess,
		order.Amount,
	); err != nil {
		return fmt.Errorf("[ROLLBACK-PAYOUT] 回滚失败 OrderID=%v, err=%w", order.OrderID, err)
	}
	return nil
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

// isValidURL 校验是否为合法 URL
func isValidURL(u string) bool {
	parsed, err := url.ParseRequestURI(u)
	return err == nil && strings.HasPrefix(parsed.Scheme, "http")
}
