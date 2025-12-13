package driver

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/seuros/gopher-cypher/src/bolt/messaging"
	"github.com/seuros/gopher-cypher/src/internal/boltutil"
)

// ensureAuthenticated handles connection liveness checking and conditional
// handshake. Returns the pooled connection ready for use, or an error.
// If the connection is dead or needs re-auth, it handles that transparently.
func (d *driver) ensureAuthenticated(conn net.Conn) (*pooledConn, error) {
	pc, ok := conn.(*pooledConn)
	if !ok {
		// Shouldn't happen with our dialFn, but handle gracefully
		pc = newPooledConn(conn)
	}

	// Liveness check for already-authenticated connections
	if d.config.ConnectionPool.EnableLivenessCheck && pc.isAuthenticated() {
		if !pc.isAlive() {
			d.logger.Warn("Pooled connection dead, discarding")
			// Mark as bad and get a fresh one
			d.netPool.Put(conn, errors.New("connection dead"))

			newConn, err := d.netPool.Get()
			if err != nil {
				return nil, err
			}
			pc, ok = newConn.(*pooledConn)
			if !ok {
				pc = newPooledConn(newConn)
			}
		}
	}

	// Skip handshake if connection is still authenticated and not idle too long
	if !pc.needsReauth(d.config.ConnectionPool.MaxIdleTime) {
		if d.config.Logging != nil && d.config.Logging.LogConnectionPool {
			d.logger.Debug("Reusing authenticated connection", "idle_time", pc.idleTime())
		}
		pc.touch()
		return pc, nil
	}

	// Need full handshake
	if d.config.Logging != nil && d.config.Logging.LogBoltMessages {
		d.logger.Debug("Performing Bolt handshake")
	}

	major, minor, err := boltutil.CheckVersion(pc.Conn)
	if err != nil {
		d.logger.Error("Bolt version check failed", "error", err)
		return nil, err
	}

	if d.config.Logging != nil && d.config.Logging.LogBoltMessages {
		d.logger.Debug("Bolt version negotiated", "major", major, "minor", minor)
	}

	err = boltutil.SendHello(pc.Conn)
	if err != nil {
		d.logger.Error("HELLO message failed", "error", err)
		return nil, err
	}

	if d.config.Logging != nil && d.config.Logging.LogBoltMessages {
		d.logger.Debug("HELLO message successful")
	}

	err = boltutil.Authenticate(pc.Conn, d.urlResolver)
	if err != nil {
		d.logger.Error("Authentication failed", "error", err)
		return nil, err
	}

	if d.config.Logging != nil && d.config.Logging.LogBoltMessages {
		d.logger.Debug("Authentication successful")
	}

	pc.markAuthenticated(major, minor)
	return pc, nil
}

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
		_, spanCtx = d.observability.startQuerySpan(ctx, query, params, d.config.Observability)
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

	// Ensure connection is authenticated (with liveness check and conditional handshake)
	pc, err := d.ensureAuthenticated(conn)
	if err != nil {
		d.netPool.Put(conn, err)
		if d.observability != nil && d.config.Observability != nil {
			d.observability.recordConnectionEvent("authenticate", d.config.Observability, err)
			d.observability.finishQuerySpan(spanCtx, summary, err, d.config.Observability)
		}
		return nil, nil, summary, err
	}

	// Record successful authentication
	if d.observability != nil && d.config.Observability != nil {
		d.observability.recordConnectionEvent("authenticate", d.config.Observability, nil)
	}

	if d.config.Logging != nil && d.config.Logging.LogBoltMessages {
		d.logger.Debug("Sending RUN message", "query_type", summary.QueryType)
	}

	runMessage := messaging.NewRun(query, params, metaData)
	cols, rows, queryErr := runMessage.Send(pc.Conn)

	// Complete summary
	summary.ExecutionTime = time.Since(startTime)
	if rows != nil {
		summary.RecordsConsumed = int64(len(rows))
		summary.RecordsAvailable = int64(len(rows))
	}

	// Log query completion
	if queryErr != nil {
		d.logger.Error("Query execution failed", "error", queryErr, "duration", summary.ExecutionTime)
		pc.markDirty()
	} else {
		if d.config.Logging != nil && d.config.Logging.LogQueryTiming {
			d.logger.Info("Query completed", "duration", summary.ExecutionTime, "records", summary.RecordsConsumed, "query_type", summary.QueryType)
		} else {
			d.logger.Debug("Query completed", "duration", summary.ExecutionTime, "records", summary.RecordsConsumed, "columns", len(cols))
		}
	}

	d.netPool.Put(conn, queryErr)

	// Finish observability span
	if d.observability != nil && d.config.Observability != nil {
		d.observability.finishQuerySpan(spanCtx, summary, queryErr, d.config.Observability)
	}

	return cols, rows, summary, queryErr
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
		_, spanCtx = d.observability.startQuerySpan(ctx, query, params, d.config.Observability)
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

	// Ensure connection is authenticated (with liveness check and conditional handshake)
	pc, err := d.ensureAuthenticated(conn)
	if err != nil {
		d.netPool.Put(conn, err)
		if d.observability != nil && d.config.Observability != nil {
			d.observability.recordConnectionEvent("authenticate", d.config.Observability, err)
			d.observability.finishQuerySpan(spanCtx, summary, err, d.config.Observability)
		}
		return nil, err
	}

	// Record successful authentication
	if d.observability != nil && d.config.Observability != nil {
		d.observability.recordConnectionEvent("authenticate", d.config.Observability, nil)
	}

	// Create streaming connection wrapper
	streamConn := &streamingConnectionWrapper{
		conn:          pc,
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
		_ = streamConn.Close()
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
