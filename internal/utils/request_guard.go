package utils

import (
	"strconv"
	"time"
)

// 将毫秒转换为秒 + 纳秒
func ParseTimestamp(tsStr string) (time.Time, error) {
	ms, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	sec := ms / 1000
	nsec := (ms % 1000) * 1e6
	return time.Unix(sec, nsec), nil
}

// 当前时间与请求时间差在合法窗口内（单位秒）
func IsTimestampValid(ts time.Time, window time.Duration) bool {
	now := time.Now()
	diff := now.Sub(ts)
	return diff >= 0 && diff <= window
}

// 检查 nonce 格式
func IsValidNonce(nonce string) bool {
	// 最少8位，只允许数字字母
	if len(nonce) < 8 {
		return false
	}
	for _, r := range nonce {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

func GetTimestampMs() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
