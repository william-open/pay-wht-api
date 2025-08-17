package middleware

import (
	"bytes"
	"time"

	"github.com/gin-gonic/gin"
)

func Recover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				c.JSON(500, gin.H{"code": 500, "msg": "internal error"})
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
