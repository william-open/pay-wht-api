package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// TelegramMessage Telegram API 发送体
type TelegramMessage struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
	Parse  string `json:"parse_mode,omitempty"`
}

// TelegramResponse 响应体
type TelegramResponse struct {
	Ok          bool   `json:"ok"`
	Description string `json:"description,omitempty"`
}

// ================= 全局初始化 =================

var (
	httpClient *http.Client
	botToken   string
)

func init() {
	_ = godotenv.Load()

	botToken = os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Println("[Telegram] ⚠️ TELEGRAM_BOT_TOKEN 未设置，消息无法发送。")
	}

	// 构建带超时与代理支持的 http.Client
	httpClient = buildHTTPClient()
}

// ================= HTTP 客户端构造 =================

func buildHTTPClient() *http.Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second, // 建立连接超时
			KeepAlive: 60 * time.Second,
		}).DialContext,
		MaxIdleConns:          20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   8 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// 支持手动代理配置
	if proxy := os.Getenv("HTTP_PROXY"); proxy != "" {
		if proxyURL, err := url.Parse(proxy); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
			log.Printf("[Telegram] 🌐 使用代理: %s\n", proxy)
		}
	}

	return &http.Client{
		Timeout:   10 * time.Second, // 整体请求超时
		Transport: transport,
	}
}

// EscapeMarkdownV2 安全转义所有 Telegram MarkdownV2 特殊字符
func EscapeMarkdownV2(text string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"~", "\\~",
		"`", "\\`",
		">", "\\>",
		"#", "\\#",
		"+", "\\+",
		"-", "\\-",
		"=", "\\=",
		"|", "\\|",
		"{", "\\{",
		"}", "\\}",
		".", "\\.",
		"!", "\\!",
		"<", "\\<", // ✅ 新增：< 也需转义（HTML 符号冲突）
		"&", "\\&", // ✅ 新增：防止 HTML 解析干扰
	)
	// 二次安全: 把多个反斜杠压缩为单一符号（避免二次拼接出现 "\\\\"）
	safe := replacer.Replace(text)
	safe = strings.ReplaceAll(safe, "\\\\", "\\")
	return safe
}

// ================= 基础发送函数 =================

// SendTelegramMessage 同步发送（带重试机制）
func SendTelegramMessage(chatID, content string) error {
	if botToken == "" {
		return fmt.Errorf("missing TELEGRAM_BOT_TOKEN in env")
	}

	msg := TelegramMessage{
		ChatID: chatID,
		Text:   EscapeMarkdownV2(content),
		Parse:  "MarkdownV2",
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("json marshal error: %w", err)
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	var lastErr error
	const maxRetries = 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		resp, err := httpClient.Post(url, "application/json", bytes.NewBuffer(body))
		if err != nil {
			lastErr = fmt.Errorf("http post error: %w", err)
			log.Printf("[Telegram][%v] 第 %d/%d 次发送失败: %v", chatID, attempt, maxRetries, err)

			sleep := time.Duration(attempt*2) * time.Second // 指数退避
			time.Sleep(sleep)
			continue
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("telegram http status %d: %s", resp.StatusCode, string(respBody))
			log.Printf("[Telegram][%v] 第 %d/%d 次返回错误: %v", chatID, attempt, maxRetries, lastErr)
			time.Sleep(time.Duration(attempt*2) * time.Second)
			continue
		}

		var tgResp TelegramResponse
		if err := json.Unmarshal(respBody, &tgResp); err != nil {
			lastErr = fmt.Errorf("unmarshal response error: %w", err)
			continue
		}

		if !tgResp.Ok {
			lastErr = fmt.Errorf("telegram send failed: %s", tgResp.Description)
			log.Printf("[Telegram][%v] 第 %d/%d 次失败: %s", chatID, attempt, maxRetries, tgResp.Description)
			time.Sleep(time.Duration(attempt*2) * time.Second)
			continue
		}

		// ✅ 成功
		return nil
	}

	return fmt.Errorf("telegram send failed after %d retries: %w", maxRetries, lastErr)
}

// AsyncNotify 异步发送
func AsyncNotify(chatID string, content string) {
	go func() {
		if err := SendTelegramMessage(chatID, content); err != nil {
			log.Printf("[Telegram][%v] 消息发送失败: %v", chatID, err)
		}
	}()
}

// ================= 消息模板 =================

func InfoMessage(title, content string) string {
	return fmt.Sprintf("ℹ️ *%s*\n\n%s", title, content)
}

func WarningMessage(title, content string) string {
	return fmt.Sprintf("⚠️ *%s*\n\n```\n%s\n```", title, content)
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
			log.Printf("[Telegram][%v] 消息发送失败: %v", chatID, err)
		}
	}
}
