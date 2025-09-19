package mainmodel

type CurrencyCode struct {
	ID      uint64 `gorm:"primaryKey;autoIncrement;comment:主键"`
	Code    string `gorm:"type:varchar(10);uniqueIndex;not null;comment:币种代码（ISO 4217）"`
	NameEn  string `gorm:"type:varchar(100);not null;comment:英文名称"`
	NameZh  string `gorm:"type:varchar(100);not null;comment:中文名称"`
	Symbol  string `gorm:"type:varchar(10);comment:货币符号"`
	Country string `gorm:"type:varchar(100);comment:主要使用国家或地区"`
	IsOpen  uint8  `gorm:"type:tinyint(1);default:0;not null;comment:是否开启:0禁用，1启动"`
}

func (CurrencyCode) TableName() string { return "w_currency_code" }
