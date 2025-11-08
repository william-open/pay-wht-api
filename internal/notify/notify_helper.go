package notify

import (
	"encoding/json"
	"fmt"
	"strings"
	"wht-order-api/internal/system"
	"wht-order-api/internal/utils/timeutil"
)

// NotifyUpstreamAlert 上游异常报警（自动提取基础交易信息 + 简洁 JSON 展示）
func NotifyUpstreamAlert(
	level, title, url string,
	req interface{},
	resp interface{},
	extra map[string]string,
) {
	// 先序列化请求与响应
	reqJSON, _ := json.Marshal(req)
	respJSON, _ := json.Marshal(resp)

	// 尝试解析 req 为 map
	var reqMap map[string]interface{}
	_ = json.Unmarshal(reqJSON, &reqMap)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("*%s*\n", escapeMarkdown(title)))
	sb.WriteString(fmt.Sprintf("*服务接口:* %s\n", escapeMarkdown(url)))
	sb.WriteString(fmt.Sprintf("*请求时间:* %s\n", timeutil.NowShanghai().Format("2006-01-02 15:04:05")))

	// ========== 自动提取基础交易信息 ==========
	writeIf := func(label string, keys ...string) {
		for _, k := range keys {
			if v, ok := reqMap[k]; ok {
				val := fmt.Sprintf("%v", v)
				if val != "" && val != "<nil>" {
					sb.WriteString(fmt.Sprintf("%s: %s\n", escapeMarkdown(label), escapeMarkdown(val)))
					break
				}
			}
		}
	}

	writeIf("接口编码", "providerKey")
	writeIf("上游商户号", "mchNo")
	writeIf("上游产品", "payType")
	writeIf("交易货币", "currency")
	writeIf("交易金额", "amount")
	writeIf("交易单号", "mchOrderId")
	writeIf("支付方式", "payMethod")

	// ========== 额外信息（例如上游Code、Msg） ==========
	if len(extra) > 0 {
		for k, v := range extra {
			if v != "" {
				sb.WriteString(fmt.Sprintf("%s: %s\n", escapeMarkdown(k), escapeMarkdown(v)))
			}
		}
	}

	// ========== 请求参数（单行 JSON） ==========
	sReq := strings.TrimSpace(string(reqJSON))
	if sReq != "" && sReq != "{}" {
		sb.WriteString("\n*请求参数:*\n")
		sb.WriteString(fmt.Sprintf("`%s`\n", escapeMarkdown(sReq)))
	}

	// ========== 上游返回（单行 JSON） ==========
	sResp := strings.TrimSpace(string(respJSON))
	if sResp != "" && sResp != "{}" {
		sb.WriteString("\n*上游返回:*\n")
		sb.WriteString(fmt.Sprintf("`%s`\n", escapeMarkdown(sResp)))
	}

	Notify(system.BotChatID, level, title, sb.String(), true)
}

// escapeMarkdown 转义 Telegram Markdown V2 特殊字符
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
