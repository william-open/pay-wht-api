package handler

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"wht-order-api/internal/dto"
	"wht-order-api/internal/service"
)

type OrderHandler struct{ svc *service.OrderService }

func NewOrderHandler() *OrderHandler { return &OrderHandler{svc: service.NewOrderService()} }

func (h *OrderHandler) Create(c *gin.Context) {
	var req dto.CreateOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	response, err := h.svc.Create(req)
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	c.JSON(200, response)
}

func (h *OrderHandler) Get(c *gin.Context) {
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
