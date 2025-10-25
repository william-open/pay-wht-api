package handler

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"wht-order-api/internal/constant"
	"wht-order-api/internal/dto"
	"wht-order-api/internal/service"
	"wht-order-api/internal/utils"
)

// UpstreamHandler 上游服务处理器
type UpstreamHandler struct {
	svc *service.InternalUpstreamService
}

func NewUpstreamHandler() *UpstreamHandler {
	return &UpstreamHandler{svc: service.NewInternalUpstreamService()}
}

// ConfigQuery 通过上游交易订单号 查询上游供应商信息
func (h *UpstreamHandler) ConfigQuery(c *gin.Context) {

	var req dto.QueryUpstreamSupplierReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, utils.Error(constant.CodeInvalidParams))
		return
	}
	// 调用服务层处理
	response, err := h.svc.Get(req.TradeOrderId, req.TradeType)
	if err != nil {
		c.JSON(http.StatusOK, utils.Error(constant.CodeSystemError))
		return
	}
	genericResp := &dto.GenericResp[*dto.UpstreamSupplierDto]{
		Code: "0",
		Msg:  "ok",
		Data: response,
	}
	c.JSON(http.StatusOK, genericResp)
}
