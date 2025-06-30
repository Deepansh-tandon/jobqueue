package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for the application.
type Metrics struct {
	JobsProcessedTotal  *prometheus.CounterVec
	JobFailuresTotal    *prometheus.CounterVec
	JobsReapedTotal     *prometheus.CounterVec
	JobDurationSeconds  *prometheus.HistogramVec
	ActiveWorkers       *prometheus.GaugeVec
	QueueLength         *prometheus.GaugeVec
}

// NewMetrics creates and registers the Prometheus metrics.
func NewMetrics() *Metrics {
	m := &Metrics{
		JobsProcessedTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "jobqueue",
				Name:      "jobs_processed_total",
				Help:      "Total number of jobs processed, partitioned by status.",
			},
			[]string{"queue", "status"}, // status can be "completed", "failed"
		),
		JobFailuresTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "jobqueue",
				Name:      "job_failures_total",
				Help:      "Total number of job processing failures that led to a retry or DLQ.",
			},
			[]string{"queue", "type"},
		),
		JobsReapedTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "jobqueue",
				Name:      "jobs_reaped_total",
				Help:      "Total number of orphaned jobs re-queued by the reaper.",
			},
			[]string{"queue"},
		),
		JobDurationSeconds: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "jobqueue",
				Name:      "job_duration_seconds",
				Help:      "Histogram of job processing times.",
				Buckets:   prometheus.LinearBuckets(0.1, 0.1, 10), // 10 buckets, 0.1s each
			},
			[]string{"queue", "type"},
		),
		ActiveWorkers: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "jobqueue",
				Name:      "workers_active",
				Help:      "Number of currently active workers.",
			},
			[]string{"queue"},
		),
		QueueLength: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "jobqueue",
				Name:      "queue_length",
				Help:      "Number of jobs in a queue.",
			},
			[]string{"queue"},
		),
	}
	return m
}
