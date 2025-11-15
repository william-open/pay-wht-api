package middleware

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/go-playground/validator/v10"
	"io"
	"log"
	"net/http"
	"time"
	"wht-order-api/internal/dao"
	"wht-order-api/internal/dto"
	"wht-order-api/internal/service"
	"wht-order-api/internal/utils"

	"github.com/gin-gonic/gin"
)

// ReassignCreateAuth 中间件：验证 代付订单创建 POST JSON 请求签名
func ReassignCreateAuth() gin.HandlerFunc {
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
		var req dto.CreateReassignOrderReq
		if err := c.ShouldBindJSON(&req); err != nil {
			// 判断是否为字段验证错误 validator.ValidationErrors 类型断言，并逐项提取字段名与错误原因
			var ve validator.ValidationErrors
			if errors.As(err, &ve) {
				errFields := make([]map[string]string, 0)
				for _, fe := range ve {
					errFields = append(errFields, map[string]string{
						"field": fe.Field(),              // 字段名
						"error": utils.ValidationMsg(fe), // 错误信息
					})
				}
				c.JSON(http.StatusBadRequest, gin.H{
					"code":   400,
					"msg":    "参数校验失败",
					"errors": errFields,
				})
				c.Abort()
				return
			}

			// 非字段错误（如 JSON 格式错误）
			c.JSON(http.StatusBadRequest, gin.H{
				"code": 400,
				"msg":  "请求格式错误: " + err.Error(),
			})
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
		// 核心库实例
		mainDao := dao.NewMainDao()

		// 查询商户信息
		merchant, _ := mainDao.GetMerchant(req.MerchantNo)
		if merchant.Status != 1 {
			log.Printf("商户不存在: %v", merchant)
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "unauthorized"})
			c.Abort()
			return
		}

		// 校验银行编码
		if req.BankCode != "" {
			// 根据接平台银行编码查询平台银行信息
			_, pbErr := mainDao.QueryPlatformBankInfo(req.BankCode, merchant.Currency)
			if pbErr != nil {
				resultMsg := fmt.Sprintf("Bank code does not exist,%s", req.BankCode)
				log.Printf(resultMsg)
				c.JSON(http.StatusForbidden, gin.H{"code": 400, "msg": resultMsg})
				c.Abort()
				return
			}
		}

		// 获取请求IP
		clientId := utils.GetClientIP(c)
		if clientId == "" {
			log.Printf("未获取到客户端IP: %+v", merchant)
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "Unauthorized,IP Error"})
			c.Abort()
			return
		}
		req.ClientId = clientId
		// 全局白名单校验
		globalService := service.NewGlobalWhitelistService()
		// 验证IP是否允许
		verifyService := service.NewVerifyIpWhitelistService()
		if !globalService.IsGlobal(clientId) {
			canAccess := verifyService.VerifyIpWhitelist(clientId, merchant.MerchantID, 2)
			if !canAccess {
				log.Printf("IP不允许访问: %+v,IP: %v", merchant, clientId)
				c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": fmt.Sprintf("Unauthorized,IP[%v] is not whitelisted", clientId)})
				c.Abort()
				return
			}
		}

		// 提取参数做签名（排除 Sign 字段）
		params := map[string]string{
			"version":        req.Version,
			"merchant_no":    req.MerchantNo,
			"tran_flow":      req.TranFlow,
			"tran_datetime":  req.TranDatetime,
			"amount":         req.Amount,
			"pay_type":       req.PayType,
			"notify_url":     req.NotifyUrl,
			"acc_no":         req.AccNo,
			"acc_name":       req.AccName,
			"pay_email":      req.PayEmail,
			"pay_phone":      req.PayPhone,
			"bank_code":      req.BankCode,
			"bank_name":      req.BankName,
			"identity_type":  req.IdentityType,
			"identity_num":   req.IdentityNum,
			"pay_method":     req.PayMethod,
			"pay_product_id": req.PayProductId,
			"order_id":       req.OrderId,
			"sign":           req.Sign,
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
		c.Set("payout_request", req)    // 放入 context 供 handler 使用
		c.Set("request_type", "payout") // 放入 context 供 handler 使用
		c.Next()
	}
}
