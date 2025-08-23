package utils

import (
	"encoding/json"
	"fmt"
	"github.com/shopspring/decimal"
	"strings"
	"time"
)

// MatchOrderRange 判断金额是否符合 orderRange 规则
func MatchOrderRange(amount decimal.Decimal, orderRange string) bool {
	rules := strings.Split(orderRange, ",")
	for _, rule := range rules {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}
		if strings.Contains(rule, "-") {
			// 区间规则
			bounds := strings.Split(rule, "-")
			if len(bounds) != 2 {
				continue
			}
			min, err1 := decimal.NewFromString(bounds[0])
			max, err2 := decimal.NewFromString(bounds[1])
			if err1 != nil || err2 != nil {
				continue
			}
			if amount.Cmp(min) >= 0 && amount.Cmp(max) <= 0 {
				return true
			}
		} else {
			// 固定金额规则
			val, err := decimal.NewFromString(rule)
			if err != nil {
				continue
			}
			if amount.Cmp(val) == 0 {
				return true
			}
		}
	}
	return false
}

// MapToJSON map转出为json
func MapToJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// 分片表名生成器：p_order_{YYYYMM}_p{orderID % 4} 和 p_out_order_{YYYYMM}_p{orderID % 4}
func GetShardOrderTable(base string, orderID uint64, t time.Time) string {
	month := t.Format("200601")
	shard := orderID % 4
	return fmt.Sprintf("%s_%s_p%d", base, month, shard)
}

// 分片表名生成器：p_order_index_{YYYYMM}
func GetOrderIndexTable(base string, t time.Time) string {
	month := t.Format("200601")
	return fmt.Sprintf("%s_%s", base, month)
}
