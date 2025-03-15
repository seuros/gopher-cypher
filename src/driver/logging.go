package driver

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	// LogLevelDebug logs everything including detailed protocol messages
	LogLevelDebug LogLevel = iota
	// LogLevelInfo logs general information about driver operations
	LogLevelInfo
	// LogLevelWarn logs warning messages that don't stop execution
	LogLevelWarn
	// LogLevelError logs only error conditions
	LogLevelError
	// LogLevelOff disables all logging
	LogLevelOff
)

// String returns the string representation of a log level
func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	case LogLevelOff:
		return "OFF"
	default:
		return "UNKNOWN"
	}
}

// ParseLogLevel parses a string into a LogLevel
func ParseLogLevel(level string) LogLevel {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return LogLevelDebug
	case "INFO":
		return LogLevelInfo
	case "WARN", "WARNING":
		return LogLevelWarn
	case "ERROR":
		return LogLevelError
	case "OFF", "NONE":
		return LogLevelOff
	default:
		return LogLevelInfo
	}
}

// LogCategory represents different categories of logging for granular control
type LogCategory string

const (
	// LogCategoryGeneral for general driver operations
	LogCategoryGeneral LogCategory = "driver"
	// LogCategoryConnection for connection pool and networking events
	LogCategoryConnection LogCategory = "connection"
	// LogCategoryQuery for query execution and timing
	LogCategoryQuery LogCategory = "query"
	// LogCategoryBolt for low-level Bolt protocol messages
	LogCategoryBolt LogCategory = "bolt"
	// LogCategoryAuth for authentication events
	LogCategoryAuth LogCategory = "auth"
	// LogCategoryTLS for TLS/SSL related events
	LogCategoryTLS LogCategory = "tls"
	// LogCategoryStreaming for streaming result processing
	LogCategoryStreaming LogCategory = "streaming"
	// LogCategoryReactive for reactive programming events
	LogCategoryReactive LogCategory = "reactive"
)

// LogEntry represents a structured log entry with full metadata
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     LogLevel               `json:"level"`
	Category  LogCategory            `json:"category"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Source    string                 `json:"source,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
}

// Logger defines the interface for pluggable logging in the driver.
// Compatible with Neo4j's log.Logger interface but with enhanced capabilities.
type Logger interface {
	// Debug logs a debug message with optional key-value pairs
	Debug(msg string, keysAndValues ...interface{})
	// Info logs an info message with optional key-value pairs
	Info(msg string, keysAndValues ...interface{})
	// Warn logs a warning message with optional key-value pairs
	Warn(msg string, keysAndValues ...interface{})
	// Error logs an error message with optional key-value pairs
	Error(msg string, keysAndValues ...interface{})
	// IsDebugEnabled returns true if debug logging is enabled
	IsDebugEnabled() bool
	// IsInfoEnabled returns true if info logging is enabled
	IsInfoEnabled() bool
}

// CategorizedLogger extends Logger with category-specific and leveled logging
type CategorizedLogger interface {
	Logger
	// LogWithCategory logs a message with a specific category for granular control
	LogWithCategory(level LogLevel, category LogCategory, msg string, keysAndValues ...interface{})
	// IsLevelEnabled returns true if the specified level is enabled
	IsLevelEnabled(level LogLevel) bool
	// IsCategoryEnabled returns true if the specified category is enabled
	IsCategoryEnabled(category LogCategory) bool
	// SetCategoryLevel sets the log level for a specific category
	SetCategoryLevel(category LogCategory, level LogLevel)
}

// StructuredLogger provides JSON structured logging output
type StructuredLogger interface {
	Logger
	// LogStructured logs a structured entry with full metadata
	LogStructured(entry LogEntry)
}

// BoltLogger is a specialized logger for Bolt protocol tracing
// Similar to Neo4j's dedicated Bolt debug logger
type BoltLogger interface {
	Logger
	// LogBoltMessage logs a Bolt protocol message with full details
	LogBoltMessage(direction string, messageType string, fields []interface{})
	// LogBoltHandshake logs Bolt handshake details
	LogBoltHandshake(version string, clientName string, authScheme string)
	// LogBoltError logs Bolt protocol errors
	LogBoltError(code string, message string, metadata map[string]interface{})
}

