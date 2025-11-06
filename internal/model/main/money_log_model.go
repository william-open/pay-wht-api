package mainmodel

import (
	"github.com/shopspring/decimal"
	"time"
)

type MoneyLog struct {
	ID          uint64          `gorm:"column:id;primaryKey;autoIncrement" json:"id"`                     // 主键
	UID         uint64          `gorm:"column:uid;not null" json:"uid"`                                   // 用户ID
	Money       decimal.Decimal `gorm:"column:money;type:decimal(18,4);not null" json:"money"`            // 金额
	OrderNo     string          `gorm:"column:order_no;size:50;not null" json:"orderNo"`                  // 平台订单编码
	MOrderNo    string          `gorm:"column:m_order_no;size:50;not null" json:"mOrderNo"`               // 商户订单编码
	Type        int8            `gorm:"column:type;not null" json:"type"`                                 // 收益类型
	Operator    string          `gorm:"column:operator;size:15;not null" json:"operator"`                 // 操作者
	Currency    string          `gorm:"column:currency;size:10;not null" json:"currency"`                 // 币种
	Description string          `gorm:"column:description;size:30" json:"description"`                    // 备注
	OldBalance  decimal.Decimal `gorm:"column:old_balance;type:decimal(13,2);not null" json:"oldBalance"` // 原始余额
	Balance     decimal.Decimal `gorm:"column:balance;type:decimal(13,2);not null" json:"balance"`        // 变化后余额
	CreateTime  time.Time       `gorm:"column:create_time;not null" json:"createTime"`                    // 时间戳
}

func (MoneyLog) TableName() string {
	return "w_money_log"
}
