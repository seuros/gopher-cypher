# Gopher Cypher

![Gopher Cypher - Go Toolkit for Graph Databases](assets/img/cyq.png)

**The most advanced Go toolkit for Cypher-compatible graph databases.** Build secure, scalable applications with enterprise-grade logging, streaming queries, reactive programming, and comprehensive observability.

## **Why Gopher Cypher?**

**Key Features:**
- **First reactive programming implementation** for Go graph database drivers
-  **Pluggable enterprise logging** with structured output
-  **Memory-efficient streaming** for large result sets
-  **Built-in observability** with OpenTelemetry integration
-  **Comprehensive TLS support** with multiple certificate modes
-  **Security-first Cypher parser** with injection protection
-  **Universal IDE support** via Language Server Protocol

##  **What's New - Major Release**

###  **Enterprise Logging System**
- **Pluggable logger interface** - Integrate with any logging framework
- **Structured logging** - JSON, key-value, and custom formats
- **Performance logging** - Query timing, connection pool metrics
- **Debug tracing** - Full Bolt protocol message logging
- **Thread-safe** - Production-ready concurrency handling

###  **Streaming Result Processing**
- **Memory-efficient streaming** - Process millions of records with constant memory
- **Lazy loading** - Records fetched on-demand via cursor iteration
- **Peek capability** - Look-ahead without consuming records
- **Flexible consumption** - Stream, collect, or single record patterns

###  **Reactive Programming (Reactive Processing)**
- **Non-blocking processing** - Event-driven data consumption
- **Composable operators** - Transform, Filter, Batch, Throttle, and more
- **Automatic backpressure** - Prevents memory overflow with slow consumers
- **Error recovery** - Built-in resilience and stream continuation
- **Real-time analytics** - Perfect for live data processing pipelines

###  **OpenTelemetry Observability**
- **Distributed tracing** - Track queries across services
- **Metrics collection** - Query performance, connection health
- **Custom attributes** - Enrich telemetry with business context
- **Production monitoring** - Integration with observability platforms

##  **Quick Start Examples**

### Basic Driver Usage
```go
package main

import (
    "context"
    "log"
    "github.com/seuros/gopher-cypher/src/driver"
)

func main() {
    // Connect with comprehensive configuration
    config := &driver.Config{
        Logging: &driver.LoggingConfig{
            Logger:              &driver.ConsoleLogger{Level: driver.InfoLevel},
            LogQueryTiming:      true,
            LogConnectionPool:   true,
            LogBoltMessages:     false,
        },
        TLS: &driver.TLSConfig{
            Mode: driver.TLSModeSSL,
        },
    }

    dr, err := driver.NewDriverWithConfig("bolt://user:pass@localhost:7687", config)
    if err != nil {
        log.Fatal(err)
    }
    defer dr.Close()

    ctx := context.Background()
    cols, rows, summary, err := dr.RunWithContext(ctx,
        "MATCH (u:User) WHERE u.age > $minAge RETURN u.name, u.age",
        map[string]interface{}{"minAge": 18},
        nil,
    )
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Query executed in %v, returned %d records",
        summary.ExecutionTime, summary.RecordsConsumed)
}
```

### Streaming for Large Result Sets
```go
// Memory-efficient processing of large datasets
streamingDriver := dr.(driver.StreamingDriver)
result, err := streamingDriver.RunStream(ctx,
    "MATCH (n:LargeDataset) RETURN n ORDER BY n.timestamp", nil, nil)
if err != nil {
    log.Fatal(err)
}

// Process records one at a time - constant memory usage
count := 0
for result.Next(ctx) {
    record := result.Record()
    processRecord(record) // Only one record in memory at a time
    count++

    if count%10000 == 0 {
        log.Printf("Processed %d records so far...", count)
    }
}

if err := result.Err(); err != nil {
    log.Fatal(err)
}

summary, _ := result.Consume(ctx)
log.Printf("Streamed %d total records", summary.RecordsConsumed)
```

