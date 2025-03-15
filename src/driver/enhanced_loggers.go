package driver

import (
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"
	"time"
)

// EnhancedConsoleLogger provides rich console logging with colors and formatting
// Similar to Neo4j's ConsoleLogger but with enhanced capabilities
type EnhancedConsoleLogger struct {
	Level            LogLevel
	Output           io.Writer
	IncludeTimestamp bool
	IncludeSource    bool
	ColorEnabled     bool
	CategoryLevels   map[LogCategory]LogLevel
	mu               sync.RWMutex
}

// Color codes for different log levels
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorGray   = "\033[37m"
	ColorGreen  = "\033[32m"
)

func (l *EnhancedConsoleLogger) colorForLevel(level LogLevel) string {
	if !l.ColorEnabled {
		return ""
	}
	switch level {
	case LogLevelDebug:
		return ColorGray
	case LogLevelInfo:
		return ColorBlue
	case LogLevelWarn:
		return ColorYellow
	case LogLevelError:
		return ColorRed
	default:
		return ColorReset
	}
}

func (l *EnhancedConsoleLogger) shouldLog(level LogLevel, category LogCategory) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	// Check category-specific level first
	if categoryLevel, exists := l.CategoryLevels[category]; exists {
		return level >= categoryLevel
	}
	
	// Fall back to global level
	return level >= l.Level
}

func (l *EnhancedConsoleLogger) log(level LogLevel, category LogCategory, msg string, keysAndValues ...interface{}) {
	if !l.shouldLog(level, category) {
		return
	}
	
	var parts []string
	
	// Timestamp
	if l.IncludeTimestamp {
		parts = append(parts, time.Now().Format("2006-01-02 15:04:05.000"))
	}
	
	// Level with color
	color := l.colorForLevel(level)
	reset := ColorReset
	if !l.ColorEnabled {
		reset = ""
	}
	parts = append(parts, fmt.Sprintf("%s%-5s%s", color, level.String(), reset))
	
	// Category
	parts = append(parts, fmt.Sprintf("[%s]", string(category)))
	
	// Source location
	if l.IncludeSource {
		if _, file, line, ok := runtime.Caller(3); ok {
			// Extract just the filename
			parts := strings.Split(file, "/")
			filename := parts[len(parts)-1]
			parts = append(parts, fmt.Sprintf("%s:%d", filename, line))
		}
	}
	
	// Message
	parts = append(parts, msg)
	
	// Key-value pairs
	if len(keysAndValues) > 0 {
		kvPairs := formatKeyValues(keysAndValues)
		if len(kvPairs) > 0 {
			parts = append(parts, kvPairs)
		}
	}
	
	output := strings.Join(parts, " ") + "\n"
	l.Output.Write([]byte(output))
}

func (l *EnhancedConsoleLogger) Debug(msg string, keysAndValues ...interface{}) {
	l.log(LogLevelDebug, LogCategoryGeneral, msg, keysAndValues...)
}

func (l *EnhancedConsoleLogger) Info(msg string, keysAndValues ...interface{}) {
	l.log(LogLevelInfo, LogCategoryGeneral, msg, keysAndValues...)
}

func (l *EnhancedConsoleLogger) Warn(msg string, keysAndValues ...interface{}) {
	l.log(LogLevelWarn, LogCategoryGeneral, msg, keysAndValues...)
}

func (l *EnhancedConsoleLogger) Error(msg string, keysAndValues ...interface{}) {
	l.log(LogLevelError, LogCategoryGeneral, msg, keysAndValues...)
}

func (l *EnhancedConsoleLogger) IsDebugEnabled() bool {
	return l.Level <= LogLevelDebug
}

func (l *EnhancedConsoleLogger) IsInfoEnabled() bool {
	return l.Level <= LogLevelInfo
}

// LogWithCategory implements CategorizedLogger interface
func (l *EnhancedConsoleLogger) LogWithCategory(level LogLevel, category LogCategory, msg string, keysAndValues ...interface{}) {
	l.log(level, category, msg, keysAndValues...)
}

func (l *EnhancedConsoleLogger) IsLevelEnabled(level LogLevel) bool {
	return level >= l.Level
}

func (l *EnhancedConsoleLogger) IsCategoryEnabled(category LogCategory) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	if categoryLevel, exists := l.CategoryLevels[category]; exists {
		return categoryLevel != LogLevelOff
	}
	return l.Level != LogLevelOff
}

func (l *EnhancedConsoleLogger) SetCategoryLevel(category LogCategory, level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	if l.CategoryLevels == nil {
		l.CategoryLevels = make(map[LogCategory]LogLevel)
	}
	l.CategoryLevels[category] = level
}

