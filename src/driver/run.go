package driver

import (
	"context"
	"time"

	"github.com/seuros/gopher-cypher/src/bolt/messaging"
	"github.com/seuros/gopher-cypher/src/internal/boltutil"
)

func (d *driver) Run(ctx context.Context, query string, params map[string]interface{}, metaData map[string]interface{}) ([]string, []map[string]interface{}, error) {
	cols, rows, _, err := d.RunWithContext(ctx, query, params, metaData)
	return cols, rows, err
}

func (d *driver) RunWithContext(ctx context.Context, query string, params map[string]interface{}, metaData map[string]interface{}) ([]string, []map[string]interface{}, *ResultSummary, error) {
	startTime := time.Now()

	// Log query execution start
	if d.config.Logging != nil && d.config.Logging.LogQueryTiming {
		d.logger.Info("Executing query", "query", query, "param_count", len(params))
	} else {
		d.logger.Debug("Executing query", "query", query, "params", params, "metadata", metaData)
	}

	// Initialize summary
	summary := &ResultSummary{
		QueryText:     query,
		Parameters:    params,
		ServerAddress: d.urlResolver.Address(),
		QueryType:     inferQueryType(query),
		Notifications: make([]Notification, 0),
	}

	// Start observability span
	var spanCtx *spanContext
	if d.observability != nil && d.config.Observability != nil {
		ctx, spanCtx = d.observability.startQuerySpan(ctx, query, params, d.config.Observability)
	} else {
		spanCtx = &spanContext{startTime: time.Now()}
	}

	// Record connection attempt
	if d.observability != nil && d.config.Observability != nil {
		d.observability.recordConnectionEvent("connect", d.config.Observability, nil)
	}

	// init connection
	if d.config.Logging != nil && d.config.Logging.LogConnectionPool {
		d.logger.Debug("Acquiring connection from pool")
	}

	conn, err := d.netPool.Get()
	defer d.netPool.Put(conn, err)
	if err != nil {
		d.logger.Error("Failed to acquire connection from pool", "error", err)
		if d.observability != nil && d.config.Observability != nil {
			d.observability.recordConnectionEvent("connect", d.config.Observability, err)
			d.observability.finishQuerySpan(spanCtx, summary, err, d.config.Observability)
		}
		return nil, nil, summary, err
	}

	if d.config.Logging != nil && d.config.Logging.LogConnectionPool {
		d.logger.Debug("Connection acquired from pool")
	}

	if d.config.Logging != nil && d.config.Logging.LogBoltMessages {
		d.logger.Debug("Checking Bolt protocol version")
	}

	err = boltutil.CheckVersion(conn)
	if err != nil {
		d.logger.Error("Bolt version check failed", "error", err)
		if d.observability != nil && d.config.Observability != nil {
			d.observability.finishQuerySpan(spanCtx, summary, err, d.config.Observability)
		}
		return nil, nil, summary, err
	}

	if d.config.Logging != nil && d.config.Logging.LogBoltMessages {
		d.logger.Debug("Bolt version check successful")
	}

	if d.config.Logging != nil && d.config.Logging.LogBoltMessages {
		d.logger.Debug("Sending HELLO message")
	}

	err = boltutil.SendHello(conn)
	if err != nil {
		d.logger.Error("HELLO message failed", "error", err)
		if d.observability != nil && d.config.Observability != nil {
			d.observability.finishQuerySpan(spanCtx, summary, err, d.config.Observability)
		}
		return nil, nil, summary, err
	}

	if d.config.Logging != nil && d.config.Logging.LogBoltMessages {
		d.logger.Debug("HELLO message successful")
	}

	if d.config.Logging != nil && d.config.Logging.LogBoltMessages {
		d.logger.Debug("Authenticating with server")
	}

	err = boltutil.Authenticate(conn, d.urlResolver)
	if err != nil {
		d.logger.Error("Authentication failed", "error", err)
		if d.observability != nil && d.config.Observability != nil {
			d.observability.recordConnectionEvent("authenticate", d.config.Observability, err)
			d.observability.finishQuerySpan(spanCtx, summary, err, d.config.Observability)
		}
		return nil, nil, summary, err
	}

	if d.config.Logging != nil && d.config.Logging.LogBoltMessages {
		d.logger.Debug("Authentication successful")
	}

	// Record successful authentication
	if d.observability != nil && d.config.Observability != nil {
		d.observability.recordConnectionEvent("authenticate", d.config.Observability, nil)
	}

	if d.config.Logging != nil && d.config.Logging.LogBoltMessages {
		d.logger.Debug("Sending RUN message", "query_type", summary.QueryType)
	}

	runMessage := messaging.NewRun(query, params, metaData)
	cols, rows, err := runMessage.Send(conn)

	// Complete summary
	summary.ExecutionTime = time.Since(startTime)
	if rows != nil {
		summary.RecordsConsumed = int64(len(rows))
		summary.RecordsAvailable = int64(len(rows))
	}

	// Log query completion
	if err != nil {
		d.logger.Error("Query execution failed", "error", err, "duration", summary.ExecutionTime)
	} else {
		if d.config.Logging != nil && d.config.Logging.LogQueryTiming {
			d.logger.Info("Query completed", "duration", summary.ExecutionTime, "records", summary.RecordsConsumed, "query_type", summary.QueryType)
		} else {
			d.logger.Debug("Query completed", "duration", summary.ExecutionTime, "records", summary.RecordsConsumed, "columns", len(cols))
		}
	}

	// Finish observability span
	if d.observability != nil && d.config.Observability != nil {
		d.observability.finishQuerySpan(spanCtx, summary, err, d.config.Observability)
	}

	return cols, rows, summary, err
}

