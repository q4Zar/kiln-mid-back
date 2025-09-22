package http

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/internal/domain"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/pkg/logger"
)

func NewRouter(service domain.DelegationService, logger *logger.Logger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	router.Use(
		RecoveryMiddleware(logger),
		LoggingMiddleware(logger),
		CORSMiddleware(),
		RateLimitMiddleware(),
	)

	handler := NewHandler(service, logger)

	router.GET("/health", handler.GetHealth)
	router.GET("/ready", handler.GetReadiness)

	api := router.Group("/xtz")
	{
		api.GET("/delegations", handler.GetDelegations)
	}

	router.GET("/stats", handler.GetStats)

	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	return router
}
