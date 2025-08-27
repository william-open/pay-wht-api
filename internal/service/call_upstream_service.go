package service

import (
	"encoding/json"
	"fmt"
	"log"
	"wht-order-api/internal/config"
	"wht-order-api/internal/dto"
	"wht-order-api/internal/utils"
)

// CallUpstreamReceiveService 调用 PHP 服务下单-代收
func CallUpstreamReceiveService(req dto.UpstreamRequest, channel *dto.PaymentChannelVo) (string, string, string, error) {
	// 组装请求参数
	params := map[string]interface{}{
		"mchNo":       req.MchNo,
		"amount":      req.Amount,
		"currency":    req.Currency,
		"returnUrl":   req.RedirectUrl,
		"payType":     channel.UpstreamCode,
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
	}

	upstreamUrl := config.C.Upstream.ReceiveApiUrl
	log.Printf("[Upstream] 请求地址: %s", upstreamUrl)
	log.Printf("[Upstream] 请求参数: %+v", params)

	// ✅ 检测上游是否可访问（HEAD 请求）
	if err := utils.CheckUpstreamHealth(upstreamUrl); err != nil {
		log.Printf("[Upstream] 健康检查失败: %v", err)
		return "", "", "", fmt.Errorf("上游服务不可用: %v", err)
	}

	// ✅ 发起请求
	resp, err := utils.HttpPostJson(upstreamUrl, params)
	if err != nil {
		log.Printf("[Upstream] 请求失败: %v", err)
		return "", "", "", fmt.Errorf("请求上游失败: %v", err)
	}
	log.Printf("[Upstream] 响应原始数据: %s", resp)

	// ✅ 解析响应
	var data struct {
		Code interface{} `json:"code"`
		Msg  string      `json:"msg"`
		Data struct {
			UpOrderNo string `json:"up_order_no"`
			PayUrl    string `json:"pay_url"`
			MOrderId  string `json:"m_order_id"`
		} `json:"data"`
	}

	if err := json.Unmarshal([]byte(resp), &data); err != nil {
		log.Printf("[Upstream] JSON解析失败: %v", err)
		return "", "", "", fmt.Errorf("响应解析失败: %v", err)
	}

	if data.Code != string('0') {
		log.Printf("[Upstream] 上游返回错误: %s", data.Msg)
		return "", "", "", fmt.Errorf("上游错误: %s", data.Msg)
	}

	log.Printf("[Upstream] 下单成功,响应数据: upOrderNo=%+v, payUrl=%+v,mOrderId:%+v", data.Data.UpOrderNo, data.Data.PayUrl, data.Data.MOrderId)
	return data.Data.MOrderId, data.Data.UpOrderNo, data.Data.PayUrl, nil
}

// CallUpstreamPayoutService 调用 PHP 服务下单 -代付
func CallUpstreamPayoutService(req dto.UpstreamRequest, channel *dto.PaymentChannelVo) (string, string, string, error) {
	// 组装请求参数
	params := map[string]interface{}{
		"mchNo":        req.MchNo,
		"amount":       req.Amount,
		"currency":     req.Currency,
		"returnUrl":    req.RedirectUrl,
		"payType":      channel.UpstreamCode,
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
	}

	upstreamUrl := config.C.Upstream.PayoutApiUrl
	log.Printf("[Upstream] 请求地址: %s", upstreamUrl)
	log.Printf("[Upstream] 请求参数: %+v", params)

	// ✅ 检测上游是否可访问（HEAD 请求）
	if err := utils.CheckUpstreamHealth(upstreamUrl); err != nil {
		log.Printf("[Upstream] 健康检查失败: %v", err)
		return "", "", "", fmt.Errorf("上游服务不可用: %v", err)
	}

	// ✅ 发起请求
	resp, err := utils.HttpPostJson(upstreamUrl, params)
	if err != nil {
		log.Printf("[Upstream] 请求失败: %v", err)
		return "", "", "", fmt.Errorf("请求上游失败: %v", err)
	}
	log.Printf("[Upstream] 响应原始数据: %s", resp)

	// ✅ 解析响应
	var data struct {
		Code interface{} `json:"code"`
		Msg  string      `json:"msg"`
		Data struct {
			UpOrderNo string `json:"up_order_no"`
			PayUrl    string `json:"pay_url"`
			MOrderId  string `json:"m_order_id"`
		} `json:"data"`
	}

	if err := json.Unmarshal([]byte(resp), &data); err != nil {
		log.Printf("[Upstream] JSON解析失败: %v", err)
		return "", "", "", fmt.Errorf("响应解析失败: %v", err)
	}

	if data.Code != string('0') {
		log.Printf("[Upstream] 上游返回错误: %s", data.Msg)
		return "", "", "", fmt.Errorf("上游错误: %s", data.Msg)
	}

	log.Printf("[Upstream] 下单成功,响应数据: upOrderNo=%+v, payUrl=%+v,mOrderId:%+v", data.Data.UpOrderNo, data.Data.PayUrl, data.Data.MOrderId)
	return data.Data.MOrderId, data.Data.UpOrderNo, data.Data.PayUrl, nil
}
