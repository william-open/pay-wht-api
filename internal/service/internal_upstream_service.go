package service

import (
	"context"
	"fmt"
	"golang.org/x/sync/singleflight"
	"strconv"
	"time"
	"wht-order-api/internal/shard"

	"wht-order-api/internal/dao"
	"wht-order-api/internal/dto"
)

type InternalUpstreamService struct {
	mainDao       *dao.MainDao  // 主数据库
	orderDao      *dao.OrderDao //订单数据库
	indexTableDao *dao.IndexTableDao
	merchantGroup singleflight.Group
	channelGroup  singleflight.Group
	ctx           context.Context
	cancel        context.CancelFunc
}

func NewInternalUpstreamService() *InternalUpstreamService {
	ctx, cancel := context.WithCancel(context.Background())
	return &InternalUpstreamService{
		mainDao:       dao.NewMainDao(),
		orderDao:      dao.NewOrderDao(), // 默认全局 DB
		indexTableDao: dao.NewIndexTableDao(),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Get 根据上游交易号查询交易上游供应商配置信息
func (s *InternalUpstreamService) Get(tradeId string) (*dto.UpstreamSupplierDto, error) {
	var resp *dto.UpstreamSupplierDto

	tradeOrderId, err := strconv.ParseUint(tradeId, 10, 64)
	if err != nil {
		return resp, fmt.Errorf("上游交易订单号解析失败,Err:%v", err)
	}

	// 查询订单表
	upOrderTable := shard.UpOrderShard.GetTable(tradeOrderId, time.Now())
	orderData, oErr := s.orderDao.GetTxByUpOrderId(upOrderTable, tradeOrderId)
	if oErr != nil {
		return resp, fmt.Errorf("上游交易订单号,Not Found,Err:%v", oErr)
	}
	// 查询上游供应商配置信息
	upstream, upErr := s.mainDao.GetUpstreamSupplier(orderData.SupplierId)
	if upErr != nil {
		return resp, fmt.Errorf("上游供应商配置信息,Not Found,Err:%v", upErr)
	}

	return upstream, nil
}
