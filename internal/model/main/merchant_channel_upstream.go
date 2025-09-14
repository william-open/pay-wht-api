package mainmodel

type MerchantChannelUpstream struct {
	ID             int    `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	MID            int64  `gorm:"column:m_id;not null;comment:商户ID" json:"mId"`
	Currency       string `gorm:"column:currency;type:varchar(30);default:null;comment:货币符号" json:"currency"`
	SysChannelID   int64  `gorm:"column:sys_channel_id;not null;comment:系统通道编码ID" json:"sysChannelID"`
	SysChannelCode string `gorm:"column:sys_channel_code;type:varchar(30);default:null;comment:系统通道编码" json:"sysChannelCode"`
	UpChannelID    int64  `gorm:"column:up_channel_id;not null;comment:上游通道产品ID"  json:"upChannelID"`
	UpChannelCode  string `gorm:"column:up_channel_code;type:varchar(30);default:null;comment:上游通道编码"  json:"upChannelCode"`
	DispatchMode   uint8  `gorm:"column:dispatch_mode;type:tinyint unsigned;default:1;comment:调度模式：1=轮询权重，2=固定通道"  json:"dispatchMode"`
	Type           int8   `gorm:"column:type;type:tinyint(1);not null;comment:1:代收2:代付"  json:"type"`
	Status         int8   `gorm:"column:status;type:tinyint(1);default:0;comment:1:开启0:关闭"  json:"status"`
	Weight         int    `gorm:"column:weight;default:0;comment:权重值" json:"weight"`
}

// TableName sets the table name for GORM
func (MerchantChannelUpstream) TableName() string {
	return "w_merchant_channel_upstream"
}
