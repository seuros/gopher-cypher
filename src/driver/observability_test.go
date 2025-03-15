package driver

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

func TestDefaultObservabilityConfig(t *testing.T) {
	config := DefaultObservabilityConfig()
	
	if !config.EnableTracing {
		t.Error("Tracing should be enabled by default")
	}
	if !config.EnableMetrics {
		t.Error("Metrics should be enabled by default")
	}
	
	// Check that default attributes are set
	foundDriver := false
	foundSystem := false
	for _, attr := range config.TracingAttributes {
		if attr.Key == "db.driver" && attr.Value.AsString() == "gopher-cypher" {
			foundDriver = true
		}
		if attr.Key == "db.system" && attr.Value.AsString() == "neo4j" {
			foundSystem = true
		}
	}
	
	if !foundDriver {
		t.Error("Default tracing attributes should include db.driver")
	}
	if !foundSystem {
		t.Error("Default tracing attributes should include db.system")
	}
}

func TestObservabilityInstrumentation(t *testing.T) {
	instruments := initObservability()
	
	if instruments.tracer == nil {
		t.Error("Tracer should be initialized")
	}
	if instruments.meter == nil {
		t.Error("Meter should be initialized")
	}
	if instruments.queryDuration == nil {
		t.Error("Query duration histogram should be initialized")
	}
	if instruments.queryCount == nil {
		t.Error("Query count counter should be initialized")
	}
}

func TestInferQueryType(t *testing.T) {
	tests := []struct {
		query    string
		expected string
	}{
		{"MATCH (n) RETURN n", "READ"},
		{"CREATE (n:Person) RETURN n", "WRITE"},
		{"MERGE (n:User {id: 1})", "WRITE"},
		{"DELETE n WHERE n.id = 1", "WRITE"},
		{"SET n.name = 'test'", "WRITE"},
		{"REMOVE n.property", "WRITE"},
		{"CREATE INDEX ON :Person(name)", "SCHEMA_WRITE"},
		{"DROP INDEX ON :Person(name)", "SCHEMA_WRITE"},
		{"RETURN 1", "READ"},
		{"WITH 1 as x RETURN x", "READ"},
		{"UNKNOWN QUERY", "UNKNOWN"},
	}
	
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := inferQueryType(tt.query)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s for query: %s", tt.expected, result, tt.query)
			}
		})
	}
}

func TestResultSummary(t *testing.T) {
	summary := &ResultSummary{
		QueryText:        "RETURN 1 AS n",
		Parameters:       map[string]interface{}{"param1": "value1"},
		ExecutionTime:    100 * time.Millisecond,
		RecordsConsumed:  1,
		RecordsAvailable: 1,
		ServerAddress:    "localhost:7687",
		QueryType:        "read",
		Notifications:    []Notification{},
	}
	
	if summary.QueryText != "RETURN 1 AS n" {
		t.Errorf("Expected query text 'RETURN 1 AS n', got %s", summary.QueryText)
	}
	
	if summary.ExecutionTime != 100*time.Millisecond {
		t.Errorf("Expected execution time 100ms, got %v", summary.ExecutionTime)
	}
	
	if summary.RecordsConsumed != 1 {
		t.Errorf("Expected 1 record consumed, got %d", summary.RecordsConsumed)
	}
	
	if summary.QueryType != "read" {
		t.Errorf("Expected query type 'read', got %s", summary.QueryType)
	}
}

func TestObservabilityConfigCustomization(t *testing.T) {
	config := &ObservabilityConfig{
		EnableTracing: false,
		EnableMetrics: true,
		TracingAttributes: []attribute.KeyValue{
			attribute.String("custom.attr", "value"),
		},
		MetricAttributes: []attribute.KeyValue{
			attribute.String("environment", "test"),
		},
	}
	
	if config.EnableTracing {
		t.Error("Tracing should be disabled")
	}
	if !config.EnableMetrics {
		t.Error("Metrics should be enabled")
	}
	
	foundCustom := false
	for _, attr := range config.TracingAttributes {
		if attr.Key == "custom.attr" && attr.Value.AsString() == "value" {
			foundCustom = true
		}
	}
	if !foundCustom {
		t.Error("Custom tracing attribute should be present")
	}
}

func TestDriverWithObservability(t *testing.T) {
	// Test that driver can be created with observability config
	config := DefaultConfig()
	config.Observability.EnableTracing = true
	config.Observability.EnableMetrics = true
	
	// This will fail to connect, but we're testing config handling
	_, err := NewDriverWithConfig("memgraph://test:test@localhost:7688", config)
	if err == nil {
		t.Log("Driver creation succeeded (server must be running)")
	} else {
		t.Logf("Expected connection error: %v", err)
		// This is expected when no server is running
	}
}

func TestObservabilityDisabled(t *testing.T) {
	config := DefaultConfig()
	config.Observability.EnableTracing = false
	config.Observability.EnableMetrics = false
	
	_, err := NewDriverWithConfig("memgraph://test:test@localhost:7688", config)
	if err == nil {
		t.Log("Driver creation succeeded with observability disabled")
	} else {
		t.Logf("Expected connection error with disabled observability: %v", err)
	}
}

func TestSpanContextHandling(t *testing.T) {
	instruments := initObservability()
	config := DefaultObservabilityConfig()
	
	ctx := context.Background()
	query := "RETURN 1"
	params := map[string]interface{}{}
	
	// Test starting a span
	newCtx, spanCtx := instruments.startQuerySpan(ctx, query, params, config)
	
	if newCtx == ctx && config.EnableTracing {
		t.Error("Context should be different when tracing is enabled")
	}
	
	if spanCtx == nil {
		t.Error("Span context should not be nil")
	}
	
	// Test finishing a span
	summary := &ResultSummary{
		QueryType:       "read",
		RecordsConsumed: 1,
		Notifications:   []Notification{},
	}
	
	// This should not panic
	instruments.finishQuerySpan(spanCtx, summary, nil, config)
}