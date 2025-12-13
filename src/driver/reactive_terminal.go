package driver

import (
	"context"
	"sync"
	"time"
)

// Terminal operations that collect/consume the reactive stream

// ToSlice collects all records into a slice (blocking operation)
func (r *reactiveResult) ToSlice(ctx context.Context) ([]*Record, error) {
	var records []*Record
	var err error
	var wg sync.WaitGroup

	wg.Add(1)

	subscriber := &sliceSubscriber{
		records: &records,
		err:     &err,
		wg:      &wg,
	}

	if subscribeErr := r.Subscribe(ctx, subscriber); subscribeErr != nil {
		return nil, subscribeErr
	}

	wg.Wait()

	if err != nil {
		return nil, err
	}

	return records, nil
}

type sliceSubscriber struct {
	records *[]*Record
	err     *error
	wg      *sync.WaitGroup
}

func (s *sliceSubscriber) OnNext(record *Record) {
	*s.records = append(*s.records, record)
}

func (s *sliceSubscriber) OnError(err error) {
	*s.err = err
	s.wg.Done()
}

func (s *sliceSubscriber) OnComplete(summary *ResultSummary) {
	s.wg.Done()
}

// First returns the first record (blocking operation)
func (r *reactiveResult) First(ctx context.Context) (*Record, error) {
	var record *Record
	var err error
	var wg sync.WaitGroup

	wg.Add(1)

	subscriber := &firstSubscriber{
		record: &record,
		err:    &err,
		wg:     &wg,
	}

	// Take only the first record
	firstResult := r.Take(1)

	if subscribeErr := firstResult.Subscribe(ctx, subscriber); subscribeErr != nil {
		return nil, subscribeErr
	}

	wg.Wait()

	if err != nil {
		return nil, err
	}

	if record == nil {
		return nil, NewUsageError("Result contains no records")
	}

	return record, nil
}

type firstSubscriber struct {
	record **Record
	err    *error
	wg     *sync.WaitGroup
	found  bool
}

func (s *firstSubscriber) OnNext(record *Record) {
	if !s.found {
		*s.record = record
		s.found = true
		s.wg.Done() // Complete immediately after first record
	}
}

func (s *firstSubscriber) OnError(err error) {
	*s.err = err
	s.wg.Done()
}

func (s *firstSubscriber) OnComplete(summary *ResultSummary) {
	if !s.found {
		s.wg.Done()
	}
}

// Count counts all records in the stream (blocking operation)
func (r *reactiveResult) Count(ctx context.Context) (int64, error) {
	var count int64
	var err error
	var wg sync.WaitGroup

	wg.Add(1)

	subscriber := &countSubscriber{
		count: &count,
		err:   &err,
		wg:    &wg,
	}

	if subscribeErr := r.Subscribe(ctx, subscriber); subscribeErr != nil {
		return 0, subscribeErr
	}

	wg.Wait()

	if err != nil {
		return 0, err
	}

	return count, nil
}

type countSubscriber struct {
	count *int64
	err   *error
	wg    *sync.WaitGroup
}

func (s *countSubscriber) OnNext(record *Record) {
	*s.count++
}

func (s *countSubscriber) OnError(err error) {
	*s.err = err
	s.wg.Done()
}

func (s *countSubscriber) OnComplete(summary *ResultSummary) {
	s.wg.Done()
}

// Common subscriber implementations for convenience

// FuncSubscriber allows using functions as subscribers
type FuncSubscriber struct {
	OnNextFunc     func(*Record)
	OnErrorFunc    func(error)
	OnCompleteFunc func(*ResultSummary)
}

func (f *FuncSubscriber) OnNext(record *Record) {
	if f.OnNextFunc != nil {
		f.OnNextFunc(record)
	}
}

func (f *FuncSubscriber) OnError(err error) {
	if f.OnErrorFunc != nil {
		f.OnErrorFunc(err)
	}
}

func (f *FuncSubscriber) OnComplete(summary *ResultSummary) {
	if f.OnCompleteFunc != nil {
		f.OnCompleteFunc(summary)
	}
}

// ChannelSubscriber writes events to a channel
type ChannelSubscriber struct {
	RecordChan chan *Record
	ErrorChan  chan error
	DoneChan   chan *ResultSummary
}

func (c *ChannelSubscriber) OnNext(record *Record) {
	if c.RecordChan != nil {
		c.RecordChan <- record
	}
}

