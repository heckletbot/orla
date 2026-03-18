// Package metrics provides Prometheus metrics for the Orla serving layer.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const namespace = "orla"

var (
	// RequestsTotal counts inference requests by backend and status.
	RequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "requests_total",
			Help:      "Total number of inference requests by backend and status",
		},
		[]string{"backend", "status"}, // status: success, error
	)

	// QueueWaitSeconds is the time requests spend waiting in the scheduler queue.
	QueueWaitSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "queue_wait_seconds",
			Help:      "Time spent waiting in the scheduler queue before dispatch",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 14), // 1ms to ~8s
		},
		[]string{"backend"},
	)

	// BackendLatencySeconds is the time spent in the LLM backend.
	BackendLatencySeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "backend_latency_seconds",
			Help:      "Time spent in the LLM backend (inference)",
			Buckets:   prometheus.ExponentialBuckets(0.1, 2, 12), // 100ms to ~3min
		},
		[]string{"backend"},
	)

	// QueueDepth is the current number of requests waiting per backend.
	QueueDepth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "queue_depth",
			Help:      "Number of requests currently waiting in the scheduler queue per backend",
		},
		[]string{"backend"},
	)
)

func init() {
	prometheus.MustRegister(RequestsTotal, QueueWaitSeconds, BackendLatencySeconds, QueueDepth)
}
