package mainmodel

import "github.com/shopspring/decimal"

// AgentMerchant represents w_agent_merchant
type AgentMerchant struct {
	ID           int             `gorm:"column:id;primaryKey;autoIncrement" json:"id"`                          // 主键
	AID          int64           `gorm:"column:a_id;not null" json:"aId"`                                       // 代理ID
	MID          int64           `gorm:"column:m_id;not null" json:"mId"`                                       // 商户ID
	Currency     *string         `gorm:"column:currency;type:varchar(30)" json:"currency"`                      // 货币符号
	SysChannelID int64           `gorm:"column:sys_channel_id;not null" json:"sysChannelId"`                    // 系统通道编码ID
	UpChannelID  int64           `gorm:"column:up_channel_id;not null" json:"upChannelId"`                      // 上游通道编码ID
	Status       int8            `gorm:"column:status;type:tinyint(1);default:0" json:"status"`                 // 1:开启0:关闭
	DefaultRate  decimal.Decimal `gorm:"column:default_rate;type:decimal(4,2);default:0.00" json:"defaultRate"` // 代理抽点费率
	SingleFee    decimal.Decimal `gorm:"column:single_fee;type:decimal(4,2);default:0.00" json:"singleFee"`     // 单笔费用
}

func (AgentMerchant) TableName() string {
	return "w_agent_merchant"
}
