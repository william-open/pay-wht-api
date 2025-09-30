package system

import (
	"context"
	"encoding/json"
	"wht-order-api/internal/dal"
	"wht-order-api/internal/dto"
	mainmodel "wht-order-api/internal/model/main"
	rediskey "wht-order-api/internal/types/redis-key"
)

type ConfigSystem struct{}

// 根据参数key获取参数值
func (s *ConfigSystem) GetConfigByConfigKey(configKey string) dto.ConfigDetailResponse {

	var config dto.ConfigDetailResponse

	dal.MainDB.Model(mainmodel.SysConfig{}).Where("config_key = ?", configKey).Last(&config)

	return config
}

// 根据参数key获取参数配置
func (s *ConfigSystem) GetConfigCacheByConfigKey(configKey string) dto.ConfigDetailResponse {

	var config dto.ConfigDetailResponse

	// 缓存不为空不从数据库读取，减少数据库压力
	if configCache, _ := dal.RedisClient.HGet(context.Background(), rediskey.SysConfigKey(), configKey).Result(); configCache != "" {
		if err := json.Unmarshal([]byte(configCache), &config); err == nil {
			return config
		}
	}

	// 从数据库读取配置并且记录到缓存
	config = s.GetConfigByConfigKey(configKey)
	if config.ConfigId > 0 {
		configBytes, _ := json.Marshal(&config)
		dal.RedisClient.HSet(context.Background(), rediskey.SysConfigKey(), configKey, string(configBytes)).Result()
	}

	return config
}
