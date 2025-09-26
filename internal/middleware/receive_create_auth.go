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
	"wht-order-api/internal/constant"
	"wht-order-api/internal/dao"
	"wht-order-api/internal/dto"
	"wht-order-api/internal/notify"
	"wht-order-api/internal/service"
	"wht-order-api/internal/utils"

	"github.com/gin-gonic/gin"
)

// ReceiveCreateAuth 中间件：验证 代收订单创建 POST JSON 请求签名
func ReceiveCreateAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.JSON(http.StatusBadRequest, utils.Error(constant.CodeInvalidParams))
			c.Abort()
			return
		}

		if c.ContentType() != "application/json" {
			c.JSON(http.StatusBadRequest, utils.Error(constant.CodeInvalidParams))
			c.Abort()
			return
		}

		// 读取 body
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Printf("收到回调数据: %+v\n", 222)
			c.JSON(http.StatusBadRequest, utils.Error(constant.CodeInternalError))
			c.Abort()
			return
		}

		// 恢复 body
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		// 解析 JSON
		var req dto.CreateOrderReq
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
			log.Printf("错误不能解析数据: %v", err.Error())
			c.JSON(http.StatusBadRequest, utils.Error(constant.CodeInvalidParams))
			c.Abort()
			return

		}

		// 4️⃣ 校验 timestamp 和 nonce
		tsInt, err := utils.ParseTimestamp(req.TranDatetime)
		log.Printf("请求时间: %v", tsInt)
		if err != nil || !utils.IsTimestampValid(tsInt, 1*time.Minute) {
			log.Printf("请求超时: %v", req.TranDatetime)
			c.JSON(http.StatusForbidden, utils.Error(constant.CodeTimeout))
			c.Abort()
			return
		}

		// 查询商户信息
		mainDao := dao.NewMainDao()
		merchant, _ := mainDao.GetMerchant(req.MerchantNo)
		if merchant.Status != 1 {
			log.Printf("商户不存在: %v", merchant)
			c.JSON(http.StatusUnauthorized, utils.Error(constant.CodeMerchantDisabled))
			c.Abort()
			return
		}

		// 获取请求IP
		clientId := utils.GetClientIP(c)
		if clientId == "" {
			log.Printf("未获取到客户端IP: %+v", merchant)
			c.JSON(http.StatusUnauthorized, utils.Error(constant.CodeUnauthorized))
			c.Abort()
			return
		}

		// 验证IP是否允许
		verifyService := service.NewVerifyIpWhitelistService()
		canAccess := verifyService.VerifyIpWhitelist(clientId, merchant.MerchantID, 1)
		if !canAccess {
			errorTipText := fmt.Sprintf("IP不允许访问: %+v,IP: %v", merchant, clientId)
			go func() {
				err := notify.SendTelegramMessage(merchant.TelegramGroupChatId, errorTipText)
				if err != nil {

				}
			}()
			c.JSON(http.StatusUnauthorized, gin.H{"code": constant.CodeIPNotWhitelisted, "msg": fmt.Sprintf("IP:%s,不在白名单内，请联系运营人员进行添加", clientId)})
			c.Abort()
			return
		}
		req.ClientId = clientId
		// 核查商户通道编码是否开启
		canChannel := verifyService.VerifyChannelValid(merchant.MerchantID, req.PayType)
		if !canChannel {
			log.Printf("通道未启动: %+v,IP: %v", req.PayType, clientId)
			c.JSON(http.StatusUnauthorized, utils.Error(constant.CodeChannelDisabled))
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
			//"currency":      req.Currency,
			"product_info": req.ProductInfo,
			"acc_no":       req.AccNo,
			"acc_name":     req.AccName,
			"pay_email":    req.PayEmail,
			"pay_phone":    req.PayPhone,
			"bank_code":    req.BankCode,
			"bank_name":    req.BankName,
			"sign":         req.Sign,
		}

		apiKey := merchant.ApiKey
		//log.Printf("验证签名的密钥: %v", apiKey)
		//log.Printf("待验参数: %v", params)
		// 验签
		if !utils.VerifySign(params, apiKey) {
			errorTipText := utils.Error(constant.CodeSignatureError).Msg
			go func() {
				err := notify.SendTelegramMessage(merchant.TelegramGroupChatId, errorTipText)
				if err != nil {

				}
			}()
			c.JSON(http.StatusUnauthorized, utils.Error(constant.CodeSignatureError))
			c.Abort()
			return
		}
		log.Printf("请求参数: %+v", req)
		c.Set("pay_request", req)        // 放入 context 供 handler 使用
		c.Set("request_type", "receive") // 放入 context 供 handler 使用
		c.Next()
	}
}
