package middleware

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/go-playground/validator/v10"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
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

// 统一失败响应+通知
func failWithNotify(c *gin.Context, req dto.CreateOrderReq, code int, msg string, data interface{}) {
	c.JSON(code, data)
	go func() {
		notify.Notify(
			system.BotChatID,
			"warn",
			"[receive] 调用失败",
			fmt.Sprintf(
				"商户号: %s\n订单号: %s\n请求状态: Failed\n通道编码: %s\nIP: %s\n错误描述: %s\n请求参数: %s\n响应参数: %s",
				req.MerchantNo,
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

// ReceiveCreateAuth 验证代收订单创建签名
func ReceiveCreateAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		if c.Request.Method != http.MethodPost || c.ContentType() != "application/json" {
			c.JSON(http.StatusBadRequest, utils.Error(constant.CodeInvalidParams))
			c.Abort()
			return
		}

		// 限制Body大小，防止攻击
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20) // 1MB

		// 读取 Body
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			failWithNotify(c, dto.CreateOrderReq{}, http.StatusBadRequest, "读取请求体失败", utils.Error(constant.CodeInternalError))
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		// 解析 JSON
		var req dto.CreateOrderReq
		if err := c.ShouldBindJSON(&req); err != nil {
			// ✅ 字段校验错误
			var ve validator.ValidationErrors
			if errors.As(err, &ve) {
				errFields := make([]map[string]string, 0, len(ve))
				for _, fe := range ve {
					errFields = append(errFields, map[string]string{
						"field": fe.Field(),
						"error": utils.ValidationMsg(fe),
					})
				}
				failWithNotify(c, req, http.StatusBadRequest, "参数校验失败", gin.H{
					"code":   400,
					"msg":    "参数校验失败",
					"errors": errFields,
				})
				return
			}

			// ✅ JSON类型错误隐藏系统细节
			if strings.Contains(err.Error(), "cannot unmarshal") {
				re := regexp.MustCompile(`field (\w+\.)?(\w+)`)
				fieldName := "unknown"
				if m := re.FindStringSubmatch(err.Error()); len(m) > 2 {
					fieldName = m[2]
				}
				paramErr := gin.H{
					"code": 400,
					"msg":  "参数类型错误",
					"errors": []map[string]string{
						{"field": fieldName, "error": "字段类型与预期不符"},
					},
				}
				failWithNotify(c, req, http.StatusBadRequest, "JSON类型错误", paramErr)
				return
			}

			failWithNotify(c, req, http.StatusBadRequest, "JSON解析错误", utils.Error(constant.CodeParamsFormatError))
			return
		}

		// 校验时间戳
		tsInt, err := utils.ParseTimestamp(req.TranDatetime)
		if err != nil || !utils.IsTimestampValid(tsInt, time.Minute) {
			failWithNotify(c, req, http.StatusForbidden, "请求超时", utils.Error(constant.CodeTimeout))
			return
		}

		// 商户验证
		mainDao := dao.NewMainDao()
		merchant, _ := mainDao.GetMerchant(req.MerchantNo)
		if merchant.Status != 1 {
			failWithNotify(c, req, http.StatusUnauthorized, "商户未启用", utils.Error(constant.CodeMerchantDisabled))
			return
		}

		clientId := utils.GetClientIP(c)
		if clientId == "" {
			failWithNotify(c, req, http.StatusUnauthorized, "无法识别IP", utils.Error(constant.CodeUnauthorized))
			return
		}

		// 白名单校验
		verifyService := service.NewVerifyIpWhitelistService()
		if !verifyService.VerifyIpWhitelist(clientId, merchant.MerchantID, 1) {
			msg := fmt.Sprintf("IP:%s 不在白名单内", clientId)
			go notify.SendTelegramMessage(merchant.TelegramGroupChatId, msg)
			failWithNotify(c, req, http.StatusUnauthorized, msg, gin.H{"code": constant.CodeIPNotWhitelisted, "msg": msg})
			return
		}

		req.ClientId = clientId

		// 通道是否启用
		if !verifyService.VerifyChannelValid(merchant.MerchantID, req.PayType) {
			failWithNotify(c, req, http.StatusUnauthorized, "通道未启用", utils.Error(constant.CodeChannelDisabled))
			return
		}

		// 验签
		params := map[string]string{
			"version":       req.Version,
			"merchant_no":   req.MerchantNo,
			"tran_flow":     req.TranFlow,
			"tran_datetime": req.TranDatetime,
			"amount":        req.Amount,
			"pay_type":      req.PayType,
			"notify_url":    req.NotifyUrl,
			"redirect_url":  req.RedirectUrl,
			"product_info":  req.ProductInfo,
			"acc_no":        req.AccNo,
			"acc_name":      req.AccName,
			"pay_email":     req.PayEmail,
			"pay_phone":     req.PayPhone,
			"bank_code":     req.BankCode,
			"pay_method":    req.PayMethod,
			"identity_num":  req.IdentityNum,
			"identity_type": req.IdentityType,
			"bank_name":     req.BankName,
			"sign":          req.Sign,
		}
		if !utils.VerifySign(params, merchant.ApiKey) {
			go notify.SendTelegramMessage(merchant.TelegramGroupChatId, "签名验证失败")
			failWithNotify(c, req, http.StatusUnauthorized, "签名验证失败", utils.Error(constant.CodeSignatureError))
			return
		}

		// ✅ 验证通过
		log.Printf("[Receive] 校验通过: 商户号=%s, 通道=%s, 耗时=%v", req.MerchantNo, req.PayType, time.Since(start))
		c.Set("pay_request", req)
		c.Set("request_type", "receive")
		c.Next()
	}
}
