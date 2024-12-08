package docket

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
)

// PrometheusObserver implements Observer using Prometheus metrics.
// This is useful if you're already using Prometheus for monitoring.
//
// Example:
//
//	observer := docket.NewPrometheusObserver("my_service", prometheus.DefaultRegisterer)
//	graph := docket.NewGraph(docket.WithObserver(observer))
type PrometheusObserver struct {
	graphDuration     *prometheus.HistogramVec
	stepDuration      *prometheus.HistogramVec
	cacheHits         *prometheus.CounterVec
	cacheMisses       *prometheus.CounterVec
	cacheCheckLatency *prometheus.HistogramVec
	retries           *prometheus.CounterVec
	errors            *prometheus.CounterVec
}

// NewPrometheusObserver creates a Prometheus observer with the given namespace.
// All metrics will be prefixed with "{namespace}_docket_".
//
// Example:
//
//	observer := NewPrometheusObserver("myapp", prometheus.DefaultRegisterer)
//	// Creates metrics like: myapp_docket_graph_duration_seconds
func NewPrometheusObserver(namespace string, registerer prometheus.Registerer) *PrometheusObserver {
	if namespace == "" {
		namespace = "docket"
	}

	graphDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "docket",
			Name:      "graph_duration_seconds",
			Help:      "Duration of graph execution in seconds",
			Buckets:   prometheus.DefBuckets, // [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10]
		},
		[]string{"execution_id", "output_type", "status"},
	)

	stepDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "docket",
			Name:      "step_duration_seconds",
			Help:      "Duration of step execution in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"step_name", "step_type", "status"},
	)

	cacheHits := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "docket",
			Name:      "cache_hits_total",
			Help:      "Total number of cache hits",
		},
		[]string{"step_name", "step_type"},
	)

	cacheMisses := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "docket",
			Name:      "cache_misses_total",
			Help:      "Total number of cache misses",
		},
		[]string{"step_name", "step_type"},
	)

	cacheCheckLatency := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "docket",
			Name:      "cache_check_latency_seconds",
			Help:      "Latency of cache lookups in seconds",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
		[]string{"step_name", "step_type"},
	)

	retries := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "docket",
			Name:      "step_retries_total",
			Help:      "Total number of step retries",
		},
		[]string{"step_name"},
	)

	errors := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "docket",
			Name:      "step_errors_total",
			Help:      "Total number of step errors",
		},
		[]string{"step_name"},
	)

	// Register all metrics
	registerer.MustRegister(
		graphDuration,
		stepDuration,
		cacheHits,
		cacheMisses,
		cacheCheckLatency,
		retries,
		errors,
	)

	return &PrometheusObserver{
		graphDuration:     graphDuration,
		stepDuration:      stepDuration,
		cacheHits:         cacheHits,
		cacheMisses:       cacheMisses,
		cacheCheckLatency: cacheCheckLatency,
		retries:           retries,
		errors:            errors,
	}
}

func (o *PrometheusObserver) OnGraphStart(ctx context.Context, event *GraphStartEvent) {
	// Nothing to do on start for Prometheus
}

func (o *PrometheusObserver) OnGraphEnd(ctx context.Context, event *GraphEndEvent) {
	status := "success"
	if event.Error != nil {
		status = "error"
	}

	o.graphDuration.WithLabelValues(
		event.ExecutionID,
		event.OutputType,
		status,
	).Observe(event.Duration.Seconds())
}

func (o *PrometheusObserver) OnStepStart(ctx context.Context, event *StepStartEvent) {
	// Nothing to do on start for Prometheus
}

func (o *PrometheusObserver) OnStepEnd(ctx context.Context, event *StepEndEvent) {
	status := "success"
	if event.Error != nil {
		status = "error"
	} else if event.Panicked {
		status = "panic"
	}

	o.stepDuration.WithLabelValues(
		event.StepName,
		event.StepType,
		status,
	).Observe(event.Duration.Seconds())

	if event.Error != nil {
		o.errors.WithLabelValues(event.StepName).Inc()
	}
}

func (o *PrometheusObserver) OnCacheCheck(ctx context.Context, event *CacheCheckEvent) {
	labels := prometheus.Labels{
		"step_name": event.StepName,
		"step_type": event.StepType,
	}

	if event.Hit {
		o.cacheHits.With(labels).Inc()
	} else {
		o.cacheMisses.With(labels).Inc()
	}

	o.cacheCheckLatency.With(labels).Observe(event.Latency.Seconds())
}

func (o *PrometheusObserver) OnRetry(ctx context.Context, event *RetryEvent) {
	o.retries.WithLabelValues(event.StepName).Inc()
}
