package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// HttpPostJson 发送 POST JSON 请求
func HttpPostJson(url string, data interface{}) (string, error) {
	// 将参数序列化为 JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("marshal json error: %v", err)
	}

	log.Printf("请求上游URL: %v,请求上游参数: %v", url, string(jsonData))
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

func CheckUpstreamHealth(url string) error {
	log.Printf("请求检测地址: %v", url)
	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("状态码异常: %d", resp.StatusCode)
	}
	return nil
}

func GetClientIP(c *gin.Context) string {
	ip := c.GetHeader("X-Forwarded-For")
	if ip != "" {
		parts := strings.Split(ip, ",")
		return strings.TrimSpace(parts[0])
	}
	return c.ClientIP()
}