### Reactive Programming (Advanced)
```go
// Reactive graph database queries
reactiveDriver := dr.(driver.ReactiveDriver)
reactive, err := reactiveDriver.RunReactive(ctx,
    "MATCH (t:Transaction) RETURN t.amount, t.userId, t.timestamp", nil, nil)
if err != nil {
    log.Fatal(err)
}

// Build reactive processing pipeline
pipeline := reactive.
    // Filter high-value transactions
    Filter(func(record *driver.Record) bool {
        amount := (*record)["amount"].(float64)
        return amount > 10000.0
    }).
    // Transform and enrich data
    Transform(func(record *driver.Record) *driver.Record {
        userId := (*record)["userId"].(string)
        userData := fetchUserData(userId) // External API call
        enriched := driver.Record{
            "amount":    (*record)["amount"],
            "userId":    userId,
            "timestamp": (*record)["timestamp"],
            "userTier":  userData.Tier,
            "riskScore": userData.RiskScore,
        }
        return &enriched
    }).
    // Batch for efficient processing
    Batch(100).
    // Handle errors gracefully
    OnError(func(err error) error {
        log.Printf("Stream error: %v", err)
        return nil // Continue processing
    }).
    // Side effects without modifying stream
    DoOnNext(func(record *driver.Record) {
        // Real-time alerting
        if shouldAlert(*record) {
            sendFraudAlert(*record)
        }
    })

// Non-blocking subscription
pipeline.Subscribe(ctx, &driver.FuncSubscriber{
    OnNextFunc: func(record *driver.Record) {
        batch := (*record)["batch"].([]*driver.Record)
        processBatch(batch) // Bulk operations
    },
    OnCompleteFunc: func(summary *driver.ResultSummary) {
        log.Printf("Reactive processing completed: %d records", summary.RecordsConsumed)
    },
})

// Continue with other work while stream processes in background...
```

### Enterprise Logging Integration
```go
// Integrate with your existing logging framework
import "go.uber.org/zap"

// Custom logger adapter
type ZapLoggerAdapter struct {
    logger *zap.SugaredLogger
}

func (z *ZapLoggerAdapter) Debug(msg string, keysAndValues ...interface{}) {
    z.logger.Debugw(msg, keysAndValues...)
}

func (z *ZapLoggerAdapter) Info(msg string, keysAndValues ...interface{}) {
    z.logger.Infow(msg, keysAndValues...)
}

// Configure driver with your logger
zapLogger, _ := zap.NewProduction()
config := &driver.Config{
    Logging: &driver.LoggingConfig{
        Logger:            &ZapLoggerAdapter{logger: zapLogger.Sugar()},
        LogQueryTiming:    true,
        LogConnectionPool: true,
        LogBoltMessages:   true, // Full protocol debugging
    },
}

dr, _ := driver.NewDriverWithConfig(url, config)
```

### OpenTelemetry Observability
```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/trace"
)

// Configure with OpenTelemetry
config := &driver.Config{
    Observability: &driver.ObservabilityConfig{
        EnableTracing: true,
        EnableMetrics: true,
        TracingAttributes: []attribute.KeyValue{
            attribute.String("service.name", "user-service"),
            attribute.String("service.version", "1.0.0"),
        },
    },
}

dr, _ := driver.NewDriverWithConfig(url, config)

// Queries automatically traced and metrics collected
ctx, span := otel.Tracer("app").Start(ctx, "user-query")
defer span.End()

_, _, summary, _ := dr.RunWithContext(ctx, "MATCH (u:User) RETURN u", nil, nil)
// Automatic metrics: query duration, record count, error rates
```

##  **Security & TLS Support**

### Multiple TLS Modes
```go
// Production with certificate verification
driver.NewDriver("bolt+ssl://user:pass@graphdb.example.com:7687")

// Development with self-signed certificates
driver.NewDriver("bolt+ssc://user:pass@localhost:7687")

// Custom TLS configuration
config := &driver.Config{
    TLS: &driver.TLSConfig{
        Mode: driver.TLSModeCustom,
        Config: &tls.Config{
            ServerName: "my-graph-server",
            RootCAs:    customCertPool,
        },
    },
}
```

### Security-First Cypher Parser
```go
import "github.com/seuros/gopher-cypher/src/parser"

// Parse and validate Cypher queries
p, _ := parser.New()
query, err := p.Parse("MATCH (n:User) WHERE n.age > $age RETURN n.name")
if err != nil {
    log.Fatal("Unsafe query blocked:", err)
}

// Automatic injection protection
p.Parse("RETURN 1; DROP DATABASE;")        //  Blocked: Multiple statements
p.Parse("MATCH (n) WHERE 1=1 OR 'a'='a'") //  Blocked: Injection pattern
p.Parse("RETURN 'unsafe'")                 //  Blocked: Single quotes
```

##  **CLI Tools & IDE Integration**

### Professional Development Tools
```bash
# Install CLI tools
go install github.com/seuros/gopher-cypher/cmd/cyq@latest

# Validate and format queries
cyq lint queries/*.cypher
cyq fmt src/queries/user-management.cypher

# Explore AST structure
cyq inspect complex-query.cypher

# Start Language Server for IDE integration
cyq lsp
```

### Universal IDE Support
Works with **any LSP-capable editor**:
- **VSCode** - Intelligent completion and hover
- **IntelliJ/GoLand** - Advanced syntax analysis
- **Vim/Neovim** - Lightweight integration
- **Emacs** - Full language support
- **Zed, Sublime Text** - Modern editor support

