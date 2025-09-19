package event

import (
	"encoding/json"
	"log"
	"wht-order-api/internal/mq"
)

// publish 发布任意结构体到指定 producer
func publish(producerName string, payload any) error {
	if err := mq.Publish(producerName, payload); err != nil {
		body, _ := json.Marshal(payload)
		log.Printf("❌ [EVENT] 发布失败 [%s]: %v\n内容: %s", producerName, err, string(body))
		return err
	}
	return nil
}
