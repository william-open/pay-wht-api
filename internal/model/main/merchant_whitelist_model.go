package mainmodel

type MerchantWhitelist struct {
	ID         uint64 `gorm:"primaryKey;autoIncrement;comment:主键"`
	MID        uint64 `gorm:"column:m_id;not null;comment:商户ID"`
	IPAddress  string `gorm:"size:32;not null;comment:IP地址"`
	CanAdmin   uint8  `gorm:"not null;default:0;comment:登录后台权限"`
	CanPayout  uint8  `gorm:"not null;default:0;comment:代付下单"`
	CanReceive uint8  `gorm:"not null;default:0;comment:代收下单"`
}

func (MerchantWhitelist) TableName() string { return "w_merchant_whitelist" }
