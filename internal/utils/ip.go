package utils

import (
	"net"
	"strings"

	"github.com/gin-gonic/gin"
)

// GetRealClientIP 获取客户端真实 IP
func GetRealClientIP(c *gin.Context) string {
	// 优先从 Header 中取（反向代理场景）
	ipHeaders := []string{
		"CF-Connecting-IP", // Cloudflare
		"X-Real-IP",        // Nginx、Caddy
		"X-Forwarded-For",  // 多层代理
		"X-Client-IP",      // 一些代理或负载均衡器
		"X-Forwarded",      // 非标准
		"Forwarded-For",    // 非标准
		"Forwarded",        // RFC7239
	}

	for _, header := range ipHeaders {
		ipList := c.Request.Header.Get(header)
		if ipList == "" {
			continue
		}

		// X-Forwarded-For 可能包含多个IP，用逗号分隔，取第一个非空合法IP
		for _, ip := range strings.Split(ipList, ",") {
			ip = strings.TrimSpace(ip)
			if ip != "" && isValidIP(ip) {
				return ip
			}
		}
	}

	// 从 RemoteAddr 获取（最后兜底）
	ip, _, err := net.SplitHostPort(strings.TrimSpace(c.Request.RemoteAddr))
	if err == nil && isValidIP(ip) {
		return ip
	}

	return ""
}

// 判断 IP 是否为有效 IPv4/IPv6
func isValidIP(ip string) bool {
	parsed := net.ParseIP(ip)
	return parsed != nil
}
