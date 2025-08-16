package dal

import (
	"log"
	"wht-order-api/internal/config"

	"github.com/streadway/amqp"
)

var RabbitConn *amqp.Connection
var RabbitCh *amqp.Channel

func InitRabbitMQ() {
	url := config.C.RabbitMQ.URL
	conn, err := amqp.Dial(url)
	if err != nil {
		log.Fatalf("rabbitmq dial failed: %v", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("rabbitmq channel failed: %v", err)
	}

	// exchange & queues
	if err := ch.ExchangeDeclare("order_events", "topic", true, false, false, false, nil); err != nil {
		log.Fatalf("exchange declare failed: %v", err)
	}
	if _, err := ch.QueueDeclare("order_created", true, false, false, false, nil); err != nil {
		log.Fatalf("queue declare order_created failed: %v", err)
	}
	if _, err := ch.QueueDeclare("channel_callback", true, false, false, false, nil); err != nil {
		log.Fatalf("queue declare channel_callback failed: %v", err)
	}
	if err := ch.QueueBind("order_created", "order.created", "order_events", false, nil); err != nil {
		log.Fatalf("queue bind order_created failed: %v", err)
	}
	if err := ch.QueueBind("channel_callback", "channel.callback", "order_events", false, nil); err != nil {
		log.Fatalf("queue bind channel_callback failed: %v", err)
	}

	RabbitConn = conn
	RabbitCh = ch
}
