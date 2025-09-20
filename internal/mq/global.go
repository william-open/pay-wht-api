package mq

import "wht-order-api/internal/event"

var GlobalPublisher event.Publisher

func InitPublisher() {
	GlobalPublisher = NewPublisher()
}
