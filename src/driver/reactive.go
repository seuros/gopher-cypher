package driver

import (
	"context"
	"sync"
	"time"
)

// ReactiveDriver extends StreamingDriver with reactive programming capabilities
type ReactiveDriver interface {
	StreamingDriver
	// RunReactive executes a query and returns a ReactiveResult for non-blocking,
	// event-driven processing with composable operators
	RunReactive(ctx context.Context, query string, params map[string]interface{}, metaData map[string]interface{}) (ReactiveResult, error)
}

// ReactiveResult provides reactive stream processing of query results with
// composable operators, backpressure handling, and event-driven consumption
type ReactiveResult interface {
	// Subscribe consumes the reactive stream with the provided subscriber
	Subscribe(ctx context.Context, subscriber Subscriber) error

	// Records returns a channel that emits RecordEvent items
	Records(ctx context.Context) <-chan RecordEvent

	// Transform applies a transformation function to each record
	Transform(fn TransformFunc) ReactiveResult

	// Filter filters records based on a predicate function
	Filter(fn FilterFunc) ReactiveResult

	// Map transforms records to a different type
	Map(fn MapFunc) ReactiveResult

	// Batch groups records into batches of specified size
	Batch(size int) ReactiveResult

	// BatchByTime groups records into time-based batches
	BatchByTime(duration time.Duration) ReactiveResult

	// Take limits the stream to the first n records
	Take(n int64) ReactiveResult

	// Skip skips the first n records
	Skip(n int64) ReactiveResult

	// Distinct removes duplicate records based on a key function
	Distinct(keyFunc func(*Record) string) ReactiveResult

	// Throttle limits the rate of record emission
	Throttle(rate time.Duration) ReactiveResult

	// OnError handles errors in the stream
	OnError(handler ErrorHandler) ReactiveResult

	// DoOnNext performs a side effect for each record without modifying the stream
	DoOnNext(action func(*Record)) ReactiveResult

	// DoOnComplete performs a side effect when the stream completes
	DoOnComplete(action func(*ResultSummary)) ReactiveResult

	// DoOnError performs a side effect when an error occurs
	DoOnError(action func(error)) ReactiveResult

	// Keys returns the column names for this result
	Keys() ([]string, error)

	// ToSlice collects all records into a slice (blocking operation)
	ToSlice(ctx context.Context) ([]*Record, error)

	// First returns the first record (blocking operation)
	First(ctx context.Context) (*Record, error)

	// Count counts all records in the stream (blocking operation)
	Count(ctx context.Context) (int64, error)
}

// RecordEvent represents an event in the reactive stream
type RecordEvent struct {
	// Record contains the data (nil for error or completion events)
	Record *Record

	// Error contains any error that occurred (nil for successful record events)
	Error error

	// Complete indicates stream completion (true only for final event)
	Complete bool

	// Summary contains result summary (only present on completion)
	Summary *ResultSummary
}

// Subscriber defines the interface for consuming reactive streams
type Subscriber interface {
	// OnNext handles a new record
	OnNext(record *Record)

	// OnError handles an error in the stream
	OnError(err error)

	// OnComplete handles stream completion
	OnComplete(summary *ResultSummary)
}

// Function types for reactive operators
type TransformFunc func(*Record) *Record
type FilterFunc func(*Record) bool
type MapFunc func(*Record) interface{}
type ErrorHandler func(error) error

// BackpressureStrategy defines how to handle backpressure
type BackpressureStrategy int

const (
	// BackpressureBuffer buffers items when subscriber is slow
	BackpressureBuffer BackpressureStrategy = iota

	// BackpressureDrop drops items when subscriber is slow
	BackpressureDrop

	// BackpressureBlock blocks emission when subscriber is slow
	BackpressureBlock

	// BackpressureLatest keeps only the latest item when subscriber is slow
	BackpressureLatest
)

// ReactiveConfig configures reactive stream behavior
type ReactiveConfig struct {
	// BufferSize sets the size of internal buffers
	BufferSize int

	// BackpressureStrategy defines how to handle slow consumers
	BackpressureStrategy BackpressureStrategy

	// MaxConcurrency limits concurrent processing
	MaxConcurrency int

	// ErrorRecovery enables automatic error recovery
	ErrorRecovery bool

	// Metrics enables reactive stream metrics collection
	Metrics bool
}

