package dto

import (
	"github.com/shopspring/decimal"
	"time"
)

type MerchantMoney struct {
	ID          uint64          `json:"id"`           // 主键
	UID         uint64          `json:"uid"`          // 用户ID
	Status      int8            `json:"status"`       // 状态: 0=全冻结, 1=可用
	Currency    string          `json:"currency"`     // 货币
	Money       decimal.Decimal `json:"money"`        // 可用余额
	FreezeMoney decimal.Decimal `json:"freeze_money"` // 冻结余额
	CreateTime  time.Time       `json:"create_time"`  // 创建时间戳
	UpdateTime  time.Time       `json:"update_time"`  // 更新时间戳
}
