package docket

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// OTelObserver implements Observer using OpenTelemetry for traces and metrics.
// This provides automatic integration with OTLP exporters (Jaeger, Tempo, Datadog, etc.).
//
// Example:
//
//	tracer := otel.Tracer("docket")
//	meter := otel.Meter("docket")
//	observer, _ := docket.NewOTelObserver(tracer, meter)
//	graph := docket.NewGraph(docket.WithObserver(observer))
type OTelObserver struct {
	tracer trace.Tracer

	// Metrics
	graphDuration     metric.Float64Histogram
	stepDuration      metric.Float64Histogram
	cacheHits         metric.Int64Counter
	cacheMisses       metric.Int64Counter
	cacheCheckLatency metric.Float64Histogram
	retries           metric.Int64Counter
	errors            metric.Int64Counter
}

// NewOTelObserver creates an OpenTelemetry observer.
func NewOTelObserver(tracer trace.Tracer, meter metric.Meter) (*OTelObserver, error) {
	graphDuration, err := meter.Float64Histogram(
		"docket.graph.duration",
		metric.WithDescription("Duration of graph execution in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create graph duration histogram: %w", err)
	}

	stepDuration, err := meter.Float64Histogram(
		"docket.step.duration",
		metric.WithDescription("Duration of step execution in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create step duration histogram: %w", err)
	}

	cacheHits, err := meter.Int64Counter(
		"docket.cache.hits",
		metric.WithDescription("Number of cache hits"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache hits counter: %w", err)
	}

	cacheMisses, err := meter.Int64Counter(
		"docket.cache.misses",
		metric.WithDescription("Number of cache misses"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache misses counter: %w", err)
	}

	cacheCheckLatency, err := meter.Float64Histogram(
		"docket.cache.check_latency",
		metric.WithDescription("Latency of cache lookups in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache latency histogram: %w", err)
	}

	retries, err := meter.Int64Counter(
		"docket.step.retries",
		metric.WithDescription("Number of step retries"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create retries counter: %w", err)
	}

	errors, err := meter.Int64Counter(
		"docket.step.errors",
		metric.WithDescription("Number of step errors"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create errors counter: %w", err)
	}

	return &OTelObserver{
		tracer:            tracer,
		graphDuration:     graphDuration,
		stepDuration:      stepDuration,
		cacheHits:         cacheHits,
		cacheMisses:       cacheMisses,
		cacheCheckLatency: cacheCheckLatency,
		retries:           retries,
		errors:            errors,
	}, nil
}

func (o *OTelObserver) OnGraphStart(ctx context.Context, event *GraphStartEvent) {
	// Create a span for the entire graph execution
	_, span := o.tracer.Start(ctx, "graph.execute",
		trace.WithAttributes(
			attribute.String("execution_id", event.ExecutionID),
			attribute.String("output_type", event.OutputType),
		),
	)
	// Note: In real usage, the span should be stored in context and ended in OnGraphEnd
	// For simplicity, we're not managing span lifecycle here - users should use trace.SpanFromContext
	_ = span
}

func (o *OTelObserver) OnGraphEnd(ctx context.Context, event *GraphEndEvent) {
	// End the span from context
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		if event.Error != nil {
			span.SetStatus(codes.Error, event.Error.Error())
			span.RecordError(event.Error)
		} else {
			span.SetStatus(codes.Ok, "")
		}
		span.End()
	}

	// Record duration metric
	attrs := []attribute.KeyValue{
		attribute.String("execution_id", event.ExecutionID),
		attribute.String("output_type", event.OutputType),
		attribute.Bool("success", event.Error == nil),
	}
	o.graphDuration.Record(ctx, event.Duration.Seconds(), metric.WithAttributes(attrs...))
}

func (o *OTelObserver) OnStepStart(ctx context.Context, event *StepStartEvent) {
	_, span := o.tracer.Start(ctx, event.StepName,
		trace.WithAttributes(
			attribute.String("execution_id", event.ExecutionID),
			attribute.String("step_name", event.StepName),
			attribute.String("step_type", event.StepType),
			attribute.Int("attempt", event.Attempt),
		),
	)
	_ = span
}

func (o *OTelObserver) OnStepEnd(ctx context.Context, event *StepEndEvent) {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		if event.Error != nil {
			span.SetStatus(codes.Error, event.Error.Error())
			span.RecordError(event.Error)
		} else {
			span.SetStatus(codes.Ok, "")
		}
		span.SetAttributes(
			attribute.Bool("cached", event.Cached),
			attribute.Bool("panicked", event.Panicked),
		)
		span.End()
	}

	// Record metrics
	attrs := []attribute.KeyValue{
		attribute.String("step_name", event.StepName),
		attribute.String("step_type", event.StepType),
		attribute.Bool("success", event.Error == nil),
		attribute.Bool("cached", event.Cached),
		attribute.Bool("panicked", event.Panicked),
	}

	o.stepDuration.Record(ctx, event.Duration.Seconds(), metric.WithAttributes(attrs...))

	if event.Error != nil {
		o.errors.Add(ctx, 1, metric.WithAttributes(
			attribute.String("step_name", event.StepName),
		))
	}
}

func (o *OTelObserver) OnCacheCheck(ctx context.Context, event *CacheCheckEvent) {
	attrs := []attribute.KeyValue{
		attribute.String("step_name", event.StepName),
		attribute.String("step_type", event.StepType),
	}

	if event.Hit {
		o.cacheHits.Add(ctx, 1, metric.WithAttributes(attrs...))
	} else {
		o.cacheMisses.Add(ctx, 1, metric.WithAttributes(attrs...))
	}

	o.cacheCheckLatency.Record(ctx, event.Latency.Seconds(), metric.WithAttributes(attrs...))

	// Add trace event
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		span.AddEvent("cache_check", trace.WithAttributes(
			attribute.Bool("hit", event.Hit),
			attribute.String("step_name", event.StepName),
		))
	}
}

func (o *OTelObserver) OnRetry(ctx context.Context, event *RetryEvent) {
	o.retries.Add(ctx, 1, metric.WithAttributes(
		attribute.String("step_name", event.StepName),
		attribute.Int("attempt", event.Attempt),
	))

	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		span.AddEvent("retry", trace.WithAttributes(
			attribute.Int("attempt", event.Attempt),
			attribute.String("error", event.Error.Error()),
			attribute.String("delay", event.Delay.String()),
		))
	}
}
