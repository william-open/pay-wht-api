package logger

import (
	"wht-order-api/internal/shard"
)

var OrderLogShard *shard.ShardEngine

func InitLogger() {
	OrderLogShard = shard.NewShardEngine("p_order_log", 4)
}
