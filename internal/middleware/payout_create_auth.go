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
	"wht-order-api/internal/system"
	"wht-order-api/internal/utils"

	"github.com/gin-gonic/gin"
)

// 统一错误响应 + 通知
func failPayoutWithTgNotify(c *gin.Context, req dto.CreatePayoutOrderReq, httpCode int, msg string, data interface{}) {
	c.JSON(httpCode, data)
	go func() {
		mainDao := dao.NewMainDao()
		merchant, _ := mainDao.GetMerchant(req.MerchantNo)
		notify.Notify(
			system.BotChatID,
			"warn",
			"[代付] 调用失败",
			fmt.Sprintf(
				"商户号: %v\n商户昵称: %v\n应用ID: %v\n商户单号: %s\n请求状态: 失败\n通道编码: %s\n请求IP: %s\n错误描述: %s\n请求参数: %s\n响应参数: %s",
				merchant.MerchantID,
				merchant.NickName,
				merchant.AppId,
				req.TranFlow,
				req.PayType,
				utils.GetRealClientIP(c),
				msg,
				utils.MapToJSON(req),
				utils.MapToJSON(data),
			),
			true,
		)
	}()
	c.Abort()
}

// PayoutCreateAuth 中间件：验证代付订单创建签名
func PayoutCreateAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		if c.Request.Method != http.MethodPost || c.ContentType() != "application/json" {
			c.Next()
			return
		}

		// 限制 body 大小防止攻击
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20) // 1MB

		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			failPayoutWithTgNotify(c, dto.CreatePayoutOrderReq{}, http.StatusBadRequest, "无法读取请求体", utils.Error(constant.CodeInternalError))
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		var req dto.CreatePayoutOrderReq
		if err := c.ShouldBindJSON(&req); err != nil {
			var ve validator.ValidationErrors
			if errors.As(err, &ve) {
				errFields := make([]map[string]string, 0, len(ve))
				for _, fe := range ve {
					errFields = append(errFields, map[string]string{
						"field": fe.Field(),
						"error": utils.ValidationMsg(fe),
					})
				}
				failPayoutWithTgNotify(c, req, http.StatusBadRequest, "参数校验失败", gin.H{
					"code":   400,
					"msg":    "参数校验失败",
					"errors": errFields,
				})
				return
			}

			failPayoutWithTgNotify(c, req, http.StatusBadRequest, "请求格式错误", gin.H{
				"code": 400,
				"msg":  "JSON格式错误，请检查参数类型",
			})
			return
		}

		// 校验时间戳
		tsInt, err := utils.ParseTimestamp(req.TranDatetime)
		if err != nil || !utils.IsTimestampValid(tsInt, time.Minute) {
			failPayoutWithTgNotify(c, req, http.StatusForbidden, "请求过期", utils.Error(constant.CodeTimeout))
			return
		}

		mainDao := dao.NewMainDao()

		// 校验商户
		merchant, mErr := mainDao.GetMerchant(req.MerchantNo)
		if mErr != nil {
			failPayoutWithTgNotify(c, req, http.StatusUnauthorized, "商户异常", utils.Error(constant.CodeMerchantAbnormal))
			return
		}
		if merchant.Status != 1 {
			failPayoutWithTgNotify(c, req, http.StatusUnauthorized, "商户未启用", utils.Error(constant.CodeMerchantDisabled))
			return
		}

		// 校验银行编码
		if req.BankCode != "" {
			if _, err := mainDao.QueryPlatformBankInfo(req.BankCode, merchant.Currency); err != nil {
				msg := fmt.Sprintf("银行编码不存在: %s", req.BankCode)
				failPayoutWithTgNotify(c, req, http.StatusBadRequest, msg, gin.H{"code": 400, "msg": msg})
				return
			}
		}

		// 获取客户端 IP
		clientId := utils.GetClientIP(c)
		if clientId == "" {
			failPayoutWithTgNotify(c, req, http.StatusUnauthorized, "无法识别客户端IP", utils.Error(constant.CodeUnauthorized))
			return
		}
		req.ClientId = clientId

		// 全局白名单校验
		globalService := service.NewGlobalWhitelistService()
		// 验证 IP 白名单（类型 2 = 代付）
		verifyService := service.NewVerifyIpWhitelistService()
		if !globalService.IsGlobal(clientId) {
			if !verifyService.VerifyIpWhitelist(clientId, merchant.MerchantID, 2) {
				msg := fmt.Sprintf("IP:%s 不在白名单内", clientId)
				failPayoutWithTgNotify(c, req, http.StatusUnauthorized, msg, gin.H{"code": constant.CodeIPNotWhitelisted, "msg": msg})
				return
			}
		}

		// 构造签名参数（排除 sign）
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
			"account_type":  req.AccountType,
			"cci_no":        req.CciNo,
			"branch_bank":   req.BranchBank,
			"address":       req.Address,
			"network":       req.Network,
			"sign":          req.Sign,
		}

		// 验签
		if !utils.VerifySign(params, merchant.ApiKey) {
			failPayoutWithTgNotify(c, req, http.StatusUnauthorized, "签名验证失败", utils.Error(constant.CodeSignatureError))
			return
		}

		log.Printf("[Payout] ✅ 验签通过 商户号=%s 通道=%s IP=%s 耗时=%v",
			req.MerchantNo, req.PayType, clientId, time.Since(start))

		c.Set("payout_request", req)
		c.Set("request_type", "payout")
		c.Next()
	}
}
