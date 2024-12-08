## Observability Example

This example demonstrates how to integrate Docket with your observability stack for metrics, logging, and tracing.

## What This Demonstrates

- **Prometheus metrics** - Track execution duration, cache hits/misses, retries, errors
- **Structured logging** - JSON logs with execution context using Go's `log/slog`
- **Multiple observers** - Combine different observability backends
- **Cache monitoring** - Observe cache performance
- **Retry tracking** - See when and why steps retry

## Running the Example

```bash
go run examples/observability/main.go
```

The example will:
1. Execute a slow step (200ms)
2. Execute again (fast, from cache)
3. Execute with a different input (may retry on random failure)
4. Expose Prometheus metrics on http://localhost:2112/metrics

## Output

### Structured Logs (JSON)

```json
{"time":"2024-12-08T...","level":"INFO","msg":"graph execution started","execution_id":"exec-1","output_type":"*lettercount.LetterCount"}
{"time":"2024-12-08T...","level":"DEBUG","msg":"step execution started","execution_id":"exec-1","step_name":"SlowCount","step_type":"*lettercount.LetterCount","attempt":1}
{"time":"2024-12-08T...","level":"DEBUG","msg":"cache check","execution_id":"exec-1","step_name":"SlowCount","step_type":"*lettercount.LetterCount","hit":false,"latency":"50µs"}
{"time":"2024-12-08T...","level":"DEBUG","msg":"step execution completed","execution_id":"exec-1","step_name":"SlowCount","step_type":"*lettercount.LetterCount","duration":"205ms","cached":false}
{"time":"2024-12-08T...","level":"INFO","msg":"graph execution completed","execution_id":"exec-1","output_type":"*lettercount.LetterCount","duration":"206ms"}
```

### Prometheus Metrics

Visit http://localhost:2112/metrics to see:

```prometheus
# Graph execution duration
example_docket_graph_duration_seconds_bucket{execution_id="exec-1",output_type="*lettercount.LetterCount",status="success",le="0.25"} 1
example_docket_graph_duration_seconds_sum{execution_id="exec-1",output_type="*lettercount.LetterCount",status="success"} 0.206

# Step execution duration
example_docket_step_duration_seconds_bucket{step_name="SlowCount",step_type="*lettercount.LetterCount",status="success",le="0.25"} 1
example_docket_step_duration_seconds_sum{step_name="SlowCount",step_type="*lettercount.LetterCount",status="success"} 0.205

# Cache hits/misses
example_docket_cache_hits_total{step_name="SlowCount",step_type="*lettercount.LetterCount"} 1
example_docket_cache_misses_total{step_name="SlowCount",step_type="*lettercount.LetterCount"} 1

# Cache check latency
example_docket_cache_check_latency_seconds_sum{step_name="SlowCount",step_type="*lettercount.LetterCount"} 0.000102

# Retries and errors
example_docket_step_retries_total{step_name="SlowCount"} 1
example_docket_step_errors_total{step_name="SlowCount"} 0
```

## Available Observability Backends

### 1. Prometheus Metrics

**Best for:** Production monitoring, alerting, dashboards (Grafana)

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

registry := prometheus.NewRegistry()
observer := docket.NewPrometheusObserver("myapp", registry)

graph := docket.NewGraph(docket.WithObserver(observer))

// Expose metrics endpoint
http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
http.ListenAndServe(":2112", nil)
```

**Metrics exported:**
- `{namespace}_docket_graph_duration_seconds` - Graph execution time (histogram)
- `{namespace}_docket_step_duration_seconds` - Step execution time (histogram)
- `{namespace}_docket_cache_hits_total` - Cache hit count (counter)
- `{namespace}_docket_cache_misses_total` - Cache miss count (counter)
- `{namespace}_docket_cache_check_latency_seconds` - Cache lookup time (histogram)
- `{namespace}_docket_step_retries_total` - Retry count (counter)
- `{namespace}_docket_step_errors_total` - Error count (counter)

### 2. OpenTelemetry (OTel)

**Best for:** Distributed tracing, vendor-neutral observability, multi-backend support

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
    "go.opentelemetry.io/otel/sdk/trace"
)

// Setup OTel tracer
exporter, _ := otlptracehttp.New(ctx)
tp := trace.NewTracerProvider(trace.WithBatcher(exporter))
otel.SetTracerProvider(tp)

tracer := otel.Tracer("docket")
meter := otel.Meter("docket")

observer, _ := docket.NewOTelObserver(tracer, meter)
graph := docket.NewGraph(docket.WithObserver(observer))
```

**Integrates with:**
- Jaeger (distributed tracing)
- Tempo (Grafana tracing)
- Datadog APM
- New Relic
- Honeycomb
- Any OTLP-compatible backend

### 3. Structured Logging (slog)

**Best for:** Debugging, audit logs, development

