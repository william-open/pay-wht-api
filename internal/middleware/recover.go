package middleware

import (
	"bytes"
	"log"
	"runtime/debug"
	"time"
	"wht-order-api/internal/dto"

	"github.com/gin-gonic/gin"
)

func Recover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic recovered: %v\n", r)
				debug.PrintStack() // 打印完整堆栈，包含文件名和行号
				ctxVal, _ := c.Get("audit_ctx")
				auditCtx := ctxVal.(*dto.AuditContextPayload)
				c.JSON(500, gin.H{"code": 500, "msg": "system internal error", "trace_id": auditCtx.TraceID})
				c.Abort()
			}
		}()
		c.Next()
	}
}

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		buf := new(bytes.Buffer)
		if c.Request.Body != nil {
			buf.ReadFrom(c.Request.Body)
			c.Request.Body = ioNopCloser{bytes.NewReader(buf.Bytes())}
		}
		c.Next()
		_ = start // 可接入日志系统
	}
}

type ioNopCloser struct{ *bytes.Reader }

func (ioNopCloser) Close() error { return nil }
