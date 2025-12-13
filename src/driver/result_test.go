package driver

import (
	"context"
	"errors"
	"testing"
)

// MockStreamConnection implements StreamConnection for testing
type MockStreamConnection struct {
	keys      []string
	records   []*Record
	summary   *ResultSummary
	index     int
	closed    bool
	pullCount int
}

func NewMockStreamConnection(keys []string, records []*Record) *MockStreamConnection {
	return &MockStreamConnection{
		keys:    keys,
		records: records,
		index:   0,
	}
}

func (m *MockStreamConnection) GetKeys() ([]string, error) {
	return m.keys, nil
}

func (m *MockStreamConnection) PullNext(ctx context.Context, batchSize int) (*Record, *ResultSummary, error) {
	m.pullCount++

	if m.index >= len(m.records) {
		// Return summary on exhaustion
		if m.summary == nil {
			m.summary = &ResultSummary{
				RecordsConsumed:  int64(len(m.records)),
				RecordsAvailable: int64(len(m.records)),
			}
		}
		return nil, m.summary, nil
	}

	record := m.records[m.index]
	m.index++
	return record, nil, nil
}

func (m *MockStreamConnection) Close() error {
	m.closed = true
	return nil
}

type ErrStreamConnection struct {
	keys   []string
	err    error
	closed bool
}

func (m *ErrStreamConnection) GetKeys() ([]string, error) {
	return m.keys, nil
}

func (m *ErrStreamConnection) PullNext(ctx context.Context, batchSize int) (*Record, *ResultSummary, error) {
	return nil, nil, m.err
}

func (m *ErrStreamConnection) Close() error {
	m.closed = true
	return nil
}

type KeysErrStreamConnection struct {
	err    error
	closed bool
}

func (m *KeysErrStreamConnection) GetKeys() ([]string, error) {
	return nil, m.err
}

func (m *KeysErrStreamConnection) PullNext(ctx context.Context, batchSize int) (*Record, *ResultSummary, error) {
	return nil, nil, errors.New("unexpected PullNext call")
}

func (m *KeysErrStreamConnection) Close() error {
	m.closed = true
	return nil
}

func TestStreamingResult_Next(t *testing.T) {
	// Setup test data
	keys := []string{"name", "age"}
	records := []*Record{
		{"name": "CHAD", "age": 30},
		{"name": "Bob", "age": 25},
		{"name": "Charlie", "age": 35},
	}

	mockConn := NewMockStreamConnection(keys, records)
	result := NewStreamingResult(mockConn, "MATCH (n) RETURN n.name, n.age", nil)

	ctx := context.Background()

	// Test Keys()
	resultKeys, err := result.Keys()
	if err != nil {
		t.Fatalf("Keys() failed: %v", err)
	}
	if len(resultKeys) != 2 || resultKeys[0] != "name" || resultKeys[1] != "age" {
		t.Errorf("Expected keys [name, age], got %v", resultKeys)
	}

	// Test iteration
	count := 0
	for result.Next(ctx) {
		record := result.Record()
		if record == nil {
			t.Error("Record() returned nil during iteration")
			continue
		}
		count++

		name := (*record)["name"].(string)
		age := (*record)["age"].(int)

		t.Logf("Record %d: name=%s, age=%d", count, name, age)
	}

	if err := result.Err(); err != nil {
		t.Errorf("Iteration failed with error: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 records, got %d", count)
	}

	// Verify connection was accessed properly
	if mockConn.pullCount != 4 { // 3 records + 1 final pull that returns summary
		t.Errorf("Expected 4 pull operations, got %d", mockConn.pullCount)
	}
}

func TestStreamingResult_Next_ClosesConnectionOnError(t *testing.T) {
	mockConn := &ErrStreamConnection{
		keys: []string{"x"},
		err:  errors.New("boom"),
	}
	result := NewStreamingResult(mockConn, "RETURN 1 AS x", nil)

	ctx := context.Background()
	if result.Next(ctx) {
		t.Fatal("Next() should return false on error")
	}

	if err := result.Err(); err == nil || err.Error() != "boom" {
		t.Fatalf("Expected error 'boom', got %v", err)
	}

	if !mockConn.closed {
		t.Error("Expected connection to be closed on error")
	}
}

func TestStreamingResult_Consume_ClosesConnectionOnError(t *testing.T) {
	mockConn := &ErrStreamConnection{
		keys: []string{"x"},
		err:  errors.New("boom"),
	}
	result := NewStreamingResult(mockConn, "RETURN 1 AS x", nil)

	ctx := context.Background()
	_ = result.Next(ctx)

	summary, err := result.Consume(ctx)
	if err == nil || err.Error() != "boom" {
		t.Fatalf("Expected Consume() error 'boom', got %v", err)
	}
	if summary == nil {
		t.Fatal("Expected non-nil summary even on error")
	}
	if !mockConn.closed {
		t.Error("Expected connection to be closed after Consume() on error")
	}
}

