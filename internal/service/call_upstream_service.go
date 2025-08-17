package service

import (
	"encoding/json"
	"fmt"
	"wht-order-api/internal/config"
	"wht-order-api/internal/dto"
	"wht-order-api/internal/utils"
)

// CallUpstreamService 调用 PHP 服务下单
func CallUpstreamService(req dto.CreateOrderReq, channel *dto.PaymentChannelVo) (string, string, error) {
	// 组装请求参数
	params := map[string]interface{}{
		"order_id":     req.TranFlow,
		"amount":       req.Amount,
		"currency":     req.Currency,
		"notify_url":   req.NotifyUrl,
		"return_url":   req.RedirectUrl,
		"channel_code": channel.Coding,
		"merchant_no":  req.MerchantNo,
		"product_info": req.ProductInfo,
		"pay_email":    req.PayEmail,
		"pay_phone":    req.PayPhone,
	}

	resp, err := utils.HttpPostJson(config.C.Upstream.ApiUrl, params)
	if err != nil {
		return "", "", err
	}

	var data struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			UpOrderNo string `json:"up_order_no"`
			PayUrl    string `json:"pay_url"`
		} `json:"data"`
	}

	if err := json.Unmarshal([]byte(resp), &data); err != nil {
		return "", "", err
	}

	if data.Code != 0 {
		return "", "", fmt.Errorf("php error: %s", data.Msg)
	}

	return data.Data.UpOrderNo, data.Data.PayUrl, nil
}
