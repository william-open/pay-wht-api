package idgen

import "github.com/bwmarrin/snowflake"

var node *snowflake.Node

func Init(nodeID int64) {
	n, err := snowflake.NewNode(nodeID)
	if err != nil {
		panic(err)
	}
	node = n
}
func New() uint64 { return uint64(node.Generate().Int64()) }
