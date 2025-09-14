package mainmodel

import "github.com/shopspring/decimal"

// MerchantChannel represents w_merchant_channel
type MerchantChannel struct {
	ID             uint64          `gorm:"column:id;primaryKey;autoIncrement" json:"id"`                                                   // 主键
	Type           int8            `gorm:"column:type;not null;comment:1:代收2:代付" json:"type"`                                              // 通道类型
	MID            uint64          `gorm:"column:m_id;not null" json:"mId"`                                                                // 商户ID
	Currency       string          `gorm:"column:currency;type:varchar(30)" json:"currency"`                                               // 货币符号
	SysChannelID   uint64          `gorm:"column:sys_channel_id;not null" json:"sysChannelId"`                                             // 系统通道编码ID
	SysChannelCode string          `gorm:"column:sys_channel_code;not null" json:"sysChannelCode"`                                         // 系统通道编码
	DispatchMode   int8            `gorm:"column:dispatch_mode;type:tinyint(1);default:1;comment:调度模式：1=轮询权重，2=固定通道"  json:"dispatchMode"` //派单模式
	Status         int8            `gorm:"column:status;type:tinyint(1);default:0" json:"status"`                                          // 1:开启0:关闭
	DefaultRate    decimal.Decimal `gorm:"column:default_rate;type:decimal(4,2);default:0.00" json:"defaultRate"`                          // 默认费率
	SingleFee      decimal.Decimal `gorm:"column:single_fee;type:decimal(4,2);default:0.00" json:"singleFee"`                              // 单笔费用
}

func (MerchantChannel) TableName() string { return "w_merchant_channel" }
