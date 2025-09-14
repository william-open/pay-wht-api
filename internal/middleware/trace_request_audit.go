package middleware

import (
	"bytes"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"io"
	"time"
	"wht-order-api/internal/dto"
	"wht-order-api/internal/logger"
)

func TraceAuditMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := uuid.New().String()
		bodyBytes, _ := io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		ctx := &dto.AuditContextPayload{
			TraceID:     traceID,
			RequestBody: string(bodyBytes),
			IP:          c.ClientIP(),
			UserAgent:   c.GetHeader("User-Agent"),
			StartTime:   time.Now(),
		}
		c.Set("audit_ctx", ctx)
		c.Writer.Header().Set("X-Trace-ID", traceID)

		c.Next()

		ctx.LatencyMs = time.Since(ctx.StartTime).Milliseconds()
		logger.WriteOrderLog(ctx)
	}
}
