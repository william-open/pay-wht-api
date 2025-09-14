package service

import (
	"net"
	"strings"
	"wht-order-api/internal/dao"
)

type VerifyService struct {
	mainDao *dao.MainDao
}

func NewVerifyIpWhitelistService() *VerifyService {
	return &VerifyService{
		mainDao: dao.NewMainDao(),
	}
}

// VerifyIpWhitelist 验证白名单
func (s *VerifyService) VerifyIpWhitelist(ipAddress string, mId uint64, mode int8) bool {

	// 查询白名单 IP
	whitelist, err := s.mainDao.GetMerchantWhitelist(mId, mode)
	if err != nil {
		return false
	}

	// 构建白名单集合
	allowed := make(map[string]struct{})
	for _, entry := range whitelist {
		ipList := strings.Split(entry.IPAddress, ",")
		for _, ip := range ipList {
			ip = strings.TrimSpace(ip)
			if net.ParseIP(ip) != nil {
				allowed[ip] = struct{}{}
			}
		}
	}

	// 验证请求 IP 是否允许
	if _, ok := allowed[ipAddress]; !ok {
		return false
	}
	return true
}

// VerifyChannelValid 验证通道是否开启或者有效
func (s *VerifyService) VerifyChannelValid(mId uint64, channelCode string) bool {

	err := s.mainDao.CheckChannelValid(mId, channelCode)
	if err != nil {
		return false
	}
	return true
}
