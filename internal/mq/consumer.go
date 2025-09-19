package mq

import (
	"github.com/streadway/amqp"
	"log"
	"wht-order-api/internal/config"
	"wht-order-api/internal/dal"
)

func StartConsumer(name string, handler func(amqp.Delivery)) {
	var cfg *config.RabbitConsumerCfg
	for _, c := range config.C.RabbitMQ.Consumers {
		if c.Name == name {
			cfg = &c
			break
		}
	}
	if cfg == nil {
		log.Printf("❌ 未找到消费者配置: %s", name)
		return
	}

	ch := dal.GetChannel()
	if ch == nil {
		log.Println("❌ RabbitMQ 通道未初始化")
		return
	}

	// 声明交换机
	err := ch.ExchangeDeclare(
		cfg.Exchange,
		cfg.ExchangeType,
		cfg.Durable,
		cfg.AutoDelete,
		false,
		cfg.NoWait,
		nil,
	)
	if err != nil {
		log.Printf("❌ ExchangeDeclare 失败 [%s]: %v", cfg.Exchange, err)
		return
	}

	// 声明队列
	_, err = ch.QueueDeclare(
		cfg.Queue,
		cfg.Durable,
		cfg.AutoDelete,
		cfg.Exclusive,
		cfg.NoWait,
		nil,
	)
	if err != nil {
		log.Printf("❌ QueueDeclare 失败 [%s]: %v", cfg.Queue, err)
		return
	}

	// 绑定队列
	err = ch.QueueBind(
		cfg.Queue,
		cfg.RoutingKey,
		cfg.Exchange,
		cfg.NoWait,
		nil,
	)
	if err != nil {
		log.Printf("❌ QueueBind 失败 [%s → %s]: %v", cfg.Queue, cfg.Exchange, err)
		return
	}

	// 开始消费
	msgs, err := ch.Consume(
		cfg.Queue,
		"",
		false, false, false, false, nil,
	)
	if err != nil {
		log.Printf("❌ Consume 失败 [%s]: %v", cfg.Queue, err)
		return
	}

	log.Printf("✅ [%s] 正在监听队列: %s", name, cfg.Queue)

	for d := range msgs {
		go handler(d)
	}
}
