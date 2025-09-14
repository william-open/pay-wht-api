package shard

import (
	"fmt"
	"hash/crc32"
)

// CRC32ShardStrategy 使用 CRC32 哈希进行分片
type CRC32ShardStrategy struct {
	ShardCount uint32
}

func NewCRC32Strategy(count uint32) *CRC32ShardStrategy {
	return &CRC32ShardStrategy{ShardCount: count}
}

func (s *CRC32ShardStrategy) GetShard(orderID uint64) int {
	hash := crc32.ChecksumIEEE([]byte(fmt.Sprintf("%d", orderID)))
	return int(hash % s.ShardCount)
}
