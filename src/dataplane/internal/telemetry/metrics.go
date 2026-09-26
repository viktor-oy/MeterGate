package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	PolicyEvalDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "metergate_policy_eval_duration_seconds",
		Help:    "Time spent evaluating the rate limit policy",
		Buckets: prometheus.DefBuckets,
	}, []string{"route_id", "plan"})

	ProxyUpstreamDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "metergate_proxy_upstream_duration_seconds",
		Help:    "Time spent waiting for the upstream API to respond",
		Buckets: prometheus.DefBuckets,
	}, []string{"route_id", "plan"})

	RedisOperationDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "metergate_redis_operation_duration_seconds",
		Help:    "Time spent on Redis operations",
		Buckets: prometheus.DefBuckets,
	}, []string{"operation"})

	PolicyDecisions = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "metergate_policy_decisions_total",
		Help: "Total number of policy decisions",
	}, []string{"decision", "reason", "route_id", "plan"})

	CacheOperations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "metergate_cache_operations_total",
		Help: "Total number of cache operations",
	}, []string{"cache_level", "result"})
)
