package event

import "wht-order-api/internal/dto"

type Publisher interface {
	Publish(topic string, msg any) error
}

type EventHandler interface {
	HandleOrderCreated(e dto.OrderMessageMQ) error
}
