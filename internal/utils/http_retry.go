package utils

import (
	"context"
	"fmt"
	"log"
	"time"
)

// DoWithRetry 执行带重试逻辑的函数
func DoWithRetry(ctx context.Context, maxRetries int, interval time.Duration, fn func() error) error {
	var err error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err = fn()
		if err == nil {
			return nil
		}

		// 若超时或临时网络错误，进行重试
		log.Printf("[RETRY] 第 %d/%d 次失败: %v", attempt, maxRetries, err)

		// 最后一次失败则直接返回
		if attempt == maxRetries {
			break
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("上下文已取消或超时: %w", ctx.Err())
		case <-time.After(interval):
			// 等待后重试
		}
	}
	return err
}
