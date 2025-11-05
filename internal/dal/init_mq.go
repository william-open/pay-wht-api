package dal

import (
	"fmt"
	"github.com/streadway/amqp"
	"log"
	"sync"
	"time"
	"wht-order-api/internal/config"
)

var (
	mqConn    *amqp.Connection
	mqChannel *amqp.Channel

	mu sync.Mutex

	// 用 NotifyClose 事件来判断是否已关闭（而不是 IsClosed）
	connClosedCh chan *amqp.Error
	chClosedCh   chan *amqp.Error

	reconnecting bool
)

// InitRabbitMQ 初始化（首次连接）
func InitRabbitMQ() error {
	return connect()
}

// -------- 内部：连接与自愈 --------

func connect() error {
	mu.Lock()
	defer mu.Unlock()

	// 若已连通则直接返回（用 isAlive 判断）
	if isConnAlive() && isChanAlive() {
		return nil
	}

	url := fmt.Sprintf("amqp://%s:%s@%s:%d/%s",
		config.C.RabbitMQ.Username,
		config.C.RabbitMQ.Password,
		config.C.RabbitMQ.Host,
		config.C.RabbitMQ.Port,
		config.C.RabbitMQ.VirtualHost,
	)
	log.Printf("[RabbitMQ] 🌀 连接中: %s", url)

	conn, err := amqp.Dial(url)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	mqConn = conn
	connClosedCh = conn.NotifyClose(make(chan *amqp.Error, 1))

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		mqConn = nil
		connClosedCh = nil
		return fmt.Errorf("创建通道失败: %w", err)
	}
	mqChannel = ch
	chClosedCh = ch.NotifyClose(make(chan *amqp.Error, 1))

	// QoS（可选）
	if pc := config.C.RabbitMQ.PrefetchCount; pc > 0 {
		if err := ch.Qos(pc, 0, false); err != nil {
			log.Printf("[RabbitMQ] ⚠️ 设置 QoS 失败: %v", err)
		}
	}

	log.Printf("[RabbitMQ] ✅ 初始化成功 → Host=%s Port=%d VHost=%s",
		config.C.RabbitMQ.Host, config.C.RabbitMQ.Port, config.C.RabbitMQ.VirtualHost)

	// 后台监听关闭事件
	go watchClose()

	return nil
}

// 监听关闭事件，触发重连
func watchClose() {
	for {
		select {
		case err, ok := <-connClosedCh:
			if ok {
				log.Printf("[RabbitMQ] ⚠️ 连接关闭: %v", err)
				reconnect()
				return
			}
		case err, ok := <-chClosedCh:
			if ok {
				log.Printf("[RabbitMQ] ⚠️ 通道关闭: %v", err)
				reconnect()
				return
			}
		}
	}
}

// 自愈重连（阻塞重试直至成功）
func reconnect() {
	mu.Lock()
	if reconnecting {
		mu.Unlock()
		return
	}
	reconnecting = true
	mu.Unlock()

	defer func() {
		mu.Lock()
		reconnecting = false
		mu.Unlock()
	}()

	for {
		log.Println("[RabbitMQ] 🔄 正在重连...")
		if err := connect(); err == nil {
			log.Println("[RabbitMQ] ✅ 重连成功")
			return
		}
		time.Sleep(5 * time.Second)
	}
}

// -------- 状态判断（不用 IsClosed） --------

func isConnAlive() bool {
	if mqConn == nil || connClosedCh == nil {
		return false
	}
	select {
	case <-connClosedCh: // 一旦能读到，说明已关闭
		return false
	default:
		return true
	}
}

func isChanAlive() bool {
	if mqChannel == nil || chClosedCh == nil {
		return false
	}
	select {
	case <-chClosedCh:
		return false
	default:
		return true
	}
}

// -------- 对外获取 --------

func GetConnection() *amqp.Connection {
	// 若已断开，尝试重连
	if !isConnAlive() {
		reconnect()
	}
	return mqConn
}

func GetChannel() *amqp.Channel {
	if !isChanAlive() {
		reconnect()
	}
	return mqChannel
}
