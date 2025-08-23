package middleware

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"time"
	"wht-order-api/internal/dao"
	"wht-order-api/internal/dto"
	"wht-order-api/internal/utils"

	"github.com/gin-gonic/gin"
)

// PayoutCreateAuth 中间件：验证 代付订单创建 POST JSON 请求签名
func PayoutCreateAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.Next()
			return
		}

		if c.ContentType() != "application/json" {
			c.Next()
			return
		}

		// 读取 body
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Printf("收到回调数据: %+v\n", 222)
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "cannot read body"})
			c.Abort()
			return
		}

		// 恢复 body
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		// 解析 JSON
		var req dto.CreatePayoutOrderReq
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("错误不能解析数据: %v", err.Error())
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid request params"})
			c.Abort()
			return
		}

		// 4️⃣ 校验 timestamp 和 nonce
		tsInt, err := utils.ParseTimestamp(req.TranDatetime)
		log.Printf("请求时间: %v", tsInt)
		if err != nil || !utils.IsTimestampValid(tsInt, 1*time.Minute) {
			log.Printf("请求超时: %v", req.TranDatetime)
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "request timeout"})
			c.Abort()
			return
		}

		// 查询商户信息
		mainDao := &dao.MainDao{}
		merchant, _ := mainDao.GetMerchant(req.MerchantNo)
		if merchant.Status != 1 {
			log.Printf("商户不存在: %v", merchant)
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "unauthorized"})
			c.Abort()
			return
		}

		// 提取参数做签名（排除 Sign 字段）
		params := map[string]string{
			"version":       req.Version,
			"merchant_no":   req.MerchantNo,
			"tran_flow":     req.TranFlow,
			"tran_datetime": req.TranDatetime,
			"amount":        req.Amount,
			"pay_type":      req.PayType,
			"notify_url":    req.NotifyUrl,
			"acc_no":        req.AccNo,
			"acc_name":      req.AccName,
			"pay_email":     req.PayEmail,
			"pay_phone":     req.PayPhone,
			"bank_code":     req.BankCode,
			"bank_name":     req.BankName,
			"identity_type": req.IdentityType,
			"identity_num":  req.IdentityNum,
			"pay_method":    req.PayMethod,
			"sign":          req.Sign,
		}

		apiKey := merchant.ApiKey
		//log.Printf("验证签名的密钥: %v", apiKey)
		//log.Printf("待验参数: %v", params)
		// 验签
		if !utils.VerifySign(params, apiKey) {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "invalid signature"})
			c.Abort()
			return
		}
		c.Set("payout_request", req) // 放入 context 供 handler 使用
		c.Next()
	}
}
