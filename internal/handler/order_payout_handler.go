package handler

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"strconv"
	"time"
	"wht-order-api/internal/constant"
	"wht-order-api/internal/dto"
	"wht-order-api/internal/service"
	"wht-order-api/internal/utils"
)

// 代付处理器
type PayoutOrderPayoutHandler struct{ svc *service.PayoutOrderService }

func NewPayoutOrderHandler() *PayoutOrderPayoutHandler {
	return &PayoutOrderPayoutHandler{svc: service.NewPayoutOrderService()}
}

// PayoutOrderCreate 代付订单创建
func (h *PayoutOrderPayoutHandler) PayoutOrderCreate(c *gin.Context) {
	// 从中间件获取 pay_request 数据
	val, exists := c.Get("payout_request")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "pay_request not found"})
		return
	}

	// 类型断言为 dto.CreatePayoutOrderReq
	req, ok := val.(dto.CreatePayoutOrderReq)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid pay_request type"})
		return
	}

	// 打印调试日志（可选）
	log.Printf("代付收到数据: %+v\n", req)
	requestType, _ := c.Get("request_type")
	// 获取审计上下文
	ctxVal, _ := c.Get("audit_ctx")
	auditCtx := ctxVal.(*dto.AuditContextPayload)
	auditCtx.MerchantNo = req.MerchantNo
	auditCtx.TranFlow = req.TranFlow
	auditCtx.ChannelCode = req.PayType
	auditCtx.CreatedAt = time.Now()
	auditCtx.RequestType = requestType.(string)
	// 调用服务层处理
	response, err := h.svc.Create(req)
	paySerialNo, parseErr := strconv.ParseUint(response.PaySerialNo, 10, 64)
	if parseErr != nil {
		log.Printf("[TraceId]: %+v,响应信息: %+v", auditCtx.TraceID, err.Error())
		c.JSON(http.StatusOK, utils.CustomErrorWithTrace(constant.CodeSystemError, err.Error(), auditCtx.TraceID))
		return
	}
	auditCtx.PlatformOrderID = paySerialNo
	if err != nil {
		auditCtx.Status = "failed"
		auditCtx.ErrorMsg = err.Error()
		auditCtx.ResponseBody = `{"code":400,"msg":"` + err.Error() + `"}`
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	// 响应成功，记录 trace_id 并写入日志
	response.TraceID = auditCtx.TraceID
	respJson, _ := json.Marshal(response)
	auditCtx.Status = "success"
	auditCtx.ResponseBody = string(respJson)
	c.JSON(http.StatusOK, response)
}

// PayoutOrderQuery 代付订单查询
func (h *PayoutOrderPayoutHandler) PayoutOrderQuery(c *gin.Context) {
	// 从中间件获取 account_request 数据
	val, exists := c.Get("payout_query_request")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "account_request not found"})
		return
	}

	// 类型断言为 dto.AccountReq
	req, ok := val.(dto.QueryPayoutOrderReq)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid pay_request type"})
		return
	}
	// 获取审计上下文
	requestType, _ := c.Get("request_type")
	ctxVal, _ := c.Get("audit_ctx")
	auditCtx := ctxVal.(*dto.AuditContextPayload)
	auditCtx.MerchantNo = req.MerchantNo
	auditCtx.TranFlow = req.TranFlow
	auditCtx.Status = "success"
	auditCtx.RequestType = requestType.(string)

	// 调用服务层处理
	response, err := h.svc.Get(req)
	if err != nil {
		auditCtx.Status = "failed"
		auditCtx.ErrorMsg = err.Error()
		auditCtx.ResponseBody = `{"code":400,"msg":"` + err.Error() + `"}`
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	respJson, _ := json.Marshal(response)
	auditCtx.ResponseBody = string(respJson)
	c.JSON(http.StatusOK, response)
}
