package mainmodel

import "github.com/shopspring/decimal"

type PayProduct struct {
	ID             int             `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	Title          string          `gorm:"column:title;type:varchar(40);not null;comment:通道产品名称" json:"title"`
	Currency       string          `gorm:"column:currency;type:varchar(30);default:null;comment:国家货币符号" json:"currency"`
	Type           int8            `gorm:"column:type;type:tinyint;default:null;comment:通道类型：1:代收2:代付" json:"type"`
	UpstreamID     int             `gorm:"column:upstream_id;not null;comment:供应商ID" json:"upstreamId"`
	UpstreamCode   string          `gorm:"column:upstream_code;type:varchar(100);not null;comment:上游通道编码" json:"upstreamCode"`
	InterfaceID    int             `gorm:"column:interface_id;not null;comment:接口ID" json:"interfaceId"`
	SysChannelID   int64           `gorm:"column:sys_channel_id;not null;comment:系统通道编码ID" json:"sysChannelId"`
	SysChannelCode string          `gorm:"column:sys_channel_code;type:varchar(30);default:null;comment:系统通道编码" json:"sysChannelCode"`
	Status         int8            `gorm:"column:status;type:tinyint(1);default:0;comment:1:开启0:关闭" json:"status"`
	CostRate       decimal.Decimal `gorm:"column:cost_rate;type:decimal(4,2);not null;default:0.00;comment:成本费率" json:"costRate"`
	CostFee        decimal.Decimal `gorm:"column:cost_fee;type:decimal(4,2);not null;default:0.00;comment:成本单笔费用" json:"costFee"`
	MinAmount      decimal.Decimal `gorm:"column:min_amount;type:decimal(20,2);default:null;comment:最小金额" json:"minAmount"`
	MaxAmount      decimal.Decimal `gorm:"column:max_amount;type:decimal(20,2);default:null;comment:最大金额" json:"maxAmount"`
	FixedAmount    string          `gorm:"column:fixed_amount;type:varchar(100);not null;comment:固定金额,多个使用英文逗号连接" json:"fixedAmount"`
	SuccessRate    decimal.Decimal `gorm:"column:success_rate;type:decimal(5,2);default:100.00;comment:成功率" json:"successRate"`
}

// TableName sets the table name for GORM
func (PayProduct) TableName() string {
	return "w_pay_product"
}
