package shard

import (
	"sync"
)

// strategyRegistry 用于注册和切换策略
var (
	strategyRegistry = make(map[string]ShardStrategy)
	activeStrategy   ShardStrategy
	mu               sync.RWMutex
)

// RegisterStrategy 注册新的策略
func RegisterStrategy(name string, strategy ShardStrategy) {
	mu.Lock()
	defer mu.Unlock()
	strategyRegistry[name] = strategy
}

// UseStrategy 切换当前使用的策略
func UseStrategy(name string) bool {
	mu.Lock()
	defer mu.Unlock()
	s, ok := strategyRegistry[name]
	if ok {
		activeStrategy = s
	}
	return ok
}

// GetActiveStrategy 获取当前策略
func GetActiveStrategy() ShardStrategy {
	mu.RLock()
	defer mu.RUnlock()
	if activeStrategy == nil {
		return NewCRC32Strategy(4) // 默认策略
	}
	return activeStrategy
}
