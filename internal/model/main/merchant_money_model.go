package mainmodel

import (
	"github.com/shopspring/decimal"
	"time"
)

type MerchantMoney struct {
	ID          uint64          `gorm:"column:id;primaryKey;autoIncrement" json:"id"`                                       // 主键
	UID         uint64          `gorm:"column:uid;not null" json:"uid"`                                                     // 用户ID
	Status      int8            `gorm:"column:status;not null;default:1" json:"status"`                                     // 状态: 0=全冻结, 1=可用
	Currency    string          `gorm:"column:currency;size:10;not null" json:"currency"`                                   // 货币
	Money       decimal.Decimal `gorm:"column:money;type:decimal(18,4);not null;default:0.0000" json:"money"`               // 可用余额
	FreezeMoney decimal.Decimal `gorm:"column:freeze_money;type:decimal(18,4);not null;default:0.0000" json:"freeze_money"` // 冻结余额
	CreateTime  time.Time       `gorm:"column:create_time;not null" json:"create_time"`                                     // 创建时间戳
	UpdateTime  time.Time       `gorm:"column:update_time;not null" json:"update_time"`
}

func (MerchantMoney) TableName() string {
	return "w_merchant_money"
}
