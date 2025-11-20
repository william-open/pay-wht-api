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

// ReassignOrderHandler 代付改派处理器
type ReassignOrderHandler struct{ svc *service.ReassignOrderService }

func NewReassignOrderHandler() *ReassignOrderHandler {
	return &ReassignOrderHandler{svc: service.NewReassignOrderService()}
}

// ReassignOrderCreate 代付订单创建
func (h *ReassignOrderHandler) ReassignOrderCreate(c *gin.Context) {
	val, exists := c.Get("payout_request")
	if !exists {
		c.JSON(http.StatusOK, utils.Error(constant.CodeMissingParams))
		return
	}
	req, ok := val.(dto.CreateReassignOrderReq)
	if !ok {
		c.JSON(http.StatusOK, utils.Error(constant.CodeParamsTypeError))
		return
	}
	log.Printf("收到数据: %+v\n", req)

	// 获取审计上下文
	requestType, _ := c.Get("request_type")
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
		//c.JSON(http.StatusOK, utils.CustomErrorWithTrace(constant.CodeSystemError, err.Error(), auditCtx.TraceID))
		c.JSON(http.StatusOK, utils.CustomErrorWithTrace(constant.CodeTransactionFailed, err.Error(), auditCtx.TraceID))
		return
	}
	auditCtx.PlatformOrderID = paySerialNo
	if err != nil {
		auditCtx.Status = "failed"
		auditCtx.ErrorMsg = err.Error()
		auditCtx.ResponseBody = `{"code":400,"msg":"` + err.Error() + `"}`
		log.Printf("[TraceId]: %+v,响应信息: %+v", auditCtx.TraceID, err.Error())
		c.JSON(http.StatusOK, utils.CustomErrorWithTrace(constant.CodeSystemError, err.Error(), auditCtx.TraceID))
		return
	}
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid ID parameter"})
		return
	}
	// 响应成功，记录 trace_id 并写入日志
	response.TraceID = auditCtx.TraceID
	respJson, _ := json.Marshal(response)
	auditCtx.Status = "success"
	auditCtx.ResponseBody = string(respJson)

	c.JSON(http.StatusOK, response)
}
