package idgen

import (
	"fmt"
	"github.com/bwmarrin/snowflake"
	"log"
	"sync"
	"time"
)

var (
	nodeMap sync.Map // map[string]*snowflake.Node
)

// InitNode 初始化指定名称的 Snowflake 节点
func InitNode(name string, nodeID int64) error {
	n, err := snowflake.NewNode(nodeID)
	if err != nil {
		return fmt.Errorf("InitNode failed: %w", err)
	}
	nodeMap.Store(name, n)
	return nil
}

// NewFrom 生成指定节点的 ID
func NewFrom(name string) uint64 {
	val, ok := nodeMap.Load(name)
	if !ok {
		panic(fmt.Sprintf("Snowflake node not initialized: %s", name))
	}
	return uint64(val.(*snowflake.Node).Generate().Int64())
}

// New 默认节点生成器（"default"）
func New() uint64 {
	return NewFrom("default")
}

// CheckSystemClock 时间回拨保护机制,snowflake 本身不防止时间回拨，但你可以加一个守护模块
func CheckSystemClock() {
	last := time.Now().UnixMilli()
	ticker := time.NewTicker(time.Second)
	for now := range ticker.C {
		current := now.UnixMilli()
		if current < last {
			log.Fatalf("[IDGen] System clock moved backward: last=%d, now=%d", last, current)
		}
		last = current
	}
}
