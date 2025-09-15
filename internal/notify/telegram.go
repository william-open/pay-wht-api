package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

type TelegramMessage struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
	Parse  string `json:"parse_mode"`
}

func init() {
	_ = godotenv.Load() // 自动加载 .env 文件
}

func SendTelegramMessage(chatID string, content string) error {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		return fmt.Errorf("missing TELEGRAM_BOT_TOKEN in env")
	}

	msg := TelegramMessage{
		ChatID: chatID,
		Text:   content,
		Parse:  "Markdown",
	}
	body, _ := json.Marshal(msg)
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)
	return nil
}

// 异步发送错误信息
func NotifySendMsgToTG(chatId string, content string) {
	go func() {
		if err := SendTelegramMessage(chatId, content); err != nil {
			log.Printf("Telegram 消息发送失败: %v", err)
		}
	}()
}
