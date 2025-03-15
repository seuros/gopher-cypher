package driver

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// MockReactiveStreamConnection for testing reactive functionality
type MockReactiveStreamConnection struct {
	records   []*Record
	keys      []string
	index     int
	delay     time.Duration
	shouldErr bool
	closed    bool
}

func NewMockReactiveStreamConnection(records []*Record, keys []string) *MockReactiveStreamConnection {
	return &MockReactiveStreamConnection{
		records: records,
		keys:    keys,
		index:   0,
	}
}

func (m *MockReactiveStreamConnection) SetDelay(delay time.Duration) {
	m.delay = delay
}

func (m *MockReactiveStreamConnection) SetError(shouldErr bool) {
	m.shouldErr = shouldErr
}

func (m *MockReactiveStreamConnection) GetKeys() ([]string, error) {
	return m.keys, nil
}

func (m *MockReactiveStreamConnection) PullNext(ctx context.Context, batchSize int) (*Record, *ResultSummary, error) {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	
	if m.shouldErr {
		return nil, nil, fmt.Errorf("mock error")
	}
	
	if m.index >= len(m.records) {
		summary := &ResultSummary{
			RecordsConsumed:  int64(len(m.records)),
			RecordsAvailable: int64(len(m.records)),
		}
		return nil, summary, nil
	}
	
	record := m.records[m.index]
	m.index++
	return record, nil, nil
}

func (m *MockReactiveStreamConnection) Close() error {
	m.closed = true
	return nil
}

func createMockStreamingResult(records []*Record, keys []string) Result {
	conn := NewMockReactiveStreamConnection(records, keys)
	return NewStreamingResult(conn, "MOCK QUERY", nil)
}

func TestReactiveResult_BasicSubscription(t *testing.T) {
	// Setup test data
	records := []*Record{
		{"name": "CHAD", "age": 30},
		{"name": "Bob", "age": 25},
		{"name": "Charlie", "age": 35},
	}
	keys := []string{"name", "age"}
	
	streamingResult := createMockStreamingResult(records, keys)
	reactiveResult := NewReactiveResult(streamingResult, "MATCH (n) RETURN n.name, n.age", nil, DefaultReactiveConfig())
	
	ctx := context.Background()
	
	var collectedRecords []*Record
	var receivedSummary *ResultSummary
	var receivedError error
	var wg sync.WaitGroup
	
	wg.Add(1)
	
	subscriber := &FuncSubscriber{
		OnNextFunc: func(record *Record) {
			collectedRecords = append(collectedRecords, record)
		},
		OnErrorFunc: func(err error) {
			receivedError = err
			wg.Done()
		},
		OnCompleteFunc: func(summary *ResultSummary) {
			receivedSummary = summary
			wg.Done()
		},
	}
	
	err := reactiveResult.Subscribe(ctx, subscriber)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	
	wg.Wait()
	
	if receivedError != nil {
		t.Errorf("Unexpected error: %v", receivedError)
	}
	
	if len(collectedRecords) != 3 {
		t.Errorf("Expected 3 records, got %d", len(collectedRecords))
	}
	
	if receivedSummary == nil {
		t.Error("Expected summary but got nil")
	}
	
	// Verify record contents
	if (*collectedRecords[0])["name"] != "CHAD" {
		t.Errorf("First record incorrect: %v", *collectedRecords[0])
	}
}

func TestReactiveResult_Transform(t *testing.T) {
	records := []*Record{
		{"value": 1},
		{"value": 2},
		{"value": 3},
	}
	keys := []string{"value"}
	
	streamingResult := createMockStreamingResult(records, keys)
	reactiveResult := NewReactiveResult(streamingResult, "MATCH (n) RETURN n.value", nil, DefaultReactiveConfig())
	
	// Transform to double the values
	transformed := reactiveResult.Transform(func(record *Record) *Record {
		value := (*record)["value"].(int)
		newRecord := Record{"value": value * 2}
		return &newRecord
	})
	
	ctx := context.Background()
	collectedRecords, err := transformed.ToSlice(ctx)
	
	if err != nil {
		t.Fatalf("ToSlice failed: %v", err)
	}
	
	if len(collectedRecords) != 3 {
		t.Errorf("Expected 3 records, got %d", len(collectedRecords))
	}
	
	// Check transformation
	expectedValues := []int{2, 4, 6}
	for i, record := range collectedRecords {
		value := (*record)["value"].(int)
		if value != expectedValues[i] {
			t.Errorf("Record %d: expected %d, got %d", i, expectedValues[i], value)
		}
	}
}

