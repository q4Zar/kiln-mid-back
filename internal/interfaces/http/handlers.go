package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/internal/domain"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/pkg/logger"
)

type Handler struct {
	service domain.DelegationService
	logger  *logger.Logger
}

func NewHandler(service domain.DelegationService, logger *logger.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

func (h *Handler) GetDelegations(c *gin.Context) {
	yearStr := c.Query("year")
	var yearPtr *int

	if yearStr != "" {
		year, err := strconv.Atoi(yearStr)
		if err != nil {
			h.logger.Errorw("Invalid year parameter", "year", yearStr, "error", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid year parameter. Must be a valid YYYY format",
			})
			return
		}

		if year < 2018 || year > 2100 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Year must be between 2018 and 2100",
			})
			return
		}

		yearPtr = &year
	}

	delegations, err := h.service.GetDelegations(yearPtr)
	if err != nil {
		h.logger.Errorw("Failed to get delegations", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve delegations",
		})
		return
	}

	response := domain.DelegationResponse{
		Data: delegations,
	}

	if response.Data == nil {
		response.Data = []domain.Delegation{}
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) GetHealth(c *gin.Context) {
	delegations, err := h.service.GetDelegations(nil)
	if err != nil {
		h.logger.Errorw("Health check failed", "error", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":            "healthy",
		"total_delegations": len(delegations),
	})
}

func (h *Handler) GetReadiness(c *gin.Context) {
	_, err := h.service.GetDelegations(nil)
	if err != nil {
		h.logger.Errorw("Readiness check failed", "error", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"error":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
	})
}

func (h *Handler) GetStats(c *gin.Context) {
	type StatsProvider interface {
		GetStats() (map[string]interface{}, error)
	}

	provider, ok := h.service.(StatsProvider)
	if !ok {
		c.JSON(http.StatusNotImplemented, gin.H{
			"error": "Stats not available",
		})
		return
	}

	stats, err := provider.GetStats()
	if err != nil {
		h.logger.Errorw("Failed to get stats", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve statistics",
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}
