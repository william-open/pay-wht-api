package utils

import (
	"crypto/md5"
	"encoding/hex"
	"sort"
	"strings"
)

// GenerateSign 生成签名（用于请求或验证）
func GenerateSign(params map[string]string, secretKey string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "sign" || strings.TrimSpace(v) == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for i, k := range keys {
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(params[k])
		if i < len(keys)-1 {
			sb.WriteString("&")
		}
	}
	sb.WriteString("&key=")
	sb.WriteString(secretKey)

	//log.Printf("签名query字符串:%v", sb.String())
	hash := md5.Sum([]byte(sb.String()))
	signStr := strings.ToUpper(hex.EncodeToString(hash[:]))
	//log.Printf("签名值: %v", signStr)
	return signStr
}

// VerifySign 验证签名是否匹配
func VerifySign(params map[string]string, secretKey string) bool {
	receivedSign := params["sign"]
	if receivedSign == "" {
		return false
	}
	expectedSign := GenerateSign(params, secretKey)
	return strings.EqualFold(receivedSign, expectedSign)
}
