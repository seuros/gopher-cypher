package driver

import (
	"context"
	"strings"
	"time"
)

// Record represents a single record in a result set
type Record map[string]interface{}

// Result provides streaming access to query results using cursor-style iteration.
// This interface follows Neo4j driver patterns for memory-efficient processing
// of large result sets.
type Result interface {
	// Keys returns the column names available in the result set
	Keys() ([]string, error)

	// Next advances to the next record and returns true if a record is available.
	// Returns false when no more records or an error occurs.
	Next(ctx context.Context) bool

	// NextRecord advances to the next record and sets the record parameter.
	// Returns true if a record is available, false otherwise.
	NextRecord(ctx context.Context, record **Record) bool

	// Record returns the current record. May be nil if Next() hasn't been called
	// or returned false.
	Record() *Record

	// Peek returns true if there is a record after the current one without advancing.
	// Useful for lookahead without consuming the record.
	Peek(ctx context.Context) bool

	// PeekRecord returns the next record without advancing the cursor.
	PeekRecord(ctx context.Context, record **Record) bool

	// Err returns any error that occurred during iteration
	Err() error

	// Collect fetches all remaining records and returns them as a slice.
	// This consumes the entire result stream.
	Collect(ctx context.Context) ([]*Record, error)

	// Single returns exactly one record from the stream.
	// Returns error if zero or more than one record remains.
	Single(ctx context.Context) (*Record, error)

	// Consume discards all remaining records and returns the result summary.
	// This closes the result stream.
	Consume(ctx context.Context) (*ResultSummary, error)

	// IsOpen returns true if the result stream is still available for reading
	IsOpen() bool
}

// StreamingResult implements the Result interface with lazy loading
type StreamingResult struct {
	conn       StreamConnection
	keys       []string
	currentRec *Record
	peekedRec  *Record
	hasPeeked  bool
	summary    *ResultSummary
	err        error
	closed     bool
	query      string
	params     map[string]interface{}
	startTime  time.Time
}

// StreamConnection defines the interface for streaming connections
type StreamConnection interface {
	// PullNext fetches the next record from the stream
	PullNext(ctx context.Context, batchSize int) (*Record, *ResultSummary, error)
	// GetKeys returns the column keys for this result stream
	GetKeys() ([]string, error)
	// Close closes the stream and releases resources
	Close() error
}

// NewStreamingResult creates a new streaming result
func NewStreamingResult(conn StreamConnection, query string, params map[string]interface{}) *StreamingResult {
	return &StreamingResult{
		conn:      conn,
		query:     query,
		params:    params,
		startTime: time.Now(),
	}
}

func (r *StreamingResult) Keys() ([]string, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.keys == nil {
		r.keys, r.err = r.conn.GetKeys()
	}
	return r.keys, r.err
}

func (r *StreamingResult) Next(ctx context.Context) bool {
	if r.err != nil || r.closed {
		return false
	}

	// If we have a peeked record, use it
	if r.hasPeeked {
		r.currentRec = r.peekedRec
		r.peekedRec = nil
		r.hasPeeked = false
		return r.currentRec != nil
	}

	// Fetch next record
	r.currentRec, r.summary, r.err = r.conn.PullNext(ctx, 1)
	if r.summary != nil {
		r.closed = true
		return false
	}

	return r.currentRec != nil && r.err == nil
}

func (r *StreamingResult) NextRecord(ctx context.Context, record **Record) bool {
	hasNext := r.Next(ctx)
	if record != nil {
		*record = r.currentRec
	}
	return hasNext
}

func (r *StreamingResult) Record() *Record {
	return r.currentRec
}

func (r *StreamingResult) Peek(ctx context.Context) bool {
	if r.err != nil || r.closed {
		return false
	}

	if !r.hasPeeked {
		r.peekedRec, r.summary, r.err = r.conn.PullNext(ctx, 1)
		r.hasPeeked = true
		if r.summary != nil {
			r.closed = true
		}
	}

	return r.peekedRec != nil && r.err == nil
}

func (r *StreamingResult) PeekRecord(ctx context.Context, record **Record) bool {
	hasPeek := r.Peek(ctx)
	if record != nil {
		*record = r.peekedRec
	}
	return hasPeek
}

