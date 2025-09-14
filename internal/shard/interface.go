package shard

// ShardStrategy 定义分片策略接口
type ShardStrategy interface {
	GetShard(orderID uint64) int
}
