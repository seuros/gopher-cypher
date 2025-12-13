package driver

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestEnhancedConsoleLogger_LogLevels(t *testing.T) {
	var buf bytes.Buffer
	logger := &EnhancedConsoleLogger{
		Level:            LogLevelInfo,
		Output:           &buf,
		IncludeTimestamp: false,
		ColorEnabled:     false,
	}

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	output := buf.String()

	// Debug should be filtered out
	if strings.Contains(output, "debug message") {
		t.Error("Debug message should be filtered out at INFO level")
	}

	// Others should be present
	if !strings.Contains(output, "info message") {
		t.Error("Info message should be present")
	}
	if !strings.Contains(output, "warn message") {
		t.Error("Warn message should be present")
	}
	if !strings.Contains(output, "error message") {
		t.Error("Error message should be present")
	}
}

func TestEnhancedConsoleLogger_CategoryLevels(t *testing.T) {
	var buf bytes.Buffer
	logger := &EnhancedConsoleLogger{
		Level:            LogLevelWarn, // Global level is WARN
		Output:           &buf,
		IncludeTimestamp: false,
		ColorEnabled:     false,
		CategoryLevels:   make(map[LogCategory]LogLevel),
	}

	// Set Bolt category to DEBUG level
	logger.SetCategoryLevel(LogCategoryBolt, LogLevelDebug)

	// Test general category (should follow global level)
	logger.LogWithCategory(LogLevelInfo, LogCategoryGeneral, "general info")

	// Test Bolt category (should allow debug)
	logger.LogWithCategory(LogLevelDebug, LogCategoryBolt, "bolt debug")

	output := buf.String()

	// General info should be filtered out (below WARN)
	if strings.Contains(output, "general info") {
		t.Error("General info should be filtered out at WARN level")
	}

	// Bolt debug should be present (category-specific DEBUG level)
	if !strings.Contains(output, "bolt debug") {
		t.Error("Bolt debug should be present with category-specific DEBUG level")
	}
}

func TestEnhancedConsoleLogger_KeyValuePairs(t *testing.T) {
	var buf bytes.Buffer
	logger := &EnhancedConsoleLogger{
		Level:            LogLevelInfo,
		Output:           &buf,
		IncludeTimestamp: false,
		ColorEnabled:     false,
	}

	logger.Info("test message", "key1", "value1", "key2", 42)

	output := buf.String()

	if !strings.Contains(output, "key1=value1") {
		t.Error("Key-value pair key1=value1 should be present")
	}
	if !strings.Contains(output, "key2=42") {
		t.Error("Key-value pair key2=42 should be present")
	}
}

func TestEnhancedStructuredLogger_JSONOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := &EnhancedStructuredLogger{
		Level:            LogLevelInfo,
		Output:           &buf,
		IncludeTimestamp: true,
		IncludeSource:    false,
	}

	logger.Info("test message", "key1", "value1", "key2", 42)

	output := strings.TrimSpace(buf.String())

	var entry LogEntry
	err := json.Unmarshal([]byte(output), &entry)
	if err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if entry.Level != LogLevelInfo {
		t.Errorf("Expected level INFO, got %v", entry.Level)
	}

	if entry.Message != "test message" {
		t.Errorf("Expected message 'test message', got '%s'", entry.Message)
	}

	if entry.Fields["key1"] != "value1" {
		t.Errorf("Expected field key1=value1, got %v", entry.Fields["key1"])
	}

	if entry.Fields["key2"] != float64(42) { // JSON unmarshals numbers as float64
		t.Errorf("Expected field key2=42, got %v", entry.Fields["key2"])
	}
}

func TestDedicatedBoltLogger_ProtocolLogging(t *testing.T) {
	var buf bytes.Buffer
	logger := &DedicatedBoltLogger{
		Level:  LogLevelDebug,
		Output: &buf,
	}

	// Test Bolt message logging
	fields := []interface{}{"test", 123, map[string]interface{}{"key": "value"}}
	logger.LogBoltMessage("SEND", "RUN", fields)

	output := buf.String()

	if !strings.Contains(output, "[BOLT]") {
		t.Error("Output should contain [BOLT] prefix")
	}
	if !strings.Contains(output, "Bolt SEND: RUN") {
		t.Error("Output should contain Bolt message details")
	}
	if !strings.Contains(output, "field_count=3") {
		t.Error("Output should contain field count")
	}
}

func TestDedicatedBoltLogger_HandshakeLogging(t *testing.T) {
	var buf bytes.Buffer
	logger := &DedicatedBoltLogger{
		Level:  LogLevelInfo,
		Output: &buf,
	}

	logger.LogBoltHandshake("5.2", "gopher-cypher/1.0", "basic")

	output := buf.String()

	if !strings.Contains(output, "Bolt handshake initiated") {
		t.Error("Output should contain handshake message")
	}
	if !strings.Contains(output, "version=5.2") {
		t.Error("Output should contain version")
	}
	if !strings.Contains(output, "client=gopher-cypher/1.0") {
		t.Error("Output should contain client name")
	}
	if !strings.Contains(output, "auth_scheme=basic") {
		t.Error("Output should contain auth scheme")
	}
}

