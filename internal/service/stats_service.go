package service

import (
	"wht-order-api/internal/dto"
	"wht-order-api/internal/event"
)

type StatsService struct{}

func (s *StatsService) OnOrderCreated(order *dto.OrderMessageMQ) {
	event.PublishOrderStat(order)
}
