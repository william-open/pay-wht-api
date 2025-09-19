package mainmodel

import "time"

type SysConfig struct {
	ConfigId    int `gorm:"primaryKey;autoIncrement"`
	ConfigName  string
	ConfigKey   string
	ConfigValue string
	ConfigType  string `gorm:"default:N"`
	CreateBy    string
	CreateTime  time.Time `gorm:"autoCreateTime"`
	UpdateBy    string
	UpdateTime  time.Time `gorm:"autoUpdateTime"`
	Remark      string
}

func (SysConfig) TableName() string {
	return "sys_config"
}