func TestReactiveResult_Filter(t *testing.T) {
	records := []*Record{
		{"value": 1},
		{"value": 2},
		{"value": 3},
		{"value": 4},
		{"value": 5},
	}
	keys := []string{"value"}
	
	streamingResult := createMockStreamingResult(records, keys)
	reactiveResult := NewReactiveResult(streamingResult, "MATCH (n) RETURN n.value", nil, DefaultReactiveConfig())
	
	// Filter to keep only even values
	filtered := reactiveResult.Filter(func(record *Record) bool {
		value := (*record)["value"].(int)
		return value%2 == 0
	})
	
	ctx := context.Background()
	collectedRecords, err := filtered.ToSlice(ctx)
	
	if err != nil {
		t.Fatalf("ToSlice failed: %v", err)
	}
	
	if len(collectedRecords) != 2 {
		t.Errorf("Expected 2 records, got %d", len(collectedRecords))
	}
	
	// Check filtered values
	expectedValues := []int{2, 4}
	for i, record := range collectedRecords {
		value := (*record)["value"].(int)
		if value != expectedValues[i] {
			t.Errorf("Record %d: expected %d, got %d", i, expectedValues[i], value)
		}
	}
}

func TestReactiveResult_Batch(t *testing.T) {
	records := []*Record{
		{"value": 1},
		{"value": 2},
		{"value": 3},
		{"value": 4},
		{"value": 5},
	}
	keys := []string{"value"}
	
	streamingResult := createMockStreamingResult(records, keys)
	reactiveResult := NewReactiveResult(streamingResult, "MATCH (n) RETURN n.value", nil, DefaultReactiveConfig())
	
	// Batch records in groups of 2
	batched := reactiveResult.Batch(2)
	
	ctx := context.Background()
	collectedRecords, err := batched.ToSlice(ctx)
	
	if err != nil {
		t.Fatalf("ToSlice failed: %v", err)
	}
	
	// Should have 3 batch records: [1,2], [3,4], [5]
	if len(collectedRecords) != 3 {
		t.Errorf("Expected 3 batch records, got %d", len(collectedRecords))
	}
	
	// Check first batch
	firstBatch := (*collectedRecords[0])["batch"].([]*Record)
	if len(firstBatch) != 2 {
		t.Errorf("First batch should have 2 records, got %d", len(firstBatch))
	}
	
	// Check last batch (incomplete)
	lastBatch := (*collectedRecords[2])["batch"].([]*Record)
	if len(lastBatch) != 1 {
		t.Errorf("Last batch should have 1 record, got %d", len(lastBatch))
	}
}

func TestReactiveResult_Take(t *testing.T) {
	records := []*Record{
		{"value": 1},
		{"value": 2},
		{"value": 3},
		{"value": 4},
		{"value": 5},
	}
	keys := []string{"value"}
	
	streamingResult := createMockStreamingResult(records, keys)
	reactiveResult := NewReactiveResult(streamingResult, "MATCH (n) RETURN n.value", nil, DefaultReactiveConfig())
	
	// Take only first 3 records
	taken := reactiveResult.Take(3)
	
	ctx := context.Background()
	collectedRecords, err := taken.ToSlice(ctx)
	
	if err != nil {
		t.Fatalf("ToSlice failed: %v", err)
	}
	
	if len(collectedRecords) != 3 {
		t.Errorf("Expected 3 records, got %d", len(collectedRecords))
	}
	
	// Check values
	for i, record := range collectedRecords {
		value := (*record)["value"].(int)
		expected := i + 1
		if value != expected {
			t.Errorf("Record %d: expected %d, got %d", i, expected, value)
		}
	}
}

