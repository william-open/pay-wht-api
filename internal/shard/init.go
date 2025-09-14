package shard

var (
	OrderShard       *ShardEngine
	UpOrderShard     *ShardEngine
	OutOrderShard    *ShardEngine
	UpOutOrderShard  *ShardEngine
	OrderLogShard    *ShardEngine
	OutOrderLogShard *ShardEngine
)

// InitShardEngines 初始化所有分片引擎
func InitShardEngines() {
	OrderShard = NewShardEngine("p_order", 4)
	UpOrderShard = NewShardEngine("p_up_order", 4)
	OutOrderShard = NewShardEngine("p_out_order", 4)
	UpOutOrderShard = NewShardEngine("p_up_out_order", 4)
	OrderLogShard = NewShardEngine("p_order_log", 4)
	OutOrderLogShard = NewShardEngine("p_out_order_log", 4)
}