func TestStreamingResult_Collect(t *testing.T) {
	// Setup test data
	keys := []string{"id", "value"}
	records := []*Record{
		{"id": 1, "value": "first"},
		{"id": 2, "value": "second"},
	}

	mockConn := NewMockStreamConnection(keys, records)
	result := NewStreamingResult(mockConn, "MATCH (n) RETURN n.id, n.value", nil)

	ctx := context.Background()

	// Test Collect()
	collected, err := result.Collect(ctx)
	if err != nil {
		t.Fatalf("Collect() failed: %v", err)
	}

	if len(collected) != 2 {
		t.Errorf("Expected 2 collected records, got %d", len(collected))
	}

	// Verify first record
	first := *collected[0]
	if first["id"] != 1 || first["value"] != "first" {
		t.Errorf("First record incorrect: %v", first)
	}

	// Verify second record
	second := *collected[1]
	if second["id"] != 2 || second["value"] != "second" {
		t.Errorf("Second record incorrect: %v", second)
	}

	if !mockConn.closed {
		t.Error("Expected connection to be closed after Collect()")
	}
}

func TestStreamingResult_Single(t *testing.T) {
	// Test with exactly one record
	keys := []string{"result"}
	records := []*Record{
		{"result": "success"},
	}

	mockConn := NewMockStreamConnection(keys, records)
	result := NewStreamingResult(mockConn, "RETURN 'success' AS result", nil)

	ctx := context.Background()

	single, err := result.Single(ctx)
	if err != nil {
		t.Fatalf("Single() failed: %v", err)
	}

	if (*single)["result"] != "success" {
		t.Errorf("Single record incorrect: %v", *single)
	}

	if !mockConn.closed {
		t.Error("Expected connection to be closed after Single()")
	}
}

func TestStreamingResult_Single_NoRecords(t *testing.T) {
	// Test with no records
	keys := []string{"result"}
	records := []*Record{}

	mockConn := NewMockStreamConnection(keys, records)
	result := NewStreamingResult(mockConn, "MATCH (n) WHERE FALSE RETURN n", nil)

	ctx := context.Background()

	_, err := result.Single(ctx)
	if err == nil {
		t.Error("Single() should fail with no records")
	}

	if usageErr, ok := err.(*UsageError); !ok || usageErr.Message != "Result contains no records" {
		t.Errorf("Expected 'Result contains no records' error, got: %v", err)
	}

	if !mockConn.closed {
		t.Error("Expected connection to be closed after Single() with no records")
	}
}

func TestStreamingResult_Single_MultipleRecords(t *testing.T) {
	// Test with multiple records
	keys := []string{"value"}
	records := []*Record{
		{"value": 1},
		{"value": 2},
	}

	mockConn := NewMockStreamConnection(keys, records)
	result := NewStreamingResult(mockConn, "RETURN 1 AS value UNION RETURN 2 AS value", nil)

	ctx := context.Background()

	_, err := result.Single(ctx)
	if err == nil {
		t.Error("Single() should fail with multiple records")
	}

	if usageErr, ok := err.(*UsageError); !ok || usageErr.Message != "Result contains more than one record" {
		t.Errorf("Expected 'Result contains more than one record' error, got: %v", err)
	}

	if !mockConn.closed {
		t.Error("Expected connection to be closed after Single() with multiple records")
	}
}

func TestStreamingResult_Peek(t *testing.T) {
	// Setup test data
	keys := []string{"num"}
	records := []*Record{
		{"num": 1},
		{"num": 2},
		{"num": 3},
	}

	mockConn := NewMockStreamConnection(keys, records)
	result := NewStreamingResult(mockConn, "RETURN 1 AS num UNION RETURN 2 AS num UNION RETURN 3 AS num", nil)

	ctx := context.Background()

	// Peek at first record
	hasPeek := result.Peek(ctx)
	if !hasPeek {
		t.Error("Peek() should return true for first record")
	}

	var peekedRec *Record
	hasPeekRecord := result.PeekRecord(ctx, &peekedRec)
	if !hasPeekRecord {
		t.Error("PeekRecord() should return true for first record")
	}

	if (*peekedRec)["num"] != 1 {
		t.Errorf("Peeked record should be 1, got %v", (*peekedRec)["num"])
	}

	// Now advance with Next() - should get the same record
	if !result.Next(ctx) {
		t.Error("Next() should return true after peek")
	}

	currentRec := result.Record()
	if (*currentRec)["num"] != 1 {
		t.Errorf("Current record should be 1 after peek, got %v", (*currentRec)["num"])
	}

	// Continue with normal iteration
	if !result.Next(ctx) {
		t.Error("Next() should return true for second record")
	}

	currentRec = result.Record()
	if (*currentRec)["num"] != 2 {
		t.Errorf("Second record should be 2, got %v", (*currentRec)["num"])
	}
}

func TestStreamingResult_Keys_ClosesConnectionOnError(t *testing.T) {
	mockConn := &KeysErrStreamConnection{err: errors.New("keys failed")}
	result := NewStreamingResult(mockConn, "RETURN 1", nil)

	_, err := result.Keys()
	if err == nil || err.Error() != "keys failed" {
		t.Fatalf("Expected Keys() error 'keys failed', got %v", err)
	}
	if !mockConn.closed {
		t.Error("Expected connection to be closed when Keys() fails")
	}
}
