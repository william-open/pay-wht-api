package service

import (
	"errors"
	"wht-order-api/internal/dao"
	"wht-order-api/internal/dto"
)

type CommonService struct {
	mainDao       *dao.MainDao
	orderDao      *dao.OrderDao
	indexTableDao *dao.IndexTableDao
}

func NewCommonService() *CommonService {
	return &CommonService{
		mainDao:       dao.NewMainDao(),
		orderDao:      dao.NewOrderDao(),
		indexTableDao: dao.NewIndexTableDao(),
	}
}

func (s *CommonService) GetMerchantChannelInfo(mid uint64, channelCode string) (*dto.MerchantChannelDTO, error) {

	var detail *dto.MerchantChannelDTO
	// 1) 主库校验
	detail, err := s.mainDao.DetailChannel(mid, channelCode)
	if err != nil {
		return detail, errors.New("merchant channel invalid")
	}

	return detail, nil
}
