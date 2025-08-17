package mainmodel

type Channel struct {
	Id       uint64 `gorm:"column:id;primaryKey"`
	Title    string `gorm:"column:title"`
	Currency string `gorm:"column:currency"`
	Coding   string `gorm:"column:coding"`
	Type     int8   `gorm:"column:type"`
	Status   int8   `gorm:"column:status"`
}

func (Channel) TableName() string { return "w_pay_way" }
