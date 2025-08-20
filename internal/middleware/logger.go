package middleware

import (
	"github.com/gin-gonic/gin"
	"time"
	"wht-order-api/internal/logger"
)

func RequestLogger() gin.HandlerFunc {
	infoLog := logger.NewLogger("info")
	errorLog := logger.NewLogger("error")

	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)

		entry := map[string]interface{}{
			"status":     c.Writer.Status(),
			"method":     c.Request.Method,
			"path":       c.Request.URL.Path,
			"ip":         c.ClientIP(),
			"latency":    latency.String(),
			"user-agent": c.Request.UserAgent(),
		}

		if len(c.Errors) > 0 {
			errorLog.WithFields(entry).Error(c.Errors.String())
		} else {
			infoLog.WithFields(entry).Info("request completed")
		}
	}
}