func TestReactiveResult_Skip(t *testing.T) {
	records := []*Record{
		{"value": 1},
		{"value": 2},
		{"value": 3},
		{"value": 4},
		{"value": 5},
	}
	keys := []string{"value"}
	
	streamingResult := createMockStreamingResult(records, keys)
	reactiveResult := NewReactiveResult(streamingResult, "MATCH (n) RETURN n.value", nil, DefaultReactiveConfig())
	
	// Skip first 2 records
	skipped := reactiveResult.Skip(2)
	
	ctx := context.Background()
	collectedRecords, err := skipped.ToSlice(ctx)
	
	if err != nil {
		t.Fatalf("ToSlice failed: %v", err)
	}
	
	if len(collectedRecords) != 3 {
		t.Errorf("Expected 3 records, got %d", len(collectedRecords))
	}
	
	// Check values (should be 3, 4, 5)
	expectedValues := []int{3, 4, 5}
	for i, record := range collectedRecords {
		value := (*record)["value"].(int)
		if value != expectedValues[i] {
			t.Errorf("Record %d: expected %d, got %d", i, expectedValues[i], value)
		}
	}
}

func TestReactiveResult_ChainedOperators(t *testing.T) {
	records := []*Record{
		{"value": 1},
		{"value": 2},
		{"value": 3},
		{"value": 4},
		{"value": 5},
		{"value": 6},
	}
	keys := []string{"value"}
	
	streamingResult := createMockStreamingResult(records, keys)
	reactiveResult := NewReactiveResult(streamingResult, "MATCH (n) RETURN n.value", nil, DefaultReactiveConfig())
	
	// Chain multiple operators: filter even numbers, transform to double, take 2
	result := reactiveResult.
		Filter(func(record *Record) bool {
			value := (*record)["value"].(int)
			return value%2 == 0
		}).
		Transform(func(record *Record) *Record {
			value := (*record)["value"].(int)
			newRecord := Record{"value": value * 2}
			return &newRecord
		}).
		Take(2)
	
	ctx := context.Background()
	collectedRecords, err := result.ToSlice(ctx)
	
	if err != nil {
		t.Fatalf("ToSlice failed: %v", err)
	}
	
	if len(collectedRecords) != 2 {
		t.Errorf("Expected 2 records, got %d", len(collectedRecords))
	}
	
	// Should get [4, 8] (2*2, 4*2)
	expectedValues := []int{4, 8}
	for i, record := range collectedRecords {
		value := (*record)["value"].(int)
		if value != expectedValues[i] {
			t.Errorf("Record %d: expected %d, got %d", i, expectedValues[i], value)
		}
	}
}

func TestReactiveResult_First(t *testing.T) {
	records := []*Record{
		{"name": "CHAD"},
		{"name": "Bob"},
		{"name": "Charlie"},
	}
	keys := []string{"name"}
	
	streamingResult := createMockStreamingResult(records, keys)
	reactiveResult := NewReactiveResult(streamingResult, "MATCH (n) RETURN n.name", nil, DefaultReactiveConfig())
	
	ctx := context.Background()
	firstRecord, err := reactiveResult.First(ctx)
	
	if err != nil {
		t.Fatalf("First failed: %v", err)
	}
	
	if firstRecord == nil {
		t.Fatal("First returned nil record")
	}
	
	name := (*firstRecord)["name"].(string)
	if name != "CHAD" {
		t.Errorf("Expected 'CHAD', got '%s'", name)
	}
}

func TestReactiveResult_Count(t *testing.T) {
	records := []*Record{
		{"value": 1},
		{"value": 2},
		{"value": 3},
	}
	keys := []string{"value"}
	
	streamingResult := createMockStreamingResult(records, keys)
	reactiveResult := NewReactiveResult(streamingResult, "MATCH (n) RETURN n.value", nil, DefaultReactiveConfig())
	
	ctx := context.Background()
	count, err := reactiveResult.Count(ctx)
	
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	
	if count != 3 {
		t.Errorf("Expected count 3, got %d", count)
	}
}

