package dto

import "github.com/shopspring/decimal"

type MoneyLog struct {
	ID          uint64          `json:"id"`          // 主键
	UID         uint64          `json:"uid"`         // 用户ID
	Money       decimal.Decimal `json:"money"`       // 金额
	OrderNo     string          `json:"order_no"`    // 订单编码
	Type        int8            `json:"type"`        // 收益类型
	Operator    string          `json:"operator"`    // 操作者
	Currency    string          `json:"currency"`    // 币种
	Description string          `json:"description"` // 备注
	OldBalance  decimal.Decimal `json:"old_balance"` // 原始余额
	Balance     decimal.Decimal `json:"balance"`     // 变化后余额
	CreateTime  int64           `json:"create_time"` // 时间戳
}