// DefaultReactiveConfig returns default reactive configuration
func DefaultReactiveConfig() *ReactiveConfig {
	return &ReactiveConfig{
		BufferSize:           1000,
		BackpressureStrategy: BackpressureBuffer,
		MaxConcurrency:       10,
		ErrorRecovery:        true,
		Metrics:              true,
	}
}

// reactiveResult implements ReactiveResult interface
type reactiveResult struct {
	source      Result
	query       string
	params      map[string]interface{}
	config      *ReactiveConfig
	operators   []reactiveOperator
	mu          sync.RWMutex
	logger      Logger
	observables *observabilityInstruments
}

// reactiveOperator represents a composable operation in the reactive chain
type reactiveOperator interface {
	apply(ctx context.Context, input <-chan RecordEvent, output chan<- RecordEvent) error
}

// NewReactiveResult creates a new reactive result from a streaming result
func NewReactiveResult(source Result, query string, params map[string]interface{}, config *ReactiveConfig) ReactiveResult {
	if config == nil {
		config = DefaultReactiveConfig()
	}

	return &reactiveResult{
		source:    source,
		query:     query,
		params:    params,
		config:    config,
		operators: make([]reactiveOperator, 0),
	}
}

func (r *reactiveResult) Keys() ([]string, error) {
	return r.source.Keys()
}

func (r *reactiveResult) Subscribe(ctx context.Context, subscriber Subscriber) error {
	recordChan := r.Records(ctx)

	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				if err, ok := rec.(error); ok {
					subscriber.OnError(err)
				} else {
					subscriber.OnError(NewUsageError("Panic in reactive stream"))
				}
			}
		}()

		for {
			select {
			case event, ok := <-recordChan:
				if !ok {
					return // Channel closed
				}

				if event.Error != nil {
					subscriber.OnError(event.Error)
					return
				}

				if event.Complete {
					subscriber.OnComplete(event.Summary)
					return
				}

				if event.Record != nil {
					subscriber.OnNext(event.Record)
				}

			case <-ctx.Done():
				subscriber.OnError(ctx.Err())
				return
			}
		}
	}()

	return nil
}

func (r *reactiveResult) Records(ctx context.Context) <-chan RecordEvent {
	output := make(chan RecordEvent, r.config.BufferSize)

	go func() {
		defer close(output)

		// Track all goroutines for proper cleanup
		var wg sync.WaitGroup

		// Create initial source channel
		source := make(chan RecordEvent, r.config.BufferSize)

		// Start source emission with tracking
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.emitFromSource(ctx, source)
		}()

		// Apply operators in chain
		current := source
		for _, op := range r.operators {
			next := make(chan RecordEvent, r.config.BufferSize)
			wg.Add(1)
			go func(operator reactiveOperator, input <-chan RecordEvent, out chan<- RecordEvent) {
				defer wg.Done()
				defer close(out)
				_ = operator.apply(ctx, input, out)
				// Drain remaining input on context cancellation to unblock upstream
				for range input {
				}
			}(op, current, next)
			current = next
		}

		// Forward final results to output, respecting context
		done := false
		for !done {
			select {
			case event, ok := <-current:
				if !ok {
					done = true
					break
				}
				select {
				case output <- event:
				case <-ctx.Done():
					done = true
				}
			case <-ctx.Done():
				done = true
			}
		}

		// Drain remaining events from current channel to unblock upstream goroutines
		go func() {
			for range current {
			}
		}()

		// Wait for all goroutines to complete
		wg.Wait()
	}()

	return output
}

func (r *reactiveResult) emitFromSource(ctx context.Context, output chan<- RecordEvent) {
	defer close(output)

	for r.source.Next(ctx) {
		record := r.source.Record()

		// Create a copy to avoid shared state issues
		recordCopy := make(Record)
		for k, v := range *record {
			recordCopy[k] = v
		}

		event := RecordEvent{
			Record: &recordCopy,
		}

		select {
		case output <- event:
		case <-ctx.Done():
			return
		}
	}

	// Check for errors
	if err := r.source.Err(); err != nil {
		select {
		case output <- RecordEvent{Error: err}:
		case <-ctx.Done():
		}
		return
	}

	// Emit completion with summary
	summary, err := r.source.Consume(ctx)
	if err != nil {
		select {
		case output <- RecordEvent{Error: err}:
		case <-ctx.Done():
		}
		return
	}

	select {
	case output <- RecordEvent{Complete: true, Summary: summary}:
	case <-ctx.Done():
	}
}

// Operator implementations follow in next functions...
