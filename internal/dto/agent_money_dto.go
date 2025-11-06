package dto

import (
	"github.com/shopspring/decimal"
	"time"
)

type AgentMoney struct {
	ID         uint64          `json:"id"`          // 主键
	AID        uint64          `json:"a_id"`        // 代理用户ID
	MID        uint64          `json:"m_id"`        // 商户用户ID
	Type       int8            `json:"type"`        // 收益类型
	Money      decimal.Decimal `json:"money"`       // 收益金额
	OrderNo    string          `json:"order_no"`    // 平台订单编码
	MOrderNo   string          `json:"m_order_no"`  // 商户订单编码
	OrderMoney decimal.Decimal `json:"order_money"` // 来源订单金额
	Currency   string          `json:"currency"`    // 币种
	Remark     string          `json:"remark"`      // 备注
	CreateTime time.Time       `json:"createTime"`  // 创建时间戳
}
