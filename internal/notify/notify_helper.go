package notify

import (
	"encoding/json"
	"fmt"
	"strings"
	"wht-order-api/internal/system"
	"wht-order-api/internal/utils/timeutil"
)

// NotifyUpstreamAlert 统一上游异常报警（Markdown版本，无HTML标签）
func NotifyUpstreamAlert(level, title, url string, req interface{}, resp interface{}, extra map[string]string) {
	reqJSON, _ := json.MarshalIndent(req, "", "  ")
	respJSON, _ := json.MarshalIndent(resp, "", "  ")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("*%s*\n", escapeMarkdown(title)))
	sb.WriteString(fmt.Sprintf("*接口:* %s\n", escapeMarkdown(url)))
	sb.WriteString(fmt.Sprintf("*时间:* %s\n", timeutil.NowShanghai().Format("2006-01-02 15:04:05")))

	if len(extra) > 0 {
		for k, v := range extra {
			sb.WriteString(fmt.Sprintf("%s: %s\n", escapeMarkdown(k), escapeMarkdown(v)))
		}
	}

	sb.WriteString("\n*请求参数:*\n```json\n")
	sb.WriteString(escapeMarkdown(string(reqJSON)))
	sb.WriteString("\n```\n")

	if resp != nil {
		s := strings.TrimSpace(string(respJSON))
		if s != "{}" && s != `""` {
			sb.WriteString("*响应数据:*\n```json\n")
			sb.WriteString(escapeMarkdown(s))
			sb.WriteString("\n```\n")
		}
	}

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
