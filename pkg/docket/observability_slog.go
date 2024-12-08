package docket

import (
	"context"
	"log/slog"
)

// SlogObserver implements Observer using Go's structured logging (log/slog).
// This emits structured logs for all execution events.
//
// Example:
//
//	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	observer := docket.NewSlogObserver(logger, slog.LevelInfo)
//	graph := docket.NewGraph(docket.WithObserver(observer))
type SlogObserver struct {
	logger   *slog.Logger
	minLevel slog.Level
}

// NewSlogObserver creates an observer that logs to the given slog.Logger.
// Only events at or above minLevel will be logged.
func NewSlogObserver(logger *slog.Logger, minLevel slog.Level) *SlogObserver {
	return &SlogObserver{
		logger:   logger,
		minLevel: minLevel,
	}
}

func (o *SlogObserver) OnGraphStart(ctx context.Context, event *GraphStartEvent) {
	if o.minLevel <= slog.LevelInfo {
		o.logger.InfoContext(ctx, "graph execution started",
			slog.String("execution_id", event.ExecutionID),
			slog.String("output_type", event.OutputType),
		)
	}
}

func (o *SlogObserver) OnGraphEnd(ctx context.Context, event *GraphEndEvent) {
	if event.Error != nil {
		if o.minLevel <= slog.LevelError {
			o.logger.ErrorContext(ctx, "graph execution failed",
				slog.String("execution_id", event.ExecutionID),
				slog.String("output_type", event.OutputType),
				slog.Duration("duration", event.Duration),
				slog.String("error", event.Error.Error()),
			)
		}
	} else {
		if o.minLevel <= slog.LevelInfo {
			o.logger.InfoContext(ctx, "graph execution completed",
				slog.String("execution_id", event.ExecutionID),
				slog.String("output_type", event.OutputType),
				slog.Duration("duration", event.Duration),
			)
		}
	}
}

func (o *SlogObserver) OnStepStart(ctx context.Context, event *StepStartEvent) {
	if o.minLevel <= slog.LevelDebug {
		o.logger.DebugContext(ctx, "step execution started",
			slog.String("execution_id", event.ExecutionID),
			slog.String("step_name", event.StepName),
			slog.String("step_type", event.StepType),
			slog.Int("attempt", event.Attempt),
		)
	}
}

func (o *SlogObserver) OnStepEnd(ctx context.Context, event *StepEndEvent) {
	if event.Error != nil {
		if o.minLevel <= slog.LevelWarn {
			o.logger.WarnContext(ctx, "step execution failed",
				slog.String("execution_id", event.ExecutionID),
				slog.String("step_name", event.StepName),
				slog.String("step_type", event.StepType),
				slog.Int("attempt", event.Attempt),
				slog.Duration("duration", event.Duration),
				slog.Bool("panicked", event.Panicked),
				slog.String("error", event.Error.Error()),
			)
		}
	} else {
		if o.minLevel <= slog.LevelDebug {
			o.logger.DebugContext(ctx, "step execution completed",
				slog.String("execution_id", event.ExecutionID),
				slog.String("step_name", event.StepName),
				slog.String("step_type", event.StepType),
				slog.Duration("duration", event.Duration),
				slog.Bool("cached", event.Cached),
			)
		}
	}
}

func (o *SlogObserver) OnCacheCheck(ctx context.Context, event *CacheCheckEvent) {
	if o.minLevel <= slog.LevelDebug {
		o.logger.DebugContext(ctx, "cache check",
			slog.String("execution_id", event.ExecutionID),
			slog.String("step_name", event.StepName),
			slog.String("step_type", event.StepType),
			slog.Bool("hit", event.Hit),
			slog.Duration("latency", event.Latency),
		)
	}
}

func (o *SlogObserver) OnRetry(ctx context.Context, event *RetryEvent) {
	if o.minLevel <= slog.LevelWarn {
		o.logger.WarnContext(ctx, "step retry",
			slog.String("execution_id", event.ExecutionID),
			slog.String("step_name", event.StepName),
			slog.Int("attempt", event.Attempt),
			slog.Duration("delay", event.Delay),
			slog.String("error", event.Error.Error()),
		)
	}
}
