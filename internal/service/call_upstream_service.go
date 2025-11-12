package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/shopspring/decimal"
	"log"
	"net/url"
	"strconv"
	"strings"
	"wht-order-api/internal/config"
	"wht-order-api/internal/dto"
	"wht-order-api/internal/notify"
	"wht-order-api/internal/utils"
)

// CallUpstreamReceiveService 调用上游服务下单 - 代收
func CallUpstreamReceiveService(ctx context.Context, req dto.UpstreamRequest, mchReq *dto.CreateOrderReq) (string, string, string, error) {
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
		notify.NotifyUpstreamAlert("warn", "代收上游不可用", upstreamUrl, mchReq, params, nil, map[string]string{
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
		notify.NotifyUpstreamAlert("error", "代收上游请求失败(重试后仍失败)", upstreamUrl, mchReq, params, resp, map[string]string{
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
		notify.NotifyUpstreamAlert("error", "代收上游响应解析失败", upstreamUrl, mchReq, params, resp, map[string]string{
			"错误": respErr.Error(),
		})
		return "", "", "", fmt.Errorf("上游响应解析失败")
	}

	// ✅ 判断响应成功
	if !isSuccessCode(string(response.Code)) || string(response.Data.Code) != "0" {
		log.Printf("[Upstream-Receive] 上游返回错误: data.code=%s, data.msg=%s", response.Data.Code, response.Data.Msg)
		notify.NotifyUpstreamAlert("warn", "代收上游交易错误", upstreamUrl, mchReq, params, response, map[string]string{
			"上游Code": string(response.Data.Code),
			"上游Msg":  fmt.Sprintf("%v", response.Data.Msg),
		})
		return "", "", "", fmt.Errorf("交易失败")
	}

	if response.Data.PayUrl == "" || !isValidURL(response.Data.PayUrl) {
		log.Printf("[Upstream-Receive] 上游返回错误: payUrl 无效")
		notify.NotifyUpstreamAlert("warn", "代收上游返回无效支付链接", upstreamUrl, mchReq, params, response, nil)
		return "", "", "", fmt.Errorf("交易失败")
	}

	log.Printf("[Upstream-Receive] 收单下单成功, upOrderNo=%s, payUrl=%s, mOrderId=%s",
		response.Data.UpOrderNo, response.Data.PayUrl, response.Data.MOrderId)

	return response.Data.MOrderId, response.Data.UpOrderNo, response.Data.PayUrl, nil
}

// CallUpstreamPayoutService 调用上游服务下单 - 代付
func CallUpstreamPayoutService(ctx context.Context, req dto.UpstreamRequest, mchReq *dto.CreatePayoutOrderReq) (string, string, string, error) {
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
		"network":           req.Network,           //区块链网络
	}

	upstreamUrl := config.C.Upstream.PayoutApiUrl
	log.Printf("[Upstream-Payout] 请求地址: %s", upstreamUrl)
	log.Printf("[Upstream-Payout] 请求参数: %+v", params)

	// ✅ 健康检查
	if err := utils.CheckUpstreamHealth(ctxTimeout, upstreamUrl); err != nil {
		log.Printf("[Upstream-Payout] 健康检查失败: %v", err)
		notify.NotifyUpstreamAlert("warn", "代付上游不可用", upstreamUrl, mchReq, params, nil, map[string]string{
			"错误": err.Error(),
		})
		return "", "", "", fmt.Errorf("上游服务不可用")
	}

	// ✅ 查询上游余额
	balance, queryErr := CheckUpstreamBalance(ctxTimeout, req, mchReq)
	if queryErr != nil {
		log.Printf("[Upstream-Payout] ❌ 查询上游余额失败: %v", queryErr)
		notify.NotifyUpstreamAlert("error", "代付上游余额查询失败", req.QueryUrl, mchReq, req, nil, map[string]string{
			"错误": queryErr.Error(),
		})
		return "", "", "", fmt.Errorf("查询上游余额失败: %v", queryErr)
	}

	log.Printf("[Upstream-Payout] 上游余额: %v, 代付金额: %v", balance, req.Amount)
	if !balanceGreaterThanOrder(req.Amount, balance) {
		log.Printf("[Upstream-Payout] ⚠️ 上游余额不足，跳过下单")
		notify.NotifyUpstreamAlert("warn", "代付上游余额不足", req.QueryUrl, mchReq, req, nil, map[string]string{
			"上游余额": fmt.Sprintf("%v", balance),
			"代付金额": fmt.Sprintf("%v", req.Amount),
		})
		return "", "", "", fmt.Errorf("上游余额不足")
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
		notify.NotifyUpstreamAlert("error", "代付上游请求失败(重试后仍失败)", upstreamUrl, mchReq, params, resp, map[string]string{
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
		notify.NotifyUpstreamAlert("error", "代付上游响应解析失败", upstreamUrl, mchReq, params, resp, map[string]string{
			"错误": err.Error(),
		})
		return "", "", "", fmt.Errorf("上游响应解析失败")
	}

	// ✅ 响应检查
	if !isSuccessCode(string(response.Code)) || string(response.Data.Code) != "0" {
		log.Printf("[Upstream-Payout] 上游返回错误: code=%v, msg=%s", response.Code, response.Msg)
		notify.NotifyUpstreamAlert("warn", "代付上游交易错误", upstreamUrl, mchReq, params, response, map[string]string{
			"上游Code": string(response.Data.Code),
			"上游Msg":  fmt.Sprintf("%v", response.Data.Msg),
		})
		return "", "", "", fmt.Errorf("交易失败")
	}

	log.Printf("[Upstream-Payout] 代付下单成功, upOrderNo=%s, mOrderId=%s, status=%s",
		response.Data.UpOrderNo, response.Data.MOrderId, response.Data.Status)

	return response.Data.MOrderId, response.Data.UpOrderNo, response.Data.PayUrl, nil
}

// CheckUpstreamBalance 查询上游余额接口
func CheckUpstreamBalance(ctx context.Context, req dto.UpstreamRequest, mchReq *dto.CreatePayoutOrderReq) (decimal.Decimal, error) {
	upstreamBalanceUrl := config.C.Upstream.BalanceApiUrl
	if upstreamBalanceUrl == "" {
		return decimal.Zero, fmt.Errorf("未配置上游余额查询地址")
	}

	params := map[string]interface{}{
		"mchNo":       req.MchNo,
		"apiKey":      req.ApiKey,
		"providerKey": req.ProviderKey,
		"mode":        "balance",
		"currency":    req.Currency,
		"payMethod":   req.PayMethod,
	}

	resp, err := utils.HttpPostJsonWithContext(ctx, upstreamBalanceUrl, params)
	if err != nil {
		return decimal.Zero, fmt.Errorf("请求上游余额接口失败: %v", err)
	}

	var result struct {
		Code utils.StringOrNumber `json:"code"`
		Msg  utils.FlexibleMsg    `json:"msg"`
		Data struct {
			Amount          string               `json:"amount"`
			FrozenAmount    string               `json:"frozenAmount"`
			AvailableAmount string               `json:"availableAmount"`
			Code            utils.StringOrNumber `json:"code"` // code编码 实际判断的字段
			Msg             utils.FlexibleMsg    `json:"msg"`  // 上游返回错误信息
		} `json:"data"`
	}

	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return decimal.Zero, fmt.Errorf("解析上游余额响应失败: %v", err)
	}

	if !isSuccessCode(string(result.Code)) || string(result.Data.Code) != "0" {
		return decimal.Zero, fmt.Errorf("上游返回错误: %v", result.Msg)
	}
	amount, err := decimal.NewFromString(result.Data.AvailableAmount)
	if err != nil {
		log.Printf("[Upstream-Query-Balance] 查询上游商户余额解析错误: amount=%v, msg=%v", amount, err)
		notify.NotifyUpstreamAlert("warn", "查询上游商户余额解析错误", upstreamBalanceUrl, mchReq, params, resp, map[string]string{
			"上游Code": string(result.Data.Code),
			"上游Msg":  fmt.Sprintf("%v", result.Data.Msg),
		})
		return decimal.Zero, fmt.Errorf("查询上游商户余额解析错误:%w", err)
	}

	return amount, nil
}

// 校验上游商户余额是否大于等于订单金额
func balanceGreaterThanOrder(orderAmount string, upstreamBalance decimal.Decimal) bool {
	amount, err := decimal.NewFromString(orderAmount)
	if err != nil {
		// 可记录日志或返回 false / panic，根据业务需求
		log.Printf("提交上游交易时比较商户余额与订单金额时，订单金额转化错误: %v", err)
		return false
	}
	return upstreamBalance.GreaterThanOrEqual(amount)
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
