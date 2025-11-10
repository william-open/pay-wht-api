package notify

import (
	"encoding/json"
	"fmt"
	"strings"
	"wht-order-api/internal/system"
	"wht-order-api/internal/utils/timeutil"
)

// NotifyUpstreamAlert 上游异常报警（层级化展示 + 下游/上游参数分层）
func NotifyUpstreamAlert(
	level, title, url string,
	downstreamReq interface{}, // 下游请求（商户 → 系统）
	upstreamReq interface{}, // 上游请求（系统 → 上游）
	upstreamResp interface{}, // 上游响应
	extra map[string]string, // 附加信息（Code、Msg 等）
) {
	// JSON 序列化
	downJSON, _ := json.Marshal(downstreamReq)
	upReqJSON, _ := json.Marshal(upstreamReq)
	upRespJSON, _ := json.Marshal(upstreamResp)

	// 尝试提取下游请求 map，用于自动抓取交易信息
	var reqMap map[string]interface{}
	_ = json.Unmarshal(downJSON, &reqMap)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🚨 *%s*\n", escapeMarkdown(title)))
	sb.WriteString(fmt.Sprintf("📡 *服务接口:* %s\n", escapeMarkdown(url)))
	sb.WriteString(fmt.Sprintf("🕒 *请求时间:* %s\n\n", timeutil.NowShanghai().Format("2006-01-02 15:04:05")))

	// ========== 一、基础交易信息 ==========
	sb.WriteString("*🧾 基础交易信息*\n")
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
	writeIf("上游供应商", "upstreamTitle")
	writeIf("上游商户号", "mchNo")
	writeIf("上游产品", "payType")
	writeIf("交易货币", "currency")
	writeIf("交易金额", "amount")
	writeIf("支付方式", "payMethod")
	writeIf("交易单号", "mchOrderId")
	writeIf("商户单号", "downstreamOrderNo")

	// ========== 二、额外信息 ==========
	if len(extra) > 0 {
		sb.WriteString("\n*🧩 额外信息*\n")
		for k, v := range extra {
			if v != "" {
				sb.WriteString(fmt.Sprintf("%s: %s\n", escapeMarkdown(k), escapeMarkdown(v)))
			}
		}
	}

	// ========== 三、下游请求参数（Downstream Request） ==========
	sDown := strings.TrimSpace(string(downJSON))
	if sDown != "" && sDown != "{}" {
		sb.WriteString("\n*📨 下游请求参数 (Downstream → System)*\n")
		sb.WriteString(fmt.Sprintf("`%s`\n", escapeMarkdown(sDown)))
	}

	// ========== 四、上游请求参数（System → Provider） ==========
	sUpReq := strings.TrimSpace(string(upReqJSON))
	if sUpReq != "" && sUpReq != "{}" {
		sb.WriteString("\n*⚙️ 上游请求参数 (System → Upstream)*\n")
		sb.WriteString(fmt.Sprintf("`%s`\n", escapeMarkdown(sUpReq)))
	}

	// ========== 五、上游返回结果（Provider Response） ==========
	sUpResp := strings.TrimSpace(string(upRespJSON))
	if sUpResp != "" && sUpResp != "{}" {
		sb.WriteString("\n*📬 上游返回结果 (Upstream → System)*\n")
		sb.WriteString(fmt.Sprintf("`%s`\n", escapeMarkdown(sUpResp)))
	}

	// 统一发送
	Notify(system.BotChatID, level, title, sb.String(), true)
}

// escapeMarkdown 转义 Telegram MarkdownV2 特殊字符
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
