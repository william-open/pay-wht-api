package utils

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HttpPostJsonWithContext 发送带上下文的HTTP POST JSON请求
func HttpPostJsonWithContext(ctx context.Context, url string, data interface{}) (string, error) {
	// 创建HTTP客户端（带超时设置）
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // 跳过证书验证
			MaxIdleConns:    100,
			MaxConnsPerHost: 100,
			IdleConnTimeout: 90 * time.Second,
		},
	}

	// 将数据转换为JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("JSON编码失败: %w", err)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "WHT-Order-API/1.0")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP错误: %s", resp.Status)
	}

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	return string(body), nil
}

// CheckUpstreamHealth 检查上游服务健康状态
func CheckUpstreamHealth(ctx context.Context, upstreamUrl string) error {
	// 解析URL
	parsedUrl, err := url.Parse(upstreamUrl)
	if err != nil {
		return fmt.Errorf("URL解析失败: %w", err)
	}

	// 创建健康检查URL（使用HEAD方法）
	healthUrl := fmt.Sprintf("%s://%s", parsedUrl.Scheme, parsedUrl.Host)

	// 创建带超时的客户端
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// 创建HEAD请求
	req, err := http.NewRequestWithContext(ctx, "HEAD", healthUrl, nil)
	if err != nil {
		return fmt.Errorf("创建健康检查请求失败: %w", err)
	}

	// 发送健康检查请求
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("健康检查失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode >= 400 {
		return fmt.Errorf("上游服务异常: %s", resp.Status)
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
