package handler

import (
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"wht-order-api/internal/dto"
	"wht-order-api/internal/service"
)

// AccountHandler 账户处理器
type AccountHandler struct{ svc *service.AccountService }

func NewAccountHandler() *AccountHandler {
	return &AccountHandler{svc: service.NewAccountService()}
}

// Query 账户查询
func (h *AccountHandler) Query(c *gin.Context) {
	// 从中间件获取 account_request 数据
	val, exists := c.Get("account_request")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "account_request not found"})
		return
	}

	// 类型断言为 dto.AccountReq
	req, ok := val.(dto.AccountReq)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid pay_request type"})
		return
	}
	// 打印调试日志（可选）
	log.Printf("账户查询收到数据: %+v\n", req)

	// 调用服务层处理
	response, err := h.svc.Get(req.MerchantNo, req.Currency)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}