func TestLoggerAdapter_Integration(t *testing.T) {
	var debugMessages []string
	var infoMessages []string
	var warnMessages []string
	var errorMessages []string

	adapter := NewLoggerAdapter(
		func(msg string, args ...interface{}) {
			debugMessages = append(debugMessages, msg)
		},
		func(msg string, args ...interface{}) {
			infoMessages = append(infoMessages, msg)
		},
		func(msg string, args ...interface{}) {
			warnMessages = append(warnMessages, msg)
		},
		func(msg string, args ...interface{}) {
			errorMessages = append(errorMessages, msg)
		},
		true, // debug enabled
		true, // info enabled
	)

	adapter.Debug("debug test", "key", "value")
	adapter.Info("info test", "number", 42)
	adapter.Warn("warn test")
	adapter.Error("error test")

	if len(debugMessages) != 1 || !strings.Contains(debugMessages[0], "debug test") {
		t.Error("Debug message not properly forwarded")
	}

	if len(infoMessages) != 1 || !strings.Contains(infoMessages[0], "info test") {
		t.Error("Info message not properly forwarded")
	}

	if len(warnMessages) != 1 || !strings.Contains(warnMessages[0], "warn test") {
		t.Error("Warn message not properly forwarded")
	}

	if len(errorMessages) != 1 || !strings.Contains(errorMessages[0], "error test") {
		t.Error("Error message not properly forwarded")
	}
}

func TestNewConsoleLoggingConfig(t *testing.T) {
	config := NewConsoleLoggingConfig(LogLevelDebug)

	if config.Level != LogLevelDebug {
		t.Errorf("Expected level DEBUG, got %v", config.Level)
	}

	if !config.LogBoltMessages {
		t.Error("LogBoltMessages should be enabled for DEBUG level")
	}

	if !config.LogConnectionPool {
		t.Error("LogConnectionPool should be enabled for DEBUG level")
	}

	if !config.LogQueryTiming {
		t.Error("LogQueryTiming should be enabled for DEBUG level")
	}

	// Test that logger is EnhancedConsoleLogger
	if _, ok := config.Logger.(*EnhancedConsoleLogger); !ok {
		t.Error("Logger should be EnhancedConsoleLogger")
	}
}

func TestNewStructuredLoggingConfig(t *testing.T) {
	var buf bytes.Buffer
	config := NewStructuredLoggingConfig(LogLevelInfo, &buf)

	if config.Level != LogLevelInfo {
		t.Errorf("Expected level INFO, got %v", config.Level)
	}

	if !config.StructuredOutput {
		t.Error("StructuredOutput should be enabled")
	}

	if !config.IncludeTimestamp {
		t.Error("IncludeTimestamp should be enabled")
	}

	if !config.IncludeSource {
		t.Error("IncludeSource should be enabled")
	}

	// Test that logger is EnhancedStructuredLogger
	if _, ok := config.Logger.(*EnhancedStructuredLogger); !ok {
		t.Error("Logger should be EnhancedStructuredLogger")
	}
}

func TestNewBoltTracingConfig(t *testing.T) {
	var buf bytes.Buffer
	config := NewBoltTracingConfig(LogLevelDebug, &buf)

	if config.BoltLogger == nil {
		t.Error("BoltLogger should be configured")
	}

	if !config.LogBoltMessages {
		t.Error("LogBoltMessages should be enabled")
	}

	// Test that BoltLogger is DedicatedBoltLogger
	if _, ok := config.BoltLogger.(*DedicatedBoltLogger); !ok {
		t.Error("BoltLogger should be DedicatedBoltLogger")
	}
}

func TestLogEntry_JSONSerialization(t *testing.T) {
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     LogLevelInfo,
		Category:  LogCategoryQuery,
		Message:   "test message",
		Fields: map[string]interface{}{
			"query":    "MATCH (n) RETURN n",
			"duration": 150,
		},
		Source:    "test.go:123",
		RequestID: "req-12345",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal LogEntry: %v", err)
	}

	var decoded LogEntry
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal LogEntry: %v", err)
	}

	if decoded.Level != LogLevelInfo {
		t.Errorf("Expected level INFO, got %v", decoded.Level)
	}

	if decoded.Category != LogCategoryQuery {
		t.Errorf("Expected category query, got %v", decoded.Category)
	}

	if decoded.Message != "test message" {
		t.Errorf("Expected message 'test message', got '%s'", decoded.Message)
	}

	if decoded.RequestID != "req-12345" {
		t.Errorf("Expected RequestID 'req-12345', got '%s'", decoded.RequestID)
	}
}

func TestLogCategory_Constants(t *testing.T) {
	categories := []LogCategory{
		LogCategoryGeneral,
		LogCategoryConnection,
		LogCategoryQuery,
		LogCategoryBolt,
		LogCategoryAuth,
		LogCategoryTLS,
		LogCategoryStreaming,
		LogCategoryReactive,
	}

	expectedNames := []string{
		"driver",
		"connection",
		"query",
		"bolt",
		"auth",
		"tls",
		"streaming",
		"reactive",
	}

	for i, category := range categories {
		if string(category) != expectedNames[i] {
			t.Errorf("Category %d: expected '%s', got '%s'", i, expectedNames[i], string(category))
		}
	}
}
