package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"wht-order-api/internal/config"
)

func AuthHMAC() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.Next()
			return
		}
		sig := c.GetHeader("X-Signature")
		if sig == "" {
			c.JSON(401, gin.H{"code": 401, "msg": "missing signature"})
			c.Abort()
			return
		}

		body, _ := io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		mac := hmac.New(sha256.New, []byte(config.C.Security.HMACSecret))
		mac.Write(body)
		if hex.EncodeToString(mac.Sum(nil)) != sig {
			c.JSON(401, gin.H{"code": 401, "msg": "bad signature"})
			c.Abort()
			return
		}
		c.Next()
	}
}