func (r *StreamingResult) Err() error {
	return r.err
}

func (r *StreamingResult) Collect(ctx context.Context) ([]*Record, error) {
	if r.err != nil {
		return nil, r.err
	}

	var records []*Record
	for r.Next(ctx) {
		// Create a copy of the current record to avoid issues with reuse
		recordCopy := make(Record)
		for k, v := range *r.currentRec {
			recordCopy[k] = v
		}
		records = append(records, &recordCopy)
	}

	if r.err != nil {
		return nil, r.err
	}

	return records, nil
}

func (r *StreamingResult) Single(ctx context.Context) (*Record, error) {
	if !r.Next(ctx) {
		if r.err != nil {
			return nil, r.err
		}
		return nil, NewUsageError("Result contains no records")
	}

	single := r.currentRec

	// Check if there's another record
	if r.Next(ctx) {
		// Consume the rest to clean up
		_, _ = r.Consume(ctx)
		return nil, NewUsageError("Result contains more than one record")
	}

	if r.err != nil {
		return nil, r.err
	}

	return single, nil
}

func (r *StreamingResult) Consume(ctx context.Context) (*ResultSummary, error) {
	if r.err != nil {
		return nil, r.err
	}

	// Drain remaining records
	for r.Next(ctx) {
		// Just consume them
	}

	if r.err != nil {
		return nil, r.err
	}

	// Close the connection
	_ = r.conn.Close()
	r.closed = true

	// Build summary if we don't have one
	if r.summary == nil {
		r.summary = &ResultSummary{
			QueryText:        r.query,
			Parameters:       r.params,
			ExecutionTime:    time.Since(r.startTime),
			RecordsConsumed:  0, // We don't track this in streaming mode
			RecordsAvailable: 0, // Unknown in streaming mode
		}
	}

	return r.summary, nil
}

func (r *StreamingResult) IsOpen() bool {
	return !r.closed && r.summary == nil
}

// UsageError represents an error in how the Result is being used
type UsageError struct {
	Message string
}

func (e *UsageError) Error() string {
	return e.Message
}

func NewUsageError(message string) *UsageError {
	return &UsageError{Message: message}
}

// DatabaseError represents a database server error (Neo4j, Memgraph, etc.)
type DatabaseError struct {
	Code    string
	Message string
}

func (e *DatabaseError) Error() string {
	if e.Code != "" {
		return e.Code + ": " + e.Message
	}
	return e.Message
}

// IsRetriable returns true if the error is transient and can be retried.
func (e *DatabaseError) IsRetriable() bool {
	return e.IsTransient() || e.IsClusterError() || e.IsConflict()
}

// IsTransient returns true for transient/temporary errors.
func (e *DatabaseError) IsTransient() bool {
	code := strings.ToLower(e.Code)
	msg := strings.ToLower(e.Message)

	return strings.Contains(code, "transient") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "unavailable") ||
		strings.Contains(msg, "temporarily")
}

// IsClusterError returns true for cluster/replication errors.
func (e *DatabaseError) IsClusterError() bool {
	code := strings.ToLower(e.Code)
	msg := strings.ToLower(e.Message)

	return strings.Contains(code, "notaleader") ||
		strings.Contains(code, "readonly") ||
		strings.Contains(msg, "not a leader") ||
		strings.Contains(msg, "read-only") ||
		strings.Contains(msg, "read only")
}

// IsConflict returns true for transaction conflict/deadlock errors.
func (e *DatabaseError) IsConflict() bool {
	code := strings.ToLower(e.Code)
	msg := strings.ToLower(e.Message)

	return strings.Contains(code, "deadlock") ||
		strings.Contains(code, "conflict") ||
		strings.Contains(msg, "deadlock") ||
		strings.Contains(msg, "conflicting transactions") ||
		strings.Contains(msg, "lock timeout") ||
		strings.Contains(msg, "serialization failure")
}

// IsAuthError returns true for authentication/authorization errors.
func (e *DatabaseError) IsAuthError() bool {
	code := strings.ToLower(e.Code)
	msg := strings.ToLower(e.Message)

	return strings.Contains(code, "security") ||
		strings.Contains(code, "auth") ||
		strings.Contains(msg, "authentication") ||
		strings.Contains(msg, "unauthorized")
}
