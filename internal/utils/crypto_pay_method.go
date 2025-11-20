package utils

import (
	"fmt"
	"strings"
)

type PayMethodInfo struct {
	PayMethod string
	Currency  string
	Chain     string
	Protocol  string
}

// 全量映射表（最终版）
var PayMethodMap = map[string]PayMethodInfo{
	// TRON
	"USDT_TRC20": {"USDT_TRC20", "USDT", "TRON", "TRC20"},
	"USDC_TRC20": {"USDC_TRC20", "USDC", "TRON", "TRC20"},

	// ETH
	"USDT_ERC20": {"USDT_ERC20", "USDT", "ETH", "ERC20"},
	"USDC_ERC20": {"USDC_ERC20", "USDC", "ETH", "ERC20"},

	// BSC
	"USDT_BEP20": {"USDT_BEP20", "USDT", "BSC", "BEP20"},
	"USDC_BEP20": {"USDC_BEP20", "USDC", "BSC", "BEP20"},

	// Polygon
	"USDT_POLYGON":        {"USDT_POLYGON", "USDT", "POLYGON", "ERC20"},
	"USDC_POLYGON":        {"USDC_POLYGON", "USDC", "POLYGON", "ERC20"},
	"USDC_POLYGON_NATIVE": {"USDC_POLYGON_NATIVE", "USDC", "POLYGON", "NATIVE"},

	// Solana
	"USDT_SPL": {"USDT_SPL", "USDT", "SOLANA", "SPL"},
	"USDC_SPL": {"USDC_SPL", "USDC", "SOLANA", "SPL"},
}

// 根据 pay_method 获取三要素：币种、链、协议
func ParsePayMethod(payMethod string) (PayMethodInfo, error) {
	info, ok := PayMethodMap[payMethod]
	if !ok {
		return PayMethodInfo{}, fmt.Errorf("unsupported pay_method: %s", payMethod)
	}
	return info, nil
}

// 工具函数（可选）
func IsTRC20(payMethod string) bool {
	info, ok := PayMethodMap[payMethod]
	return ok && info.Protocol == "TRC20"
}

func IsERC20(payMethod string) bool {
	info, ok := PayMethodMap[payMethod]
	return ok && info.Protocol == "ERC20"
}

func IsSPL(payMethod string) bool {
	info, ok := PayMethodMap[payMethod]
	return ok && info.Protocol == "SPL"
}

func IsNative(payMethod string) bool {
	info, ok := PayMethodMap[payMethod]
	return ok && info.Protocol == "NATIVE"
}

func IsCryptoCurrency(currency string) bool {
	c := strings.ToUpper(currency)
	return c == "USDT" || c == "USDC" || c == "BTC" || c == "SOL" || c == "MATIC" || c == "TRX" || c == "ETH" || c == "BNB" // 你可扩展
}

func InArray(item string, arr []string) bool {
	for _, v := range arr {
		if strings.EqualFold(v, item) {
			return true
		}
	}
	return false
}

// CryptoPayMethods 根据币种列出支持的 payMethod
var CryptoPayMethods = map[string][]string{
	"USDT": {
		"USDT_TRC20",
		"USDT_ERC20",
		"USDT_BEP20",
		"USDT_POLYGON",
		"USDT_SPL",
	},

	"USDC": {
		"USDC_TRC20",
		"USDC_ERC20",
		"USDC_BEP20",
		"USDC_POLYGON",
		"USDC_POLYGON_NATIVE",
		"USDC_SPL",
	},
}