Features:
-  **Smart completion** - Cypher keywords, labels, functions
-  **Hover information** - AST details and documentation
-  **Auto-formatting** - Consistent query styling
-  **Security warnings** - Real-time injection detection

##  **Performance & Scalability**

### Memory Efficiency
```go
// Traditional approach - loads all data into memory
cols, rows, _ := driver.Run("MATCH (n) RETURN n", nil, nil)
//  All 1M records in memory = ~500MB

// Streaming approach - constant memory usage
result, _ := streamingDriver.RunStream(ctx, "MATCH (n) RETURN n", nil, nil)
for result.Next(ctx) {
    record := result.Record()
    processRecord(record) //  Only 1 record in memory = ~500B
}
```

### Reactive Backpressure
```go
// Automatic backpressure handling prevents memory issues
config := &driver.ReactiveConfig{
    BufferSize:           1000,
    BackpressureStrategy: driver.BackpressureBuffer,
    MaxConcurrency:       10,
}

reactive := driver.NewReactiveResult(source, query, params, config)
throttled := reactive.Throttle(100 * time.Millisecond) // Rate limiting

// Never overwhelms slow consumers
throttled.Subscribe(ctx, slowSubscriber)
```

### Connection Pooling
```go
config := &driver.Config{
    ConnectionPool: &driver.PoolConfig{
        MaxSize:         50,
        IdleTimeout:     30 * time.Second,
        MaxLifetime:     1 * time.Hour,
        HealthCheckInterval: 10 * time.Second,
    },
}
```

##  **Advanced Use Cases**

### Real-time Fraud Detection
```go
fraudDetection := reactive.
    Filter(highValueTransactions).
    Transform(enrichWithUserData).
    Filter(detectSuspiciousPatterns).
    DoOnNext(sendImmediateAlert).
    BatchByTime(1 * time.Minute).
    Transform(aggregateRiskScores)

fraudDetection.Subscribe(ctx, alertingSystem)
```

### Data Pipeline Processing
```go
pipeline := reactive.
    Filter(validateData).
    Transform(cleanAndNormalize).
    Batch(1000).
    DoOnNext(bulkProcessBatch).
    OnError(retryWithBackoff).
    DoOnComplete(notifyCompletion)
```

### High-Performance Analytics
```go
analytics := reactive.
    BatchByTime(5 * time.Second).
    Transform(computeAggregates).
    DoOnNext(updateDashboard).
    DoOnNext(publishToKafka)
```

##  **Installation**

### Requirements
- **Go 1.25+** required

### Driver Library
```bash
go get github.com/seuros/gopher-cypher
```

### CLI Tools
```bash
go install github.com/seuros/gopher-cypher/cmd/cyq@latest
```

### Docker Development
```bash
git clone https://github.com/seuros/gopher-cypher
cd gopher-cypher
docker-compose up -d  # Start graph database
make test             # Run full test suite
```

## **Database Support**

Compatible with Cypher-based graph databases using the Bolt protocol:

- **Bolt Protocol** - Full v5.2/v5.8 support
- **TLS** - All configurations supported
- **Cypher** - Complete query language compatibility

Tested with Neo4j, Memgraph, and other Bolt-compatible databases.

##  **Testing & Quality**

### Comprehensive Test Suite
```bash
make test              # Full test suite
go test -cover ./...   # With coverage
go test -bench=. -benchmem ./...  # Performance benchmarks
```

### Production Ready
- **1000+ test cases** across all components
- **Extensive benchmarks** for performance validation
- **Memory leak testing** for long-running applications
- **Concurrency testing** for thread safety
- **Integration testing** with real databases

##  **Parser Coverage: 80%**

###  **Fully Supported**
- `MATCH` patterns with complex relationships
- `WHERE` conditions with all operators
- `RETURN` with aliases, functions, expressions
- `SET`, `MERGE`, `UNWIND`, `REMOVE` operations
- **Parameters** - Complete `$parameter` support
- **Functions** - Built-in and custom functions
- **Math expressions** - Complex calculations
- **Security validation** - Injection prevention

### **Coming Soon (PRs Welcome)**
- Advanced `CALL` procedures
- Complex relationship patterns
- `ORDER BY` optimizations

*I'll write these when I need them. Using ActiveCypher for now—maybe in summer.*
##  **Get Started**

1. **Install**: `go get github.com/seuros/gopher-cypher`
2. **Connect**: Configure your driver with logging and observability
3. **Query**: Use streaming or reactive processing for optimal performance
4. **Scale**: Leverage backpressure and connection pooling for production


---

Built because asking maintainers to rearchitect their codebase felt ruder than just writing my own.
