package service

import (
	"errors"
	"log"
	"wht-order-api/internal/dao"
	"wht-order-api/internal/dto"
)

type AccountService struct {
	mainDao  *dao.MainDao
	orderDao *dao.OrderDao
}

func NewAccountService() *AccountService {
	return &AccountService{
		mainDao:  dao.NewMainDao(),
		orderDao: dao.NewOrderDao(),
	}
}

func (s *AccountService) Get(mId string, currency string) (dto.AccountResp, error) {
	var resp dto.AccountResp
	// 查询商户信息
	mainDao := dao.NewMainDao()
	merchant, _ := mainDao.GetMerchant(mId)
	if merchant.Status != 1 {
		log.Printf("商户不存在: %v", merchant)
		return resp, errors.New("商户不存在")
	}

	resp, _ = s.mainDao.GetAccountDetail(merchant.MerchantID, currency)

	return resp, nil
}
