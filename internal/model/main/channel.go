package mainmodel

type Channel struct {
	ChannelID  uint64 `gorm:"column:channel_id;primaryKey"`
	Name       string `gorm:"column:name"`
	Type       string `gorm:"column:type"`
	Status     int8   `gorm:"column:status"`
	ConfigJSON string `gorm:"column:config_json"`
}

func (Channel) TableName() string { return "payment_channel" }
