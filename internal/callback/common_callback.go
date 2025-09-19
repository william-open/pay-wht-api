package callback

import (
	"net"
	"strings"
	"wht-order-api/internal/dao"
)

// verifyUpstreamWhitelist 校验上游供应商IP白名单
func verifyUpstreamWhitelist(upstreamId uint64, ipAddress string) bool {
	var mainDao *dao.MainDao
	mainDao = dao.NewMainDao()
	upstream, err := mainDao.GetUpstreamWhitelist(upstreamId)
	if err != nil || upstream == nil || upstream.Status != 1 {
		return false
	}
	// 构建白名单集合
	allowed := make(map[string]struct{})
	ipList := strings.Split(upstream.IpWhitelist, ",")
	for _, ip := range ipList {
		ip = strings.TrimSpace(ip)
		if net.ParseIP(ip) != nil {
			allowed[ip] = struct{}{}
		}
	}

	// 验证请求 IP 是否允许
	if _, ok := allowed[ipAddress]; !ok {
		return false
	}

	return true
}
