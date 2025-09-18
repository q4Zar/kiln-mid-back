package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	DelegationsProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tezos_delegations_processed_total",
			Help: "The total number of delegations processed",
		},
		[]string{"status"},
	)

	DelegationsStored = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "tezos_delegations_stored_total",
			Help: "The total number of delegations stored in database",
		},
	)

	APIRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tezos_api_request_duration_seconds",
			Help:    "Duration of API requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"endpoint", "method", "status"},
	)

	TzktAPIRequestDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "tzkt_api_request_duration_seconds",
			Help:    "Duration of TzKT API requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)

	TzktAPIRequestErrors = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "tzkt_api_request_errors_total",
			Help: "The total number of TzKT API request errors",
		},
	)

	LastIndexedLevel = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "tezos_last_indexed_level",
			Help: "The last indexed block level",
		},
	)

	DatabaseConnections = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "database_connections",
			Help: "Number of database connections",
		},
		[]string{"state"},
	)

	PollingErrors = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "tezos_polling_errors_total",
			Help: "The total number of polling errors",
		},
	)

	HistoricalIndexingProgress = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "tezos_historical_indexing_progress",
			Help: "Progress of historical indexing (0-100)",
		},
	)
)

func RecordAPIRequest(endpoint, method string, status int, duration float64) {
	APIRequestDuration.WithLabelValues(endpoint, method, string(rune(status))).Observe(duration)
}

func RecordDelegationProcessed(status string) {
	DelegationsProcessed.WithLabelValues(status).Inc()
}

func UpdateLastIndexedLevel(level int64) {
	LastIndexedLevel.Set(float64(level))
}

func RecordTzktAPIRequest(duration float64, success bool) {
	TzktAPIRequestDuration.Observe(duration)
	if !success {
		TzktAPIRequestErrors.Inc()
	}
}

func UpdateDatabaseConnections(active, idle int) {
	DatabaseConnections.WithLabelValues("active").Set(float64(active))
	DatabaseConnections.WithLabelValues("idle").Set(float64(idle))
}
