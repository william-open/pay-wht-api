package handler

import (
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"strconv"
	"wht-order-api/internal/dto"
	"wht-order-api/internal/service"
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
	log.Printf("收到数据: %+v\n", req)

	// 调用服务层处理
	response, err := h.svc.Create(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// PayoutOrderQuery 代付订单查询
func (h *PayoutOrderPayoutHandler) PayoutOrderQuery(c *gin.Context) {
	idStr := c.Param("id")
	id, _ := strconv.ParseUint(idStr, 10, 64)
	m, err := h.svc.Get(id)
	if err != nil || m == nil {
		c.JSON(404, gin.H{"code": 404, "msg": "not found"})
		return
	}
	c.JSON(200, gin.H{
		"order_id": m.OrderID,
		"amount":   m.Amount, "currency": m.Currency, "status": m.Status})
}