// EnhancedStructuredLogger provides JSON structured logging output
type EnhancedStructuredLogger struct {
	Level            LogLevel
	Output           io.Writer
	IncludeTimestamp bool
	IncludeSource    bool
	RequestIDEnabled bool
	CategoryLevels   map[LogCategory]LogLevel
	mu               sync.RWMutex
}

func (l *EnhancedStructuredLogger) shouldLog(level LogLevel, category LogCategory) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	if categoryLevel, exists := l.CategoryLevels[category]; exists {
		return level >= categoryLevel
	}
	return level >= l.Level
}

func (l *EnhancedStructuredLogger) log(level LogLevel, category LogCategory, msg string, keysAndValues ...interface{}) {
	if !l.shouldLog(level, category) {
		return
	}
	
	entry := LogEntry{
		Level:    level,
		Category: category,
		Message:  msg,
		Fields:   make(map[string]interface{}),
	}
	
	if l.IncludeTimestamp {
		entry.Timestamp = time.Now()
	}
	
	if l.IncludeSource {
		if _, file, line, ok := runtime.Caller(3); ok {
			parts := strings.Split(file, "/")
			filename := parts[len(parts)-1]
			entry.Source = fmt.Sprintf("%s:%d", filename, line)
		}
	}
	
	// Parse key-value pairs into fields
	if len(keysAndValues) > 0 {
		entry.Fields = parseKeyValues(keysAndValues)
	}
	
	l.LogStructured(entry)
}

func (l *EnhancedStructuredLogger) LogStructured(entry LogEntry) {
	data, err := json.Marshal(entry)
	if err != nil {
		// Fallback to simple output if JSON marshaling fails
		l.Output.Write([]byte(fmt.Sprintf("ERROR: Failed to marshal log entry: %v\n", err)))
		return
	}
	
	l.Output.Write(append(data, '\n'))
}

func (l *EnhancedStructuredLogger) Debug(msg string, keysAndValues ...interface{}) {
	l.log(LogLevelDebug, LogCategoryGeneral, msg, keysAndValues...)
}

func (l *EnhancedStructuredLogger) Info(msg string, keysAndValues ...interface{}) {
	l.log(LogLevelInfo, LogCategoryGeneral, msg, keysAndValues...)
}

func (l *EnhancedStructuredLogger) Warn(msg string, keysAndValues ...interface{}) {
	l.log(LogLevelWarn, LogCategoryGeneral, msg, keysAndValues...)
}

func (l *EnhancedStructuredLogger) Error(msg string, keysAndValues ...interface{}) {
	l.log(LogLevelError, LogCategoryGeneral, msg, keysAndValues...)
}

func (l *EnhancedStructuredLogger) IsDebugEnabled() bool {
	return l.Level <= LogLevelDebug
}

func (l *EnhancedStructuredLogger) IsInfoEnabled() bool {
	return l.Level <= LogLevelInfo
}

func (l *EnhancedStructuredLogger) LogWithCategory(level LogLevel, category LogCategory, msg string, keysAndValues ...interface{}) {
	l.log(level, category, msg, keysAndValues...)
}

func (l *EnhancedStructuredLogger) IsLevelEnabled(level LogLevel) bool {
	return level >= l.Level
}

func (l *EnhancedStructuredLogger) IsCategoryEnabled(category LogCategory) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	if categoryLevel, exists := l.CategoryLevels[category]; exists {
		return categoryLevel != LogLevelOff
	}
	return l.Level != LogLevelOff
}

func (l *EnhancedStructuredLogger) SetCategoryLevel(category LogCategory, level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	if l.CategoryLevels == nil {
		l.CategoryLevels = make(map[LogCategory]LogLevel)
	}
	l.CategoryLevels[category] = level
}

// DedicatedBoltLogger provides specialized Bolt protocol logging
// Similar to Neo4j's dedicated Bolt debug logger
type DedicatedBoltLogger struct {
	Level  LogLevel
	Output io.Writer
	mu     sync.Mutex
}

func (l *DedicatedBoltLogger) Debug(msg string, keysAndValues ...interface{}) {
	l.log(LogLevelDebug, "BOLT", msg, keysAndValues...)
}

func (l *DedicatedBoltLogger) Info(msg string, keysAndValues ...interface{}) {
	l.log(LogLevelInfo, "BOLT", msg, keysAndValues...)
}

func (l *DedicatedBoltLogger) Warn(msg string, keysAndValues ...interface{}) {
	l.log(LogLevelWarn, "BOLT", msg, keysAndValues...)
}

func (l *DedicatedBoltLogger) Error(msg string, keysAndValues ...interface{}) {
	l.log(LogLevelError, "BOLT", msg, keysAndValues...)
}

func (l *DedicatedBoltLogger) IsDebugEnabled() bool {
	return l.Level <= LogLevelDebug
}

