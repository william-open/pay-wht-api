package mainmodel

import (
	"github.com/shopspring/decimal"
	"time"
)

// AgentMoney 代理佣金收益记录表
type AgentMoney struct {
	ID         uint64          `gorm:"column:id;primaryKey;autoIncrement" json:"id"`                                   // 主键
	AID        uint64          `gorm:"column:a_id;not null;default:0" json:"a_id"`                                     // 代理用户ID
	MID        uint64          `gorm:"column:m_id;not null;default:0" json:"m_id"`                                     // 商户用户ID
	Type       int8            `gorm:"column:type;not null;default:11" json:"type"`                                    // 收益类型
	Money      decimal.Decimal `gorm:"column:money;type:decimal(18,2);not null;default:0.00" json:"money"`             // 收益金额
	OrderNo    string          `gorm:"column:order_no;size:30;not null" json:"order_no"`                               // 订单编码
	OrderMoney decimal.Decimal `gorm:"column:order_money;type:decimal(18,2);not null;default:0.00" json:"order_money"` // 来源订单金额
	Currency   string          `gorm:"column:currency;size:6;not null" json:"currency"`                                // 币种
	Remark     string          `gorm:"column:remark;size:10" json:"remark"`                                            // 备注
	CreateTime time.Time       `gorm:"column:create_time;default:0" json:"createTime"`                                 // 创建时间戳
}

func (AgentMoney) TableName() string {
	return "w_agent_money"
}
