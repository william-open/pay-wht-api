package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

type TelegramMessage struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
	Parse  string `json:"parse_mode,omitempty"`
}

type TelegramResponse struct {
	Ok          bool   `json:"ok"`
	Description string `json:"description,omitempty"`
}

func init() {
	_ = godotenv.Load() // 自动加载 .env 文件
}

// ================= 基础发送函数 =================

// SendTelegramMessage 同步发送
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

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("json marshal error: %w", err)
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("http post error: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response error: %w", err)
	}

	var tgResp TelegramResponse
	if err := json.Unmarshal(respBody, &tgResp); err != nil {
		return fmt.Errorf("unmarshal response error: %w", err)
	}

	if !tgResp.Ok {
		return fmt.Errorf("telegram send failed: %s", tgResp.Description)
	}

	return nil
}

// AsyncNotify 异步发送
func AsyncNotify(chatID string, content string) {
	go func() {
		if err := SendTelegramMessage(chatID, content); err != nil {
			log.Printf("[Telegram] 消息发送失败: %v", err)
		}
	}()
}

// ================= 消息模版 =================

func InfoMessage(title, content string) string {
	return fmt.Sprintf("ℹ️ *%s*\n\n%s", title, content)
}

func WarningMessage(title, content string) string {
	return fmt.Sprintf("⚠️ *%s*\n\n```%s```", title, content)
}

func ErrorMessage(title, content string) string {
	return fmt.Sprintf("❌ *%s*\n\n_%s_", title, content)
}

// ================= 统一入口 =================

// Notify 根据 level 自动选择模板
// level: info | warn | error
func Notify(chatID string, level string, title string, content string, async bool) {
	level = strings.ToLower(level)

	var msg string
	switch level {
	case "info":
		msg = InfoMessage(title, content)
	case "warn", "warning":
		msg = WarningMessage(title, content)
	case "error":
		msg = ErrorMessage(title, content)
	default:
		msg = fmt.Sprintf("*%s*\n\n%s", title, content)
	}

	if async {
		AsyncNotify(chatID, msg)
	} else {
		if err := SendTelegramMessage(chatID, msg); err != nil {
			log.Printf("[Telegram] 消息发送失败: %v", err)
		}
	}
}
