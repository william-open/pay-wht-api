package mq

import (
	"encoding/json"
	"fmt"
	"github.com/streadway/amqp"
	"log"
	"time"
	"wht-order-api/internal/config"
	"wht-order-api/internal/dal"
	"wht-order-api/internal/event"
)

type Publisher struct{}

func NewPublisher() event.Publisher {
	// 确保 MQ 已初始化（可在 main 里先调 dal.InitRabbitMQ()）
	return &Publisher{}
}

// Publish 发布消息到指定生产者配置的 exchange/routingKey
func (p *Publisher) Publish(name string, payload any) error {
	target := findProducer(name)
	if target == nil {
		return fmt.Errorf("未找到生产者配置: %s", name)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("消息序列化失败: %w", err)
	}

	// 尝试发布（最多两次，第二次会触发重连）
	if err := p.publishOnce(target, body); err != nil {
		// 若通道/连接问题，触发自愈后再试一次
		log.Printf("[MQ] ⚠️ 发布失败，尝试自愈后重试: %v", err)
		// 触发自愈：拿一次 channel（内部会重连）
		if ch := dal.GetChannel(); ch == nil {
			return fmt.Errorf("自愈失败：无法获取 MQ 通道: %w", err)
		}
		if err2 := p.publishOnce(target, body); err2 != nil {
			return fmt.Errorf("发布失败（重试后）[%s→%s]: %w", target.Exchange, target.RoutingKey, err2)
		}
	}

	log.Printf("[MQ] ✅ 成功发布 [%s → %s]: %s", target.Name, target.RoutingKey, string(body))
	return nil
}

func (p *Publisher) publishOnce(target *config.RabbitProducerCfg, body []byte) error {
	ch := dal.GetChannel()
	if ch == nil {
		return fmt.Errorf("RabbitMQ 通道不可用")
	}

	// 幂等声明交换机
	if err := ch.ExchangeDeclare(
		target.Exchange,
		target.ExchangeType,
		true,  // durable
		false, // auto-delete
		false, // internal
		false, // no-wait
		nil,
	); err != nil {
		return fmt.Errorf("交换机声明失败 [%s]: %w", target.Exchange, err)
	}

	pub := amqp.Publishing{
		ContentType:  "application/json",
		Body:         body,
		DeliveryMode: amqp.Persistent, // 持久化
		Timestamp:    time.Now(),
	}

	if err := ch.Publish(
		target.Exchange,
		target.RoutingKey,
		false, // mandatory
		false, // immediate
		pub,
	); err != nil {
		return fmt.Errorf("Publish 错误: %w", err)
	}

	return nil
}

func findProducer(name string) *config.RabbitProducerCfg {
	for _, v := range config.C.RabbitMQ.Producers {
		if v.Name == name {
			cp := v
			return &cp
		}
	}
	return nil
}
