package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HttpPostJson 发送 POST JSON 请求
func HttpPostJson(url string, data interface{}) (string, error) {
	// 将参数序列化为 JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("marshal json error: %v", err)
	}

	// 创建 HTTP 客户端（超时 10s）
	client := &http.Client{Timeout: 10 * time.Second}

	// 构建请求
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("new request error: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request error: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response error: %v", err)
	}

	// 如果状态码不是 200，返回错误
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status code: %d, body: %s", resp.StatusCode, string(body))
	}

	return string(body), nil
}
