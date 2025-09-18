package http

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/pkg/logger"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/pkg/metrics"
)

func LoggingMiddleware(logger *logger.Logger) gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		logger.Infow("Request processed",
			"method", param.Method,
			"path", param.Path,
			"status", param.StatusCode,
			"latency", param.Latency,
			"clientIP", param.ClientIP,
			"userAgent", param.Request.UserAgent(),
			"error", param.ErrorMessage,
		)
		// Record metrics
		metrics.APIRequestDuration.WithLabelValues(
			param.Path,
			param.Method,
			strconv.Itoa(param.StatusCode),
		).Observe(param.Latency.Seconds())
		return ""
	})
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func TimeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(c.Request.Context())
		c.Next()
	}
}

func RecoveryMiddleware(logger *logger.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		logger.Errorw("Panic recovered",
			"error", recovered,
			"path", c.Request.URL.Path,
			"method", c.Request.Method,
		)
		c.AbortWithStatusJSON(500, gin.H{
			"error": "Internal server error",
		})
	})
}

func RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
