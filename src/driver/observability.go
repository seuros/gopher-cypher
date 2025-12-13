package driver

import (
	"context"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	// Instrumentation library name
	instrumentationName    = "github.com/seuros/gopher-cypher/src/driver"
	instrumentationVersion = "0.2.0" // Will be replaced by build-time injection
)

// ObservabilityConfig controls telemetry collection
type ObservabilityConfig struct {
	// EnableTracing enables OpenTelemetry distributed tracing
	EnableTracing bool

	// EnableMetrics enables OpenTelemetry metrics collection
	EnableMetrics bool

	// TracingAttributes are additional attributes to add to all spans
	TracingAttributes []attribute.KeyValue

	// MetricAttributes are additional attributes to add to all metrics
	MetricAttributes []attribute.KeyValue
}

// DefaultObservabilityConfig returns default observability configuration
func DefaultObservabilityConfig() *ObservabilityConfig {
	return &ObservabilityConfig{
		EnableTracing: true,
		EnableMetrics: true,
		TracingAttributes: []attribute.KeyValue{
			attribute.String("db.system", "neo4j"),
			attribute.String("db.driver", "gopher-cypher"),
			attribute.String("db.driver.version", instrumentationVersion),
		},
		MetricAttributes: []attribute.KeyValue{
			attribute.String("db.system", "neo4j"),
			attribute.String("db.driver", "gopher-cypher"),
		},
	}
}

// observabilityInstruments holds OpenTelemetry instruments
type observabilityInstruments struct {
	tracer trace.Tracer
	meter  metric.Meter

	// Metrics
	queryDuration        metric.Float64Histogram
	queryCount           metric.Int64Counter
	connectionCount      metric.Int64UpDownCounter
	connectionErrors     metric.Int64Counter
	queryErrors          metric.Int64Counter
	recordsReturned      metric.Int64Counter
	authenticationsCount metric.Int64Counter
}

// initObservability initializes OpenTelemetry instruments
func initObservability() *observabilityInstruments {
	tracer := otel.Tracer(instrumentationName, trace.WithInstrumentationVersion(instrumentationVersion))
	meter := otel.Meter(instrumentationName, metric.WithInstrumentationVersion(instrumentationVersion))

	instruments := &observabilityInstruments{
		tracer: tracer,
		meter:  meter,
	}

	// Initialize metrics
	var err error

	instruments.queryDuration, err = meter.Float64Histogram(
		"db.query.duration",
		metric.WithDescription("Duration of database queries"),
		metric.WithUnit("s"),
	)
	if err != nil {
		otel.Handle(err)
	}

	instruments.queryCount, err = meter.Int64Counter(
		"db.query.count",
		metric.WithDescription("Number of database queries executed"),
	)
	if err != nil {
		otel.Handle(err)
	}

	instruments.connectionCount, err = meter.Int64UpDownCounter(
		"db.connection.count",
		metric.WithDescription("Number of active database connections"),
	)
	if err != nil {
		otel.Handle(err)
	}

	instruments.connectionErrors, err = meter.Int64Counter(
		"db.connection.errors",
		metric.WithDescription("Number of connection errors"),
	)
	if err != nil {
		otel.Handle(err)
	}

	instruments.queryErrors, err = meter.Int64Counter(
		"db.query.errors",
		metric.WithDescription("Number of query execution errors"),
	)
	if err != nil {
		otel.Handle(err)
	}

	instruments.recordsReturned, err = meter.Int64Counter(
		"db.query.records",
		metric.WithDescription("Number of records returned by queries"),
	)
	if err != nil {
		otel.Handle(err)
	}

	instruments.authenticationsCount, err = meter.Int64Counter(
		"db.authentication.count",
		metric.WithDescription("Number of authentication attempts"),
	)
	if err != nil {
		otel.Handle(err)
	}

	return instruments
}

// ResultSummary contains query execution metadata
type ResultSummary struct {
	// Query execution metrics
	QueryText     string
	Parameters    map[string]interface{}
	ExecutionTime time.Duration

	// Result metrics
	RecordsAvailable int64
	RecordsConsumed  int64

	// Server information
	ServerAddress string
	ServerVersion string
	Bookmark      string

	// Query classification
	QueryType string // READ, WRITE, READ_WRITE, SCHEMA_WRITE

	// Notifications from server (warnings, deprecations, etc.)
	Notifications []Notification

	// Query plan information (if available)
	Plan *QueryPlan

	// Profile information (if profiling enabled)
	Profile *QueryProfile

	// Database statistics from query execution
	NodesCreated          int64
	NodesDeleted          int64
	RelationshipsCreated  int64
	RelationshipsDeleted  int64
	PropertiesSet         int64
	LabelsAdded           int64
	LabelsRemoved         int64
	IndexesAdded          int64
	IndexesRemoved        int64
	ConstraintsAdded      int64
	ConstraintsRemoved    int64
	ContainsUpdates       bool
	ContainsSystemUpdates bool
}

// Notification represents a server notification
type Notification struct {
	Code        string
	Title       string
	Description string
	Severity    string
	Position    *Position
}

// Position represents a position in the query
type Position struct {
	Offset int
	Line   int
	Column int
}

// QueryPlan represents a query execution plan
type QueryPlan struct {
	OperatorType string
	Identifiers  []string
	Arguments    map[string]interface{}
	Children     []*QueryPlan
}

