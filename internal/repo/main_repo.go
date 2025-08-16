package repo

import (
	"wht-order-api/internal/dal"
	mainmodel "wht-order-api/internal/model/main"
)

type MainRepo struct{}

func (r *MainRepo) GetMerchant(mid uint64) (*mainmodel.Merchant, error) {
	var m mainmodel.Merchant
	if err := dal.MainDB.Where("merchant_id=?", mid).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *MainRepo) GetChannel(cid uint64) (*mainmodel.Channel, error) {
	var ch mainmodel.Channel
	if err := dal.MainDB.Where("channel_id=?", cid).First(&ch).Error; err != nil {
		return nil, err
	}
	return &ch, nil
}
