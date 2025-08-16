package handler

import (
	"net/http"
	"sort"
	"strconv"
	"time"
	"wht-order-api/internal/repo"

	"github.com/gin-gonic/gin"
	"wht-order-api/internal/dto"
	"wht-order-api/internal/service"
	"wht-order-api/internal/shard"
)

type OrderHandler struct{ svc *service.OrderService }

func NewOrderHandler() *OrderHandler { return &OrderHandler{svc: service.NewOrderService()} }

func (h *OrderHandler) Create(c *gin.Context) {
	var req dto.CreateOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	oid, err := h.svc.Create(req)
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	c.JSON(200, dto.CreateOrderResp{
		OrderID:  strconv.FormatUint(oid, 10),
		Status:   "PENDING",
		PayData:  gin.H{"redirect": "mock://channel/pay/" + strconv.FormatUint(oid, 10)},
		ExpireIn: 300,
	})
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
		"order_id": m.OrderID, "merchant_id": m.MerchantID, "merchant_ord_no": m.MerchantOrdNo,
		"amount": m.Amount, "currency": m.Currency, "status": m.Status, "pay_method": m.PayMethod,
		"created_at": m.CreatedAt,
	})
}

func (h *OrderHandler) List(c *gin.Context) {
	kw := c.Query("kw")
	statusStr := c.Query("status")
	var status *int8
	if statusStr != "" {
		tmp, _ := strconv.Atoi(statusStr)
		v := int8(tmp)
		status = &v
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	pageNum, _ := strconv.Atoi(c.DefaultQuery("page_num", "1"))
	offset := (pageNum - 1) * pageSize

	ts := time.Now()
	tables := shard.AllTables("merchant_order", ts)

	r := &repo.OrderRepo{}
	list, total, err := r.ListInTables(tables, kw, status, pageSize, offset)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt.After(list[j].CreatedAt) })

	out := make([]dto.OrderVO, 0, len(list))
	for _, m := range list {
		out = append(out, dto.OrderVO{
			OrderID: m.OrderID, MerchantID: m.MerchantID, MerchantOrdNo: m.MerchantOrdNo,
			Amount: m.Amount, Currency: m.Currency, PayMethod: m.PayMethod, Status: m.Status, CreatedAt: m.CreatedAt,
		})
	}

	c.JSON(200, gin.H{"total": total, "list": out})
}
