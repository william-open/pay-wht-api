package rediskey

import "wht-order-api/internal/config"

var (

	// 配置表数据 redis key
	SysConfigKey = config.C.Project.Name + ":system:config"

	// 字典表数据 redis key
	SysDictKey = config.C.Project.Name + ":system:dict:data"
)