// QueryProfile represents query profiling information
type QueryProfile struct {
	OperatorType string
	Identifiers  []string
	Arguments    map[string]interface{}
	DbHits       int64
	Rows         int64
	Time         time.Duration
	Children     []*QueryProfile
}

// spanContext holds span-specific context information
type spanContext struct {
	span      trace.Span
	startTime time.Time
}

// startQuerySpan creates a new tracing span for a query
func (oi *observabilityInstruments) startQuerySpan(ctx context.Context, query string, params map[string]interface{}, config *ObservabilityConfig) (context.Context, *spanContext) {
	if !config.EnableTracing {
		return ctx, &spanContext{startTime: time.Now()}
	}

	attrs := make([]attribute.KeyValue, 0, len(config.TracingAttributes)+3)
	attrs = append(attrs, config.TracingAttributes...)
	attrs = append(attrs,
		attribute.String("db.statement", query),
		attribute.String("db.operation", inferQueryType(query)),
	)

	// Add parameter count (avoid logging actual param values for security)
	if len(params) > 0 {
		attrs = append(attrs, attribute.Int("db.statement.parameter_count", len(params)))
	}

	ctx, span := oi.tracer.Start(ctx, "db.query",
		trace.WithAttributes(attrs...),
		trace.WithSpanKind(trace.SpanKindClient),
	)

	return ctx, &spanContext{
		span:      span,
		startTime: time.Now(),
	}
}

// finishQuerySpan completes a query span with results
func (oi *observabilityInstruments) finishQuerySpan(spanCtx *spanContext, summary *ResultSummary, err error, config *ObservabilityConfig) {
	duration := time.Since(spanCtx.startTime)

	// Record metrics if enabled
	if config.EnableMetrics {
		attrs := metric.WithAttributes(config.MetricAttributes...)

		// Record query duration
		oi.queryDuration.Record(context.Background(), duration.Seconds(), attrs)

		// Record query count
		queryTypeAttr := attribute.String("query.type", summary.QueryType)
		statusAttr := attribute.String("query.status", "success")
		if err != nil {
			statusAttr = attribute.String("query.status", "error")
			oi.queryErrors.Add(context.Background(), 1, metric.WithAttributes(append(config.MetricAttributes, queryTypeAttr, statusAttr)...))
		} else {
			oi.queryCount.Add(context.Background(), 1, metric.WithAttributes(append(config.MetricAttributes, queryTypeAttr, statusAttr)...))

			// Record records returned
			if summary.RecordsConsumed > 0 {
				oi.recordsReturned.Add(context.Background(), summary.RecordsConsumed, attrs)
			}
		}
	}

	// Finish tracing span if enabled
	if config.EnableTracing && spanCtx.span != nil {
		// Add result attributes
		spanCtx.span.SetAttributes(
			attribute.Int64("db.query.records_returned", summary.RecordsConsumed),
			attribute.Float64("db.query.duration_ms", float64(duration.Nanoseconds())/1e6),
			attribute.String("db.query.type", summary.QueryType),
		)

		// Add notifications as events
		for _, notification := range summary.Notifications {
			spanCtx.span.AddEvent("db.notification", trace.WithAttributes(
				attribute.String("notification.code", notification.Code),
				attribute.String("notification.title", notification.Title),
				attribute.String("notification.severity", notification.Severity),
			))
		}

		if err != nil {
			spanCtx.span.RecordError(err)
			spanCtx.span.SetStatus(codes.Error, err.Error())
		} else {
			spanCtx.span.SetStatus(codes.Ok, "")
		}

		spanCtx.span.End()
	}
}

// recordConnectionEvent records connection-related metrics
func (oi *observabilityInstruments) recordConnectionEvent(eventType string, config *ObservabilityConfig, err error) {
	if !config.EnableMetrics {
		return
	}

	attrs := metric.WithAttributes(config.MetricAttributes...)

	switch eventType {
	case "connect":
		if err != nil {
			oi.connectionErrors.Add(context.Background(), 1, attrs)
		} else {
			oi.connectionCount.Add(context.Background(), 1, attrs)
		}
	case "disconnect":
		oi.connectionCount.Add(context.Background(), -1, attrs)
	case "authenticate":
		statusAttr := attribute.String("auth.status", "success")
		if err != nil {
			statusAttr = attribute.String("auth.status", "failure")
		}
		oi.authenticationsCount.Add(context.Background(), 1, metric.WithAttributes(append(config.MetricAttributes, statusAttr)...))
	}
}

// inferQueryType attempts to determine the type of query from its text
func inferQueryType(query string) string {
	// Simple heuristic - in practice, this could be more sophisticated
	queryUpper := strings.ToUpper(query)

	// Check for schema operations first (more specific)
	switch {
	case strings.Contains(queryUpper, "CREATE INDEX"), strings.Contains(queryUpper, "DROP INDEX"),
		strings.Contains(queryUpper, "CREATE CONSTRAINT"), strings.Contains(queryUpper, "DROP CONSTRAINT"):
		return "SCHEMA_WRITE"
	case strings.Contains(queryUpper, "CREATE"), strings.Contains(queryUpper, "MERGE"),
		strings.Contains(queryUpper, "SET"), strings.Contains(queryUpper, "DELETE"),
		strings.Contains(queryUpper, "REMOVE"):
		return "WRITE"
	case strings.Contains(queryUpper, "MATCH"), strings.Contains(queryUpper, "RETURN"),
		strings.Contains(queryUpper, "WITH"):
		return "READ"
	default:
		return "UNKNOWN"
	}
}
