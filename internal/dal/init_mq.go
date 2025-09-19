package dal

import (
	"fmt"
	"github.com/streadway/amqp"
	"log"
	"wht-order-api/internal/config"
)

var (
	mqConn    *amqp.Connection
	mqChannel *amqp.Channel
)

// InitRabbitMQ 初始化连接与通道
func InitRabbitMQ() error {
	url := fmt.Sprintf("amqp://%s:%s@%s:%d/%s",
		config.C.RabbitMQ.Username,
		config.C.RabbitMQ.Password,
		config.C.RabbitMQ.Host,
		config.C.RabbitMQ.Port,
		config.C.RabbitMQ.VirtualHost,
	)

	conn, err := amqp.Dial(url)
	if err != nil {
		return fmt.Errorf("RabbitMQ连接失败: %w", err)
	}
	mqConn = conn

	ch, err := conn.Channel()
	if err != nil {
		err := conn.Close()
		if err != nil {
			return err
		} // ✅ 防止资源泄漏
		return fmt.Errorf("RabbitMQ通道创建失败: %w", err)
	}
	mqChannel = ch

	if config.C.RabbitMQ.PrefetchCount > 0 {
		err = ch.Qos(config.C.RabbitMQ.PrefetchCount, 0, false)
		if err != nil {
			return fmt.Errorf("RabbitMQ QoS设置失败: %w", err)
		}
	}

	log.Printf("[RabbitMQ] 初始化完成 → Host: %s, Port: %d, VHost: %s", config.C.RabbitMQ.Host, config.C.RabbitMQ.Port, config.C.RabbitMQ.VirtualHost)
	return nil
}

// GetChannel 获取通道
func GetChannel() *amqp.Channel {
	return mqChannel
}

// GetConnection 获取连接
func GetConnection() *amqp.Connection {
	return mqConn
}
