package rediskey

import "wht-order-api/internal/config"

// 获取配置表 Redis Key
func SysConfigKey() string {
	return config.C.Project.Name + ":system:config"
}

// 获取字典表 Redis Key
func SysDictKey() string {
	return config.C.Project.Name + ":system:dict:data"
}