func TestReactiveResult_ErrorHandling(t *testing.T) {
	records := []*Record{
		{"value": 1},
		{"value": 2},
	}
	keys := []string{"value"}
	
	streamingResult := createMockStreamingResult(records, keys)
	reactiveResult := NewReactiveResult(streamingResult, "MATCH (n) RETURN n.value", nil, DefaultReactiveConfig())
	
	var receivedError error
	var wg sync.WaitGroup
	
	wg.Add(1)
	
	// Set up error in mock connection
	if mockConn, ok := streamingResult.(*StreamingResult); ok {
		if wrapper, ok := mockConn.conn.(*MockReactiveStreamConnection); ok {
			wrapper.SetError(true)
		}
	}
	
	subscriber := &FuncSubscriber{
		OnNextFunc: func(record *Record) {
			t.Error("Should not receive records when there's an error")
		},
		OnErrorFunc: func(err error) {
			receivedError = err
			wg.Done()
		},
		OnCompleteFunc: func(summary *ResultSummary) {
			t.Error("Should not complete when there's an error")
			wg.Done()
		},
	}
	
	ctx := context.Background()
	err := reactiveResult.Subscribe(ctx, subscriber)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	
	wg.Wait()
	
	if receivedError == nil {
		t.Error("Expected to receive an error")
	}
}

func TestReactiveResult_ContextCancellation(t *testing.T) {
	records := []*Record{
		{"value": 1},
		{"value": 2},
		{"value": 3},
	}
	keys := []string{"value"}
	
	streamingResult := createMockStreamingResult(records, keys)
	reactiveResult := NewReactiveResult(streamingResult, "MATCH (n) RETURN n.value", nil, DefaultReactiveConfig())
	
	// Add delay to simulate slow processing
	if mockConn, ok := streamingResult.(*StreamingResult); ok {
		if wrapper, ok := mockConn.conn.(*MockReactiveStreamConnection); ok {
			wrapper.SetDelay(100 * time.Millisecond)
		}
	}
	
	var receivedError error
	var recordCount int
	var wg sync.WaitGroup
	
	wg.Add(1)
	
	subscriber := &FuncSubscriber{
		OnNextFunc: func(record *Record) {
			recordCount++
		},
		OnErrorFunc: func(err error) {
			receivedError = err
			wg.Done()
		},
		OnCompleteFunc: func(summary *ResultSummary) {
			wg.Done()
		},
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	
	err := reactiveResult.Subscribe(ctx, subscriber)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	
	wg.Wait()
	
	if receivedError == nil {
		t.Error("Expected timeout error due to context cancellation")
	}
	
	if recordCount >= 3 {
		t.Error("Should not have processed all records due to cancellation")
	}
}

func TestReactiveResult_DoOnNext(t *testing.T) {
	records := []*Record{
		{"value": 1},
		{"value": 2},
		{"value": 3},
	}
	keys := []string{"value"}
	
	streamingResult := createMockStreamingResult(records, keys)
	reactiveResult := NewReactiveResult(streamingResult, "MATCH (n) RETURN n.value", nil, DefaultReactiveConfig())
	
	var sideEffectCount int
	
	// Add side effect
	withSideEffect := reactiveResult.DoOnNext(func(record *Record) {
		sideEffectCount++
	})
	
	ctx := context.Background()
	collectedRecords, err := withSideEffect.ToSlice(ctx)
	
	if err != nil {
		t.Fatalf("ToSlice failed: %v", err)
	}
	
	if len(collectedRecords) != 3 {
		t.Errorf("Expected 3 records, got %d", len(collectedRecords))
	}
	
	if sideEffectCount != 3 {
		t.Errorf("Expected side effect to be called 3 times, got %d", sideEffectCount)
	}
}