```go
import "log/slog"

logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))

observer := docket.NewSlogObserver(logger, slog.LevelInfo)
graph := docket.NewGraph(docket.WithObserver(observer))
```

**Log levels:**
- `LevelDebug` - All events (step start/end, cache checks)
- `LevelInfo` - Graph executions only
- `LevelWarn` - Retries and failures
- `LevelError` - Graph failures

### 4. Custom Observer

Implement your own observer for any backend:

```go
type MyCustomObserver struct{}

func (o *MyCustomObserver) OnGraphStart(ctx context.Context, event *docket.GraphStartEvent) {
    // Send to Datadog, New Relic, custom metrics system, etc.
}

func (o *MyCustomObserver) OnGraphEnd(ctx context.Context, event *docket.GraphEndEvent) {
    // Track execution time, success rate
}

func (o *MyCustomObserver) OnStepStart(ctx context.Context, event *docket.StepStartEvent) {
    // Track individual step executions
}

func (o *MyCustomObserver) OnStepEnd(ctx context.Context, event *docket.StepEndEvent) {
    // Record step duration, errors
}

func (o *MyCustomObserver) OnCacheCheck(ctx context.Context, event *docket.CacheCheckEvent) {
    // Monitor cache hit rate
}

func (o *MyCustomObserver) OnRetry(ctx context.Context, event *docket.RetryEvent) {
    // Alert on excessive retries
}
```

### 5. Multiple Observers

Combine multiple backends:

```go
multiObserver := &docket.MultiObserver{
    Observers: []docket.Observer{
        prometheusObserver,  // Metrics
        slogObserver,        // Logs
        otelObserver,        // Traces
        customObserver,      // Your custom backend
    },
}

graph := docket.NewGraph(docket.WithObserver(multiObserver))
```

## Common Use Cases

### Production Monitoring Setup

```go
// 1. Prometheus for metrics and alerting
promObserver := docket.NewPrometheusObserver("myapp", prometheus.DefaultRegisterer)

// 2. Structured logs for audit trail
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
slogObserver := docket.NewSlogObserver(logger, slog.LevelWarn) // Only log issues

// 3. OpenTelemetry for distributed tracing
tracer := otel.Tracer("myapp")
meter := otel.Meter("myapp")
otelObserver, _ := docket.NewOTelObserver(tracer, meter)

// Combine all three
observer := &docket.MultiObserver{
    Observers: []docket.Observer{promObserver, slogObserver, otelObserver},
}

graph := docket.NewGraph(docket.WithObserver(observer))
```

### Development/Debugging Setup

```go
// Verbose logging for debugging
logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug, // See everything
}))

observer := docket.NewSlogObserver(logger, slog.LevelDebug)
graph := docket.NewGraph(docket.WithObserver(observer))
```

### Alerting on Failures

```go
type AlertingObserver struct {
    alertManager AlertManager
}

func (o *AlertingObserver) OnStepEnd(ctx context.Context, event *docket.StepEndEvent) {
    if event.Error != nil && event.Attempt >= 3 {
        o.alertManager.SendAlert("Step %s failed after %d attempts", event.StepName, event.Attempt)
    }
}

func (o *AlertingObserver) OnGraphEnd(ctx context.Context, event *docket.GraphEndEvent) {
    if event.Error != nil {
        o.alertManager.SendAlert("Graph execution failed: %v", event.Error)
    }
}
```

## Performance Considerations

1. **Observers are called synchronously** - Keep observer methods fast
2. **For expensive operations** - Buffer events and process asynchronously
3. **Metrics collection is cheap** - Prometheus/OTel metrics have minimal overhead
4. **Logging can be expensive** - Use appropriate log levels in production
5. **Disable in tests** - Use `NoOpObserver` or no observer for unit tests

## Best Practices

1. **Use appropriate log levels**
   - `DEBUG` - Development only
   - `INFO` - Important business events
   - `WARN` - Retries, recoverable errors
   - `ERROR` - Failures

2. **Monitor cache hit rate**
   ```promql
   rate(docket_cache_hits_total[5m]) /
   (rate(docket_cache_hits_total[5m]) + rate(docket_cache_misses_total[5m]))
   ```

3. **Alert on high error rates**
   ```promql
   rate(docket_step_errors_total[5m]) > 0.1
   ```

4. **Track P95/P99 latencies**
   ```promql
   histogram_quantile(0.95, rate(docket_step_duration_seconds_bucket[5m]))
   ```

5. **Monitor retry rates**
   ```promql
   rate(docket_step_retries_total[5m])
   ```

## Related Examples

- `examples/crash_recovery` - Recovery patterns
- `examples/error_handling` - Error handling strategies
- `examples/scope_comparison` - Cache behavior

## Further Reading

- [Prometheus Best Practices](https://prometheus.io/docs/practices/naming/)
- [OpenTelemetry Getting Started](https://opentelemetry.io/docs/languages/go/getting-started/)
- [Structured Logging in Go](https://go.dev/blog/slog)
