package service

import (
	"net"
	"strings"
	"wht-order-api/internal/config"
)

type GlobalWhitelistService struct{}

func NewGlobalWhitelistService() *GlobalWhitelistService {
	return &GlobalWhitelistService{}
}

// 单 IP / CIDR / 通配符
func matchRule(ip, rule string) bool {
	if rule == ip {
		return true
	}

	// 前缀通配符 172.16.5.*
	if strings.HasSuffix(rule, "*") {
		prefix := strings.TrimSuffix(rule, "*")
		return strings.HasPrefix(ip, prefix)
	}

	// CIDR
	if _, cidr, err := net.ParseCIDR(rule); err == nil {
		return cidr.Contains(net.ParseIP(ip))
	}

	return false
}

// 是否命中全局白名单
func (s *GlobalWhitelistService) IsGlobal(ip string) bool {
	for _, rule := range config.C.Security.IPWhitelist.Global {
		if matchRule(ip, rule) {
			return true
		}
	}
	return false
}