// RunStream implements StreamingDriver interface for memory-efficient query processing
func (d *driver) RunStream(ctx context.Context, query string, params map[string]interface{}, metaData map[string]interface{}) (Result, error) {
	startTime := time.Now()

	// Log query execution start
	if d.config.Logging != nil && d.config.Logging.LogQueryTiming {
		d.logger.Info("Executing streaming query", "query", query, "param_count", len(params))
	} else {
		d.logger.Debug("Executing streaming query", "query", query, "params", params, "metadata", metaData)
	}

	// Initialize summary
	summary := &ResultSummary{
		QueryText:     query,
		Parameters:    params,
		ServerAddress: d.urlResolver.Address(),
		QueryType:     inferQueryType(query),
		Notifications: make([]Notification, 0),
	}

	// Start observability span
	var spanCtx *spanContext
	if d.observability != nil && d.config.Observability != nil {
		ctx, spanCtx = d.observability.startQuerySpan(ctx, query, params, d.config.Observability)
	} else {
		spanCtx = &spanContext{startTime: time.Now()}
	}

	// Record connection attempt
	if d.observability != nil && d.config.Observability != nil {
		d.observability.recordConnectionEvent("connect", d.config.Observability, nil)
	}

	// Get connection from pool
	if d.config.Logging != nil && d.config.Logging.LogConnectionPool {
		d.logger.Debug("Acquiring connection from pool for streaming")
	}

	conn, err := d.netPool.Get()
	if err != nil {
		// Return connection to pool even on Get() error - pool may have allocated resources
		d.netPool.Put(conn, err)
		d.logger.Error("Failed to acquire connection from pool", "error", err)
		if d.observability != nil && d.config.Observability != nil {
			d.observability.recordConnectionEvent("connect", d.config.Observability, err)
			d.observability.finishQuerySpan(spanCtx, summary, err, d.config.Observability)
		}
		return nil, err
	}

	// Note: We don't defer Put() here because the streaming connection needs to keep
	// the connection alive until the result is consumed

	if d.config.Logging != nil && d.config.Logging.LogConnectionPool {
		d.logger.Debug("Connection acquired from pool for streaming")
	}

	if d.config.Logging != nil && d.config.Logging.LogBoltMessages {
		d.logger.Debug("Checking Bolt protocol version for streaming")
	}

	err = boltutil.CheckVersion(conn)
	if err != nil {
		d.logger.Error("Bolt version check failed", "error", err)
		d.netPool.Put(conn, err)
		if d.observability != nil && d.config.Observability != nil {
			d.observability.finishQuerySpan(spanCtx, summary, err, d.config.Observability)
		}
		return nil, err
	}

	if d.config.Logging != nil && d.config.Logging.LogBoltMessages {
		d.logger.Debug("Bolt version check successful for streaming")
	}

	if d.config.Logging != nil && d.config.Logging.LogBoltMessages {
		d.logger.Debug("Sending HELLO message for streaming")
	}

	err = boltutil.SendHello(conn)
	if err != nil {
		d.logger.Error("HELLO message failed", "error", err)
		d.netPool.Put(conn, err)
		if d.observability != nil && d.config.Observability != nil {
			d.observability.finishQuerySpan(spanCtx, summary, err, d.config.Observability)
		}
		return nil, err
	}

	if d.config.Logging != nil && d.config.Logging.LogBoltMessages {
		d.logger.Debug("HELLO message successful for streaming")
	}

	if d.config.Logging != nil && d.config.Logging.LogBoltMessages {
		d.logger.Debug("Authenticating with server for streaming")
	}

	err = boltutil.Authenticate(conn, d.urlResolver)
	if err != nil {
		d.logger.Error("Authentication failed", "error", err)
		d.netPool.Put(conn, err)
		if d.observability != nil && d.config.Observability != nil {
			d.observability.recordConnectionEvent("authenticate", d.config.Observability, err)
			d.observability.finishQuerySpan(spanCtx, summary, err, d.config.Observability)
		}
		return nil, err
	}

	if d.config.Logging != nil && d.config.Logging.LogBoltMessages {
		d.logger.Debug("Authentication successful for streaming")
	}

	// Record successful authentication
	if d.observability != nil && d.config.Observability != nil {
		d.observability.recordConnectionEvent("authenticate", d.config.Observability, nil)
	}

	// Create streaming connection wrapper
	streamConn := &streamingConnectionWrapper{
		conn:          conn,
		netPool:       d.netPool,
		query:         query,
		params:        params,
		metaData:      metaData,
		logger:        d.logger,
		config:        d.config,
		observability: d.observability,
		spanCtx:       spanCtx,
		summary:       summary,
		startTime:     startTime,
	}

	// Send RUN message and get keys
	err = streamConn.sendRun(ctx)
	if err != nil {
		streamConn.Close()
		return nil, err
	}

	// Create streaming result
	result := NewStreamingResult(streamConn, query, params)

	return result, nil
}

// RunReactive implements ReactiveDriver interface for non-blocking, event-driven query processing
func (d *driver) RunReactive(ctx context.Context, query string, params map[string]interface{}, metaData map[string]interface{}) (ReactiveResult, error) {
	// First get a streaming result
	streamingResult, err := d.RunStream(ctx, query, params, metaData)
	if err != nil {
		return nil, err
	}

	// Create reactive configuration
	config := DefaultReactiveConfig()
	if d.config.Logging != nil {
		d.logger.Debug("Creating reactive result", "query", query, "buffer_size", config.BufferSize)
	}

	// Wrap streaming result in reactive interface
	reactiveResult := NewReactiveResult(streamingResult, query, params, config)

	if d.config.Logging != nil && d.config.Logging.LogQueryTiming {
		d.logger.Info("Reactive query initialized", "query_type", inferQueryType(query))
	}

	return reactiveResult, nil
}