// LoggingConfig holds comprehensive logging configuration
type LoggingConfig struct {
	// Logger is the pluggable logger implementation (compatible with Neo4j's interface)
	Logger Logger
	// BoltLogger is an optional specialized logger for Bolt protocol tracing
	BoltLogger BoltLogger
	// Level sets the global minimum log level to output
	Level LogLevel
	// CategoryLevels allows setting different log levels per category
	CategoryLevels map[LogCategory]LogLevel
	// EnabledCategories specifies which categories should be logged
	EnabledCategories map[LogCategory]bool
	// StructuredOutput enables JSON structured logging format
	StructuredOutput bool
	// IncludeTimestamp includes timestamp in log output
	IncludeTimestamp bool
	// IncludeSource includes source location in logs
	IncludeSource bool
	// RequestIDEnabled enables request ID tracking
	RequestIDEnabled bool
	
	// Specific feature flags for backward compatibility
	// LogBoltMessages enables detailed Bolt protocol message logging
	LogBoltMessages bool
	// LogConnectionPool enables connection pool event logging
	LogConnectionPool bool
	// LogQueryTiming enables query execution timing logs
	LogQueryTiming bool
	// LogAuthEvents enables authentication event logging
	LogAuthEvents bool
	// LogTLSEvents enables TLS/SSL event logging
	LogTLSEvents bool
	// LogStreamingEvents enables streaming result logging
	LogStreamingEvents bool
	// LogReactiveEvents enables reactive programming event logging
	LogReactiveEvents bool
}

// DefaultLoggingConfig returns a logging configuration with no-op logger (silent by default)
// Similar to Neo4j driver's default behavior
func DefaultLoggingConfig() *LoggingConfig {
	return &LoggingConfig{
		Logger:             &NoOpLogger{},
		BoltLogger:         nil, // No Bolt logger by default
		Level:              LogLevelOff,
		CategoryLevels:     make(map[LogCategory]LogLevel),
		EnabledCategories:  make(map[LogCategory]bool),
		StructuredOutput:   false,
		IncludeTimestamp:   true,
		IncludeSource:      false,
		RequestIDEnabled:   false,
		LogBoltMessages:    false,
		LogConnectionPool:  false,
		LogQueryTiming:     false,
		LogAuthEvents:      false,
		LogTLSEvents:       false,
		LogStreamingEvents: false,
		LogReactiveEvents:  false,
	}
}

// NewConsoleLoggingConfig creates a console logging configuration similar to Neo4j's ConsoleLogger
func NewConsoleLoggingConfig(level LogLevel) *LoggingConfig {
	config := DefaultLoggingConfig()
	config.Logger = &EnhancedConsoleLogger{
		Level:            level,
		Output:           os.Stdout,
		IncludeTimestamp: true,
		IncludeSource:    false,
		ColorEnabled:     true,
	}
	config.Level = level
	config.LogBoltMessages = (level <= LogLevelDebug)
	config.LogConnectionPool = (level <= LogLevelDebug)
	config.LogQueryTiming = (level <= LogLevelInfo)
	config.LogAuthEvents = (level <= LogLevelDebug)
	config.LogTLSEvents = (level <= LogLevelDebug)
	return config
}

// NewStructuredLoggingConfig creates a structured JSON logging configuration
func NewStructuredLoggingConfig(level LogLevel, output io.Writer) *LoggingConfig {
	config := DefaultLoggingConfig()
	config.Logger = &EnhancedStructuredLogger{
		Level:            level,
		Output:           output,
		IncludeTimestamp: true,
		IncludeSource:    true,
		RequestIDEnabled: true,
	}
	config.Level = level
	config.StructuredOutput = true
	config.IncludeTimestamp = true
	config.IncludeSource = true
	config.RequestIDEnabled = true
	return config
}

// NewBoltTracingConfig creates a configuration with detailed Bolt protocol tracing
// Similar to Neo4j's Bolt debug logger
func NewBoltTracingConfig(level LogLevel, output io.Writer) *LoggingConfig {
	config := NewConsoleLoggingConfig(level)
	config.BoltLogger = &DedicatedBoltLogger{
		Level:  LogLevelDebug,
		Output: output,
	}
	config.LogBoltMessages = true
	return config
}

// NoOpLogger is a logger that does nothing (default behavior)
type NoOpLogger struct{}

