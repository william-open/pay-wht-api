package idgen

import (
	"log"
	"os"
	"strconv"
)

// InitFromEnv 初始化默认节点（支持多实例部署）
func InitFromEnv() {
	nodeIDStr := os.Getenv("SNOWFLAKE_NODE_ID")
	nodeID, err := strconv.ParseInt(nodeIDStr, 10, 64)
	if err != nil || nodeID < 0 || nodeID > 1023 {
		log.Fatalf("[IDGen] Invalid SNOWFLAKE_NODE_ID: %v", nodeIDStr)
	}
	if err := InitNode("default", nodeID); err != nil {
		log.Fatalf("[IDGen] InitNode failed: %v", err)
	}
	log.Printf("[IDGen] Snowflake node initialized: nodeID=%d", nodeID)
}
