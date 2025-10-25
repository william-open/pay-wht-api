package dto

// QueryUpstreamSupplierReq 供应商查询参数
type QueryUpstreamSupplierReq struct {
	TradeOrderId string `json:"tradeOrderId" binding:"required"` //交易订单号
}