func (l *NoOpLogger) Debug(msg string, keysAndValues ...interface{})   {}
func (l *NoOpLogger) Info(msg string, keysAndValues ...interface{})    {}
func (l *NoOpLogger) Warn(msg string, keysAndValues ...interface{})    {}
func (l *NoOpLogger) Error(msg string, keysAndValues ...interface{})   {}
func (l *NoOpLogger) IsDebugEnabled() bool                             { return false }
func (l *NoOpLogger) IsInfoEnabled() bool                              { return false }

// ConsoleLogger logs to stdout/stderr with configurable level and formatting
type ConsoleLogger struct {
	level      LogLevel
	debugLog   *log.Logger
	infoLog    *log.Logger
	warnLog    *log.Logger
	errorLog   *log.Logger
	mu         sync.RWMutex
	timeFormat string
}

// NewConsoleLogger creates a new console logger with the specified level
func NewConsoleLogger(level LogLevel) *ConsoleLogger {
	return &ConsoleLogger{
		level:      level,
		debugLog:   log.New(os.Stdout, "", 0),
		infoLog:    log.New(os.Stdout, "", 0),
		warnLog:    log.New(os.Stderr, "", 0),
		errorLog:   log.New(os.Stderr, "", 0),
		timeFormat: "2006-01-02 15:04:05.000",
	}
}

// NewConsoleLoggerWithOutput creates a console logger with custom output writers
func NewConsoleLoggerWithOutput(level LogLevel, stdout, stderr io.Writer) *ConsoleLogger {
	return &ConsoleLogger{
		level:      level,
		debugLog:   log.New(stdout, "", 0),
		infoLog:    log.New(stdout, "", 0),
		warnLog:    log.New(stderr, "", 0),
		errorLog:   log.New(stderr, "", 0),
		timeFormat: "2006-01-02 15:04:05.000",
	}
}

// SetLevel updates the log level
func (c *ConsoleLogger) SetLevel(level LogLevel) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.level = level
}

// SetTimeFormat sets the time format for log messages
func (c *ConsoleLogger) SetTimeFormat(format string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.timeFormat = format
}

func (c *ConsoleLogger) formatMessage(level LogLevel, msg string, keysAndValues ...interface{}) string {
	c.mu.RLock()
	timeFormat := c.timeFormat
	c.mu.RUnlock()
	
	timestamp := time.Now().Format(timeFormat)
	formatted := fmt.Sprintf("[%s] %s [gopher-cypher] %s", timestamp, level.String(), msg)
	
	// Append key-value pairs
	if len(keysAndValues) > 0 {
		var pairs []string
		for i := 0; i < len(keysAndValues); i += 2 {
			if i+1 < len(keysAndValues) {
				key := fmt.Sprintf("%v", keysAndValues[i])
				value := fmt.Sprintf("%v", keysAndValues[i+1])
				pairs = append(pairs, fmt.Sprintf("%s=%s", key, value))
			}
		}
		if len(pairs) > 0 {
			formatted += " | " + strings.Join(pairs, " ")
		}
	}
	
	return formatted
}

func (c *ConsoleLogger) Debug(msg string, keysAndValues ...interface{}) {
	c.mu.RLock()
	level := c.level
	c.mu.RUnlock()
	
	if level <= LogLevelDebug {
		c.debugLog.Println(c.formatMessage(LogLevelDebug, msg, keysAndValues...))
	}
}

func (c *ConsoleLogger) Info(msg string, keysAndValues ...interface{}) {
	c.mu.RLock()
	level := c.level
	c.mu.RUnlock()
	
	if level <= LogLevelInfo {
		c.infoLog.Println(c.formatMessage(LogLevelInfo, msg, keysAndValues...))
	}
}

func (c *ConsoleLogger) Warn(msg string, keysAndValues ...interface{}) {
	c.mu.RLock()
	level := c.level
	c.mu.RUnlock()
	
	if level <= LogLevelWarn {
		c.warnLog.Println(c.formatMessage(LogLevelWarn, msg, keysAndValues...))
	}
}

func (c *ConsoleLogger) Error(msg string, keysAndValues ...interface{}) {
	c.mu.RLock()
	level := c.level
	c.mu.RUnlock()
	
	if level <= LogLevelError {
		c.errorLog.Println(c.formatMessage(LogLevelError, msg, keysAndValues...))
	}
}

func (c *ConsoleLogger) IsDebugEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.level <= LogLevelDebug
}

func (c *ConsoleLogger) IsInfoEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.level <= LogLevelInfo
}
