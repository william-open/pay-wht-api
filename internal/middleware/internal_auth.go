package middleware

import "C"
import (
	"net/http"
	"strings"
	"wht-order-api/internal/config"

	"github.com/gin-gonic/gin"
)

func InternalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Token 验证
		token := c.GetHeader("X-Upstream-Token")
		upstreamAuthToken := config.C.Upstream.AuthToken
		if token != upstreamAuthToken {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code": 401,
				"msg":  "invalid upstream token",
			})
			c.Abort()
			return
		}

		// IP 白名单
		ip := c.ClientIP()
		whitelist := []string{"127.0.0.1", "192.168.", "10.", "::1"}
		allowed := false
		for _, prefix := range whitelist {
			if strings.HasPrefix(ip, prefix) {
				allowed = true
				break
			}
		}
		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{
				"code": 403,
				"msg":  "ip not allowed: " + ip,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