func (c *ChannelSubscriber) OnError(err error) {
	if c.ErrorChan != nil {
		c.ErrorChan <- err
	}
	c.close()
}

func (c *ChannelSubscriber) OnComplete(summary *ResultSummary) {
	if c.DoneChan != nil {
		c.DoneChan <- summary
	}
	c.close()
}

func (c *ChannelSubscriber) close() {
	if c.RecordChan != nil {
		close(c.RecordChan)
	}
	if c.ErrorChan != nil {
		close(c.ErrorChan)
	}
	if c.DoneChan != nil {
		close(c.DoneChan)
	}
}

// NewChannelSubscriber creates a new channel subscriber with buffered channels
func NewChannelSubscriber(bufferSize int) *ChannelSubscriber {
	return &ChannelSubscriber{
		RecordChan: make(chan *Record, bufferSize),
		ErrorChan:  make(chan error, 1),
		DoneChan:   make(chan *ResultSummary, 1),
	}
}

// Backpressure handling utilities

// BackpressureHandler manages backpressure in reactive streams
type BackpressureHandler struct {
	strategy BackpressureStrategy
	buffer   chan RecordEvent
	dropped  int64
	mu       sync.RWMutex
}

// NewBackpressureHandler creates a new backpressure handler
func NewBackpressureHandler(strategy BackpressureStrategy, bufferSize int) *BackpressureHandler {
	return &BackpressureHandler{
		strategy: strategy,
		buffer:   make(chan RecordEvent, bufferSize),
	}
}

// Handle applies backpressure strategy to an event
func (bp *BackpressureHandler) Handle(ctx context.Context, event RecordEvent, output chan<- RecordEvent) error {
	switch bp.strategy {
	case BackpressureBuffer:
		select {
		case output <- event:
		case bp.buffer <- event:
		case <-ctx.Done():
			return ctx.Err()
		}

	case BackpressureDrop:
		select {
		case output <- event:
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Drop the event
			bp.mu.Lock()
			bp.dropped++
			bp.mu.Unlock()
		}

	case BackpressureBlock:
		select {
		case output <- event:
		case <-ctx.Done():
			return ctx.Err()
		}

	case BackpressureLatest:
		select {
		case output <- event:
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Try to replace buffered event with latest
			select {
			case <-bp.buffer:
				// Removed old event
			default:
				// No buffered event
			}
			select {
			case bp.buffer <- event:
			default:
				// Still couldn't buffer, drop
				bp.mu.Lock()
				bp.dropped++
				bp.mu.Unlock()
			}
		}
	}

	return nil
}

// DrainBuffer drains the backpressure buffer
func (bp *BackpressureHandler) DrainBuffer(ctx context.Context, output chan<- RecordEvent) error {
	for {
		select {
		case event := <-bp.buffer:
			select {
			case output <- event:
			case <-ctx.Done():
				return ctx.Err()
			}
		default:
			return nil
		}
	}
}

// GetDroppedCount returns the number of events dropped due to backpressure
func (bp *BackpressureHandler) GetDroppedCount() int64 {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	return bp.dropped
}

// Reactive metrics for observability

// ReactiveMetrics tracks reactive stream performance
type ReactiveMetrics struct {
	RecordsProcessed   int64
	RecordsDropped     int64
	AverageLatency     time.Duration
	ThroughputPerSec   float64
	BackpressureEvents int64
	ErrorCount         int64
	OperatorCount      int
	mu                 sync.RWMutex
}

// NewReactiveMetrics creates a new metrics tracker
func NewReactiveMetrics() *ReactiveMetrics {
	return &ReactiveMetrics{}
}

// RecordProcessed increments the processed records counter
func (m *ReactiveMetrics) RecordProcessed() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RecordsProcessed++
}

// RecordDropped increments the dropped records counter
func (m *ReactiveMetrics) RecordDropped() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RecordsDropped++
}

// RecordError increments the error counter
func (m *ReactiveMetrics) RecordError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ErrorCount++
}

// GetSnapshot returns a snapshot of current metrics
func (m *ReactiveMetrics) GetSnapshot() ReactiveMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return ReactiveMetrics{
		RecordsProcessed:   m.RecordsProcessed,
		RecordsDropped:     m.RecordsDropped,
		AverageLatency:     m.AverageLatency,
		ThroughputPerSec:   m.ThroughputPerSec,
		BackpressureEvents: m.BackpressureEvents,
		ErrorCount:         m.ErrorCount,
		OperatorCount:      m.OperatorCount,
	}
}
