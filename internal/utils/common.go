package utils

import (
	"encoding/json"
	"strconv"
	"strings"
)

// MatchOrderRange 判断金额是否符合 orderRange 规则
func MatchOrderRange(amount float64, orderRange string) bool {
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
			min, err1 := strconv.ParseFloat(bounds[0], 64)
			max, err2 := strconv.ParseFloat(bounds[1], 64)
			if err1 != nil || err2 != nil {
				continue
			}
			if amount >= min && amount <= max {
				return true
			}
		} else {
			// 固定金额规则
			val, err := strconv.ParseFloat(rule, 64)
			if err != nil {
				continue
			}
			if amount == val {
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
