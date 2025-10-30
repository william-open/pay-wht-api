package notify

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"wht-order-api/internal/system"
)

// NotifyUpstreamAlert 统一上游异常报警（Markdown版本，无HTML标签）
func NotifyUpstreamAlert(level, title, url string, req interface{}, resp interface{}, extra map[string]string) {
	// JSON 格式化输出
	reqJSON, _ := json.MarshalIndent(req, "", "  ")
	respJSON, _ := json.MarshalIndent(resp, "", "  ")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🧩 *%s*\n", escapeMarkdown(title)))
	sb.WriteString(fmt.Sprintf("📡 *接口:* %s\n", escapeMarkdown(url)))
	sb.WriteString(fmt.Sprintf("🕒 *时间:* %s\n", time.Now().Format("2006-01-02 15:04:05")))

	// 附加额外上下文信息
	if len(extra) > 0 {
		for k, v := range extra {
			sb.WriteString(fmt.Sprintf("%s: %s\n", escapeMarkdown(k), escapeMarkdown(v)))
		}
	}

	// 请求体
	sb.WriteString("\n📤 *请求参数:*\n```json\n")
	sb.WriteString(string(reqJSON))
	sb.WriteString("\n```\n")

	// 响应体（如果存在）
	if resp != nil {
		if s := strings.TrimSpace(string(respJSON)); s != "{}" && s != `""` {
			sb.WriteString("📥 *响应数据:*\n```json\n")
			sb.WriteString(s)
			sb.WriteString("\n```\n")
		}
	}

	// 调用底层通知函数（Markdown 模式）
	Notify(system.BotChatID, level, title, sb.String(), true)
}

// escapeMarkdown 转义 Markdown 特殊字符，避免 JSON 内容触发解析错误
func escapeMarkdown(s string) string {
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
	)
	return replacer.Replace(s)
}
