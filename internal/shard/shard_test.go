package shard

import (
	"testing"
	"time"
)

func TestCRC32ShardStrategy(t *testing.T) {
	strategy := NewCRC32Strategy(4)
	orderID := uint64(123456789)
	shard := strategy.GetShard(orderID)
	if shard < 0 || shard >= 4 {
		t.Errorf("Shard out of range: %d", shard)
	}
}

func TestShardEngine_GetTable(t *testing.T) {
	engine := NewShardEngine("p_order_log", 4)
	orderID := uint64(987654321)
	timestamp := time.Date(2025, 9, 12, 12, 0, 0, 0, time.Local)
	table := engine.GetTable(orderID, timestamp)

	expectedPrefix := "p_order_log_202509_p"
	if len(table) < len(expectedPrefix) || table[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("Unexpected table name: %s", table)
	}
}
