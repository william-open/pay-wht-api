package middleware

import (
	"bytes"
	"fmt"
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

// ReceiveQueryAuth 中间件：验证 查询代收 POST JSON 请求签名
func ReceiveQueryAuth() gin.HandlerFunc {
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
			log.Printf("收到代收查询数据: %+v\n", 222)
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "cannot read body"})
			c.Abort()
			return
		}

		// 恢复 body
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		// 解析 JSON
		var req dto.QueryReceiveOrderReq
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("Receive Query:错误不能解析数据: %v", err.Error())
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid request params"})
			c.Abort()
			return
		}

		// 4️⃣ 校验 请求时间
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

		// 获取请求IP
		clientId := utils.GetClientIP(c)
		if clientId == "" {
			log.Printf("未获取到客户端IP: %+v", merchant)
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "Unauthorized,IP Error"})
			c.Abort()
			return
		}

		// 验证IP是否允许
		verifyService := service.VerifyService{}
		canAccess := verifyService.VerifyIpWhitelist(clientId, merchant.MerchantID, 1)
		if !canAccess {
			log.Printf("IP不允许访问: %+v,IP: %v", merchant, clientId)
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": fmt.Sprintf("Unauthorized,IP[%v] is not whitelisted", clientId)})
			c.Abort()
			return
		}

		// 提取参数做签名（排除 Sign 字段）
		params := map[string]string{
			"version":       req.Version,
			"merchant_no":   req.MerchantNo,
			"tran_flow":     req.TranFlow,
			"tran_datetime": req.TranDatetime,
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
		c.Set("receive_query_request", req) // 放入 context 供 handler 使用
		c.Next()
	}
}
