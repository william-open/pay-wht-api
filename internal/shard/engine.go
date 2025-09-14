package shard

import (
	"fmt"
	"log"
	"time"
)

// ShardEngine 分表路由器
type ShardEngine struct {
	BaseTable  string
	ShardCount uint32
	Strategy   ShardStrategy
}

// NewShardEngine 创建分片引擎
func NewShardEngine(base string, count uint32) *ShardEngine {
	return &ShardEngine{
		BaseTable:  base,
		ShardCount: count,
		Strategy:   NewCRC32Strategy(count),
	}
}

// GetTable 根据订单号和时间获取分表名
func (e *ShardEngine) GetTable(orderID uint64, t time.Time) string {
	if t.IsZero() || t.Year() < 2000 {
		log.Printf("[ShardEngine] 非法时间: %v，使用当前时间", t)
		t = time.Now()
	}
	month := t.Format("200601")
	shard := e.Strategy.GetShard(orderID)
	return fmt.Sprintf("%s_%s_p%d", e.BaseTable, month, shard)
}