func (l *DedicatedBoltLogger) IsInfoEnabled() bool {
	return l.Level <= LogLevelInfo
}

func (l *DedicatedBoltLogger) log(level LogLevel, prefix string, msg string, keysAndValues ...interface{}) {
	if level < l.Level {
		return
	}
	
	l.mu.Lock()
	defer l.mu.Unlock()
	
	timestamp := time.Now().Format("15:04:05.000")
	kvPairs := formatKeyValues(keysAndValues)
	
	var output string
	if len(kvPairs) > 0 {
		output = fmt.Sprintf("%s [%s] %s %s %s\n", timestamp, prefix, level.String(), msg, kvPairs)
	} else {
		output = fmt.Sprintf("%s [%s] %s %s\n", timestamp, prefix, level.String(), msg)
	}
	
	l.Output.Write([]byte(output))
}

func (l *DedicatedBoltLogger) LogBoltMessage(direction string, messageType string, fields []interface{}) {
	if !l.IsDebugEnabled() {
		return
	}
	
	fieldsStr := "[]"
	if len(fields) > 0 {
		fieldsBytes, err := json.Marshal(fields)
		if err == nil {
			fieldsStr = string(fieldsBytes)
		}
	}
	
	l.Debug(fmt.Sprintf("Bolt %s: %s", direction, messageType), 
		"fields", fieldsStr,
		"field_count", len(fields))
}

func (l *DedicatedBoltLogger) LogBoltHandshake(version string, clientName string, authScheme string) {
	l.Info("Bolt handshake initiated",
		"version", version,
		"client", clientName,
		"auth_scheme", authScheme)
}

func (l *DedicatedBoltLogger) LogBoltError(code string, message string, metadata map[string]interface{}) {
	l.Error("Bolt protocol error",
		"code", code,
		"message", message,
		"metadata", metadata)
}

// Helper functions for formatting

func formatKeyValues(keysAndValues []interface{}) string {
	if len(keysAndValues) == 0 {
		return ""
	}
	
	var parts []string
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 < len(keysAndValues) {
			key := fmt.Sprintf("%v", keysAndValues[i])
			value := fmt.Sprintf("%v", keysAndValues[i+1])
			parts = append(parts, fmt.Sprintf("%s=%s", key, value))
		}
	}
	
	if len(parts) > 0 {
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return ""
}

func parseKeyValues(keysAndValues []interface{}) map[string]interface{} {
	fields := make(map[string]interface{})
	
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 < len(keysAndValues) {
			key := fmt.Sprintf("%v", keysAndValues[i])
			fields[key] = keysAndValues[i+1]
		}
	}
	
	return fields
}

// LoggerAdapter allows wrapping external loggers to implement our Logger interface
// Similar to how Neo4j allows custom logger implementations
type LoggerAdapter struct {
	DebugFunc func(msg string, args ...interface{})
	InfoFunc  func(msg string, args ...interface{})
	WarnFunc  func(msg string, args ...interface{})
	ErrorFunc func(msg string, args ...interface{})
	
	DebugEnabled bool
	InfoEnabled  bool
}

func (a *LoggerAdapter) Debug(msg string, keysAndValues ...interface{}) {
	if a.DebugFunc != nil && a.DebugEnabled {
		a.DebugFunc(msg+" "+formatKeyValues(keysAndValues))
	}
}

func (a *LoggerAdapter) Info(msg string, keysAndValues ...interface{}) {
	if a.InfoFunc != nil && a.InfoEnabled {
		a.InfoFunc(msg+" "+formatKeyValues(keysAndValues))
	}
}

func (a *LoggerAdapter) Warn(msg string, keysAndValues ...interface{}) {
	if a.WarnFunc != nil {
		a.WarnFunc(msg+" "+formatKeyValues(keysAndValues))
	}
}

func (a *LoggerAdapter) Error(msg string, keysAndValues ...interface{}) {
	if a.ErrorFunc != nil {
		a.ErrorFunc(msg+" "+formatKeyValues(keysAndValues))
	}
}

func (a *LoggerAdapter) IsDebugEnabled() bool {
	return a.DebugEnabled
}

func (a *LoggerAdapter) IsInfoEnabled() bool {
	return a.InfoEnabled
}

// NewLoggerAdapter creates a logger adapter from external logging functions
// This allows easy integration with existing logging frameworks
func NewLoggerAdapter(
	debugFunc, infoFunc, warnFunc, errorFunc func(msg string, args ...interface{}),
	debugEnabled, infoEnabled bool,
) Logger {
	return &LoggerAdapter{
		DebugFunc:    debugFunc,
		InfoFunc:     infoFunc,
		WarnFunc:     warnFunc,
		ErrorFunc:    errorFunc,
		DebugEnabled: debugEnabled,
		InfoEnabled:  infoEnabled,
	}
}