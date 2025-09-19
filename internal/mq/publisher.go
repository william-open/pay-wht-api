package mq

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/streadway/amqp"
	"log"
	"wht-order-api/internal/config"
	"wht-order-api/internal/dal"
)

// Publish 发布消息到指定生产者配置的 exchange/routingKey
func Publish(name string, payload any) error {
	ch := dal.GetChannel()
	if ch == nil {
		return errors.New("RabbitMQ 通道未初始化")
	}

	// 查找目标生产者配置
	var target *config.RabbitProducerCfg
	for _, p := range config.C.RabbitMQ.Producers {
		if p.Name == name {
			target = &p
			break
		}
	}
	if target == nil {
		return fmt.Errorf("未找到生产者配置: %s", name)
	}

	// 声明交换机（确保存在）
	err := ch.ExchangeDeclare(
		target.Exchange,
		target.ExchangeType,
		true,  // durable
		false, // autoDelete
		false, // internal
		false, // noWait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("交换机声明失败 [%s]: %w", target.Exchange, err)
	}

	// 序列化消息体
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("消息序列化失败: %w", err)
	}

	// 发布消息
	err = ch.Publish(
		target.Exchange,
		target.RoutingKey,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
	if err != nil {
		return fmt.Errorf("消息发布失败 [%s → %s]: %w", target.Exchange, target.RoutingKey, err)
	}

	log.Printf("[MQ] ✅ 消息已发布 [%s → %s]: %s", target.Name, target.RoutingKey, string(body))
	return nil
}
