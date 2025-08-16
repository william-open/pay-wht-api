package shard

import (
	"fmt"
	"time"

	"wht-order-api/internal/config"
)

// Table returns table name like merchant_order_YYYYMM_p0
func Table(base string, ts time.Time, id uint64) string {
	month := ts.Format("200601")
	n := config.C.Order.ShardsPerMonth
	idx := int(id % uint64(n))
	return fmt.Sprintf("%s_%s_p%d", base, month, idx)
}

// AllTables returns all tables for current month
func AllTables(base string, ts time.Time) []string {
	month := ts.Format("200601")
	n := config.C.Order.ShardsPerMonth
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, fmt.Sprintf("%s_%s_p%d", base, month, i))
	}
	return out
}
