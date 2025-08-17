package mainmodel

type Merchant struct {
	MerchantID uint64 `gorm:"column:m_id;primaryKey"`
	NickName   string `gorm:"column:nickname"`
	AppId      string `gorm:"column:app_id"`
	Status     int8   `gorm:"column:status"`
	UserType   int8   `gorm:"column:user_type"`
	PId        uint64 `gorm:"column:pid"`
	PayType    int8   `gorm:"pay_type"`
	ApiKey     string `gorm:"column:api_key"`
	ApiIp      string `gorm:"column:api_ip"`
}

func (Merchant) TableName() string { return "w_merchant" }
