package utils

import (
	"log"
	"time"
	"wht-order-api/internal/dal"
)

// WeightedNode 表示一个有权节点
type WeightedNode struct {
	ID      int64
	Weight  int
	Current int
}

// SmoothWeightedRR 平滑加权轮询（Redis 持久化状态）
// redisKey: 全局唯一状态键，例如 rr_state:COLLECT:BRL
// weights: map[通道ID]权重
// 返回被选中的通道ID
func SmoothWeightedRR(redisKey string, weights map[int64]int) int64 {
	if len(weights) == 0 {
		return 0
	}

	// 尝试读取历史状态
	stateJSON, _ := dal.RedisClient.Get(dal.RedisCtx, redisKey).Result()
	last := map[int64]int{}
	if stateJSON != "" {
		if err := JSONToMap(stateJSON, &last); err != nil {
			log.Printf("[SW-RR] 解析Redis状态失败: %v", err)
		}
	}

	// 构建节点
	var nodes []WeightedNode
	var total int
	for id, w := range weights {
		if w <= 0 {
			continue
		}
		total += w
		nodes = append(nodes, WeightedNode{ID: id, Weight: w, Current: last[id] + w})
	}

	if len(nodes) == 0 {
		return 0
	}

	// 找出当前最大 current 的节点
	var best *WeightedNode
	for i := range nodes {
		if best == nil || nodes[i].Current > best.Current {
			best = &nodes[i]
		}
	}

	if best == nil {
		return 0
	}

	// 更新状态
	best.Current -= total
	for _, n := range nodes {
		last[n.ID] = n.Current
	}

	// 写回Redis
	if err := dal.RedisClient.Set(dal.RedisCtx, redisKey, MapToJSON(last), 10*time.Minute).Err(); err != nil {
		log.Printf("[SW-RR] Redis写入失败: %v", err)
	}

	return best.ID
}
