package health

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

type ChannelHealthManager struct {
	Redis     *redis.Client
	Strategy  SuccessRateStrategy
	Threshold float64 // 熔断阈值，例如 60.0
	TTL       time.Duration
}

func (m *ChannelHealthManager) Update(productID int64, success bool) error {
	ctx := context.Background()
	key := fmt.Sprintf("pay_product:success_rate:%d", productID)

	currentRate, err := m.Redis.Get(ctx, key).Float64()
	if err != nil {
		currentRate = 100.0
	}

	newRate := m.Strategy.Update(currentRate, success)
	if newRate < m.Threshold {
		// 熔断标记
		_ = m.Redis.Set(ctx, m.disabledKey(productID), 1, m.TTL).Err()
	}

	// 更新成功率缓存
	return m.Redis.Set(ctx, key, newRate, m.TTL).Err()
}

func (m *ChannelHealthManager) IsDisabled(productID int64) bool {
	ctx := context.Background()
	val, err := m.Redis.Get(ctx, m.disabledKey(productID)).Int()
	return err == nil && val == 1
}

func (m *ChannelHealthManager) disabledKey(productID int64) string {
	return fmt.Sprintf("pay_product:disabled:%d", productID)
}
