package middleware

import (
	"bytes"
	"io"
	"net/http"
	"time"
	"wht-order-api/internal/dao"
	"wht-order-api/internal/utils"

	"github.com/gin-gonic/gin"
)

// PayRequest 定义请求参数结构体
type PayRequest struct {
	Version      string `json:"version" binding:"required"`
	MerchantNo   string `json:"merchant_no" binding:"required"`
	TranFlow     string `json:"tran_flow" binding:"required"`
	TranDatetime string `json:"tran_datetime" binding:"required"` //13位时间戳
	Amount       string `json:"amount" binding:"required"`
	PayType      string `json:"pay_type" binding:"required"`
	NotifyUrl    string `json:"notify_url" binding:"omitempty,url"`
	RedirectUrl  string `json:"redirect_url"`
	Currency     string `json:"currency" binding:"required,len=3"`
	ProductInfo  string `json:"product_info" binding:"required"`
	AccNo        string `json:"acc_no"`
	AccName      string `json:"acc_name"`
	PayEmail     string `json:"pay_email"`
	PayPhone     string `json:"pay_phone"`
	BankCode     string `json:"bank_code"`
	BankName     string `json:"bank_name"`
	Sign         string `json:"sign" binding:"required"` // MD5 32大写
}

// AuthMD5Sign 中间件：验证 POST JSON 请求签名
func AuthMD5Sign() gin.HandlerFunc {
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
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "cannot read body"})
			c.Abort()
			return
		}

		// 恢复 body
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		// 解析 JSON
		var req PayRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid request params"})
			c.Abort()
			return
		}

		// 4️⃣ 校验 timestamp 和 nonce
		tsInt, err := utils.ParseTimestamp(req.TranDatetime)
		if err != nil || !utils.IsTimestampValid(tsInt, 1*time.Minute) {
			c.JSON(http.StatusForbidden, gin.H{"code": 400, "msg": "request timeout"})
			c.Abort()
			return
		}

		// 查询商户信息
		mainDao := &dao.MainDao{}
		merchant, _ := mainDao.GetMerchant(req.MerchantNo)
		if merchant.Status != 1 {
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
			"redirect_url":  req.RedirectUrl,
			"currency":      req.Currency,
			"product_info":  req.ProductInfo,
			"acc_no":        req.AccNo,
			"acc_name":      req.AccName,
			"pay_email":     req.PayEmail,
			"pay_phone":     req.PayPhone,
			"bank_code":     req.BankCode,
			"bank_name":     req.BankName,
			"sign":          req.Sign,
		}

		apiKey := ""
		// 验签
		if !utils.VerifySign(params, apiKey) {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "invalid signature"})
			c.Abort()
			return
		}
		c.Set("pay_request", req) // 放入 context 供 handler 使用
		c.Next()
	}
}
