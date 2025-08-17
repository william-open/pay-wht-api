package service

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"wht-order-api/internal/dal"
	"wht-order-api/internal/dto"
	"wht-order-api/internal/idgen"
	ordermodel "wht-order-api/internal/model/order"
	"wht-order-api/internal/mq"
	"wht-order-api/internal/repo"
	"wht-order-api/internal/shard"
)

type OrderService struct {
	mainRepo  *repo.MainRepo
	orderRepo *repo.OrderRepo
}

func NewOrderService() *OrderService {
	return &OrderService{mainRepo: &repo.MainRepo{}, orderRepo: &repo.OrderRepo{}}
}

func (s *OrderService) Create(req dto.CreateOrderReq) (uint64, error) {
	// 1) 主库校验
	merchant, err := s.mainRepo.GetMerchant(req.MerchantID)
	if err != nil || merchant == nil || merchant.Status != 1 {
		return 0, errors.New("merchant invalid")
	}
	channel, err := s.mainRepo.GetChannel(req.ChannelID)
	if err != nil || channel == nil || channel.Status != 1 {
		return 0, errors.New("channel invalid")
	}

	now := time.Now()
	oid := idgen.New()
	table := shard.Table("merchant_order", now, oid)

	// 2) 幂等
	if exist, _ := s.orderRepo.GetByMerchantNo(table, req.MerchantID, req.MerchantOrdNo); exist != nil {
		return exist.OrderID, nil
	}

	// 3) 插入
	m := &ordermodel.MerchantOrder{
		OrderID:       oid,
		MerchantID:    req.MerchantID,
		MerchantOrdNo: req.MerchantOrdNo,
		Amount:        req.Amount,
		Currency:      req.Currency,
		PayMethod:     req.PayMethod,
		Status:        0,
		NotifyURL:     req.NotifyURL,
		ChannelID:     &req.ChannelID,
		Ext:           req.Ext,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.orderRepo.Insert(table, m); err != nil {
		return 0, err
	}

	// 4) 缓存到 redis（短期）
	cacheKey := "order:" + strconv.FormatUint(oid, 10)
	_ = dal.RedisClient.Set(dal.RedisCtx, cacheKey, mapToJSON(m), 10*time.Minute).Err()

	// 5) 发布 MQ 事件
	evt := mq.OrderCreatedEvent{
		OrderID: oid, MerchantID: req.MerchantID, MerchantOrdNo: req.MerchantOrdNo,
		Amount: req.Amount, Currency: req.Currency, PayMethod: req.PayMethod, CreatedAt: now.Unix(),
	}
	_ = mq.PublishOrderCreated(evt)

	return oid, nil
}

func mapToJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func (s *OrderService) Get(id uint64) (*ordermodel.MerchantOrder, error) {
	// 优先读 redis
	cacheKey := "order:" + strconv.FormatUint(id, 10)
	if sjson, err := dal.RedisClient.Get(dal.RedisCtx, cacheKey).Result(); err == nil {
		var mo ordermodel.MerchantOrder
		if err := json.Unmarshal([]byte(sjson), &mo); err == nil {
			return &mo, nil
		}
	}
	// fallback DB (current month for demo)
	table := shard.Table("merchant_order", time.Now(), id)
	return s.orderRepo.GetByID(table, id)
}
