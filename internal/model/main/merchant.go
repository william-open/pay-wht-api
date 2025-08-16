package mainmodel

type Merchant struct {
	MerchantID uint64 `gorm:"column:merchant_id;primaryKey"`
	Name       string `gorm:"column:name"`
	Status     int8   `gorm:"column:status"`
	ApiKey     string `gorm:"column:api_key"`
	ApiSecret  string `gorm:"column:api_secret"`
}

func (Merchant) TableName() string { return "merchant" }
