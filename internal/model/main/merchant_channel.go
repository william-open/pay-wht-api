package mainmodel

// MerchantChannel represents w_merchant_channel
type MerchantChannel struct {
	ID           int     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`                            // 主键
	MID          int64   `gorm:"column:m_id;not null" json:"mId"`                                         // 商户ID
	Currency     *string `gorm:"column:currency;type:varchar(30)" json:"currency"`                        // 货币符号
	SysChannelID int64   `gorm:"column:sys_channel_id;not null" json:"sysChannelId"`                      // 系统通道编码ID
	UpChannelID  int64   `gorm:"column:up_channel_id;not null" json:"upChannelId"`                        // 上游通道编码ID
	Status       int8    `gorm:"column:status;type:tinyint(1);default:0" json:"status"`                   // 1:开启0:关闭
	DefaultRate  float64 `gorm:"column:default_rate;type:decimal(4,2);default:0.00" json:"defaultRate"`   // 默认费率
	SingleFee    float64 `gorm:"column:single_fee;type:decimal(4,2);default:0.00" json:"singleFee"`       // 单笔费用
	Weight       int     `gorm:"column:weight;default:1" json:"weight"`                                   // 权重值
	SuccessRate  float64 `gorm:"column:success_rate;type:decimal(5,2);default:100.00" json:"successRate"` // 成功率
	OrderRange   string  `gorm:"column:order_range;type:varchar(100);not null" json:"orderRange"`         // 订单金额范围
}

func (MerchantChannel) TableName() string { return "w_merchant_channel" }
