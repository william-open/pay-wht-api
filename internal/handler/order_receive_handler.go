package handler

import (
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"wht-order-api/internal/dto"
	"wht-order-api/internal/service"
)

// ReceiveOrderHandler 代收处理器
type ReceiveOrderHandler struct{ svc *service.ReceiveOrderService }

func NewReceiveOrderHandler() *ReceiveOrderHandler {
	return &ReceiveOrderHandler{svc: service.NewReceiveOrderService()}
}

// ReceiveOrderCreate 代收订单创建
func (h *ReceiveOrderHandler) ReceiveOrderCreate(c *gin.Context) {
	// 从中间件获取 pay_request 数据
	val, exists := c.Get("pay_request")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "pay_request not found"})
		return
	}

	// 类型断言为 dto.CreateOrderReq
	req, ok := val.(dto.CreateOrderReq)
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

// 代收订单查询
func (h *ReceiveOrderHandler) ReceiveOrderQuery(c *gin.Context) {
	// 从中间件获取 account_request 数据
	val, exists := c.Get("receive_query_request")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "account_request not found"})
		return
	}

	// 类型断言为 dto.AccountReq
	req, ok := val.(dto.QueryReceiveOrderReq)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid pay_request type"})
		return
	}
	// 调用服务层处理
	response, err := h.svc.Get(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}
