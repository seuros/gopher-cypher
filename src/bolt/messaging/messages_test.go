package messaging

import (
	"reflect"
	"testing"
)

func TestHelloMessage(t *testing.T) {
	metadata := map[string]interface{}{
		"user_agent": "test-client/1.0",
		"scheme":     "basic",
	}

	hello := NewHello(metadata)

	if hello.Signature() != HelloSignature {
		t.Errorf("Expected signature %d, got %d", HelloSignature, hello.Signature())
	}

	if !reflect.DeepEqual(hello.Metadata(), metadata) {
		t.Errorf("Expected metadata %v, got %v", metadata, hello.Metadata())
	}

	expectedFields := []interface{}{metadata}
	if !reflect.DeepEqual(hello.Fields(), expectedFields) {
		t.Errorf("Expected fields %v, got %v", expectedFields, hello.Fields())
	}
}

func TestGoodbyeMessage(t *testing.T) {
	goodbye := NewGoodbye()

	if goodbye.Signature() != GoodbyeSignature {
		t.Errorf("Expected signature %d, got %d", GoodbyeSignature, goodbye.Signature())
	}

	if len(goodbye.Fields()) != 0 {
		t.Errorf("Expected empty fields, got %v", goodbye.Fields())
	}
}

func TestResetMessage(t *testing.T) {
	reset := NewReset()

	if reset.Signature() != ResetSignature {
		t.Errorf("Expected signature %d, got %d", ResetSignature, reset.Signature())
	}

	if len(reset.Fields()) != 0 {
		t.Errorf("Expected empty fields, got %v", reset.Fields())
	}
}

func TestRunMessage(t *testing.T) {
	query := "MATCH (n) RETURN n"
	params := map[string]interface{}{"limit": 10}
	metadata := map[string]interface{}{"mode": "read"}

	run := NewRun(query, params, metadata)

	if run.Signature() != RunSignature {
		t.Errorf("Expected signature %d, got %d", RunSignature, run.Signature())
	}

	if run.Query() != query {
		t.Errorf("Expected query %s, got %s", query, run.Query())
	}

	if !reflect.DeepEqual(run.Parameters(), params) {
		t.Errorf("Expected parameters %v, got %v", params, run.Parameters())
	}

	// Check mode normalization
	if run.Metadata()["mode"] != "r" {
		t.Errorf("Expected normalized mode 'r', got %v", run.Metadata()["mode"])
	}

	expectedFields := []interface{}{query, params, run.Metadata()}
	if !reflect.DeepEqual(run.Fields(), expectedFields) {
		t.Errorf("Expected fields %v, got %v", expectedFields, run.Fields())
	}
}

func TestRunMessageWithNilParams(t *testing.T) {
	query := "MATCH (n) RETURN count(n)"

	run := NewRun(query, nil, nil)

	if run.Parameters() == nil {
		t.Error("Expected non-nil parameters map")
	}

	if run.Metadata() == nil {
		t.Error("Expected non-nil metadata map")
	}
}

func TestBeginMessage(t *testing.T) {
	// Test with custom metadata
	metadata := map[string]interface{}{"mode": "read", "timeout": 5000}
	begin := NewBegin(metadata)

	if begin.Signature() != BeginSignature {
		t.Errorf("Expected signature %d, got %d", BeginSignature, begin.Signature())
	}

	if !reflect.DeepEqual(begin.Metadata(), metadata) {
		t.Errorf("Expected metadata %v, got %v", metadata, begin.Metadata())
	}

	// Test with nil metadata (should set defaults)
	begin = NewBegin(nil)
	if begin.Metadata()["mode"] != "write" {
		t.Errorf("Expected default mode 'write', got %v", begin.Metadata()["mode"])
	}

	// Test Memgraph adapter handling
	memgraphMeta := map[string]interface{}{"adapter": "memgraph", "db": "test"}
	begin = NewBegin(memgraphMeta)
	if _, exists := begin.Metadata()["db"]; exists {
		t.Errorf("Expected db key to be removed for Memgraph adapter")
	}

	// Test Neo4j default db
	neoMeta := map[string]interface{}{"mode": "r"}
	begin = NewBegin(neoMeta)
	if begin.Metadata()["db"] != "neo4j" {
		t.Errorf("Expected default db 'neo4j', got %v", begin.Metadata()["db"])
	}
}

func TestCommitMessage(t *testing.T) {
	commit := NewCommit()

	if commit.Signature() != CommitSignature {
		t.Errorf("Expected signature %d, got %d", CommitSignature, commit.Signature())
	}

	if len(commit.Fields()) != 0 {
		t.Errorf("Expected empty fields, got %v", commit.Fields())
	}
}

func TestRollbackMessage(t *testing.T) {
	rollback := NewRollback()

	if rollback.Signature() != RollbackSignature {
		t.Errorf("Expected signature %d, got %d", RollbackSignature, rollback.Signature())
	}

	if len(rollback.Fields()) != 0 {
		t.Errorf("Expected empty fields, got %v", rollback.Fields())
	}
}

func TestDiscardMessage(t *testing.T) {
	metadata := map[string]interface{}{"n": 10, "qid": 1}
	discard := NewDiscard(metadata)

	if discard.Signature() != DiscardSignature {
		t.Errorf("Expected signature %d, got %d", DiscardSignature, discard.Signature())
	}

	if !reflect.DeepEqual(discard.Metadata(), metadata) {
		t.Errorf("Expected metadata %v, got %v", metadata, discard.Metadata())
	}

	if discard.N() != 10 {
		t.Errorf("Expected N() to return 10, got %v", discard.N())
	}

	if discard.QID() != 1 {
		t.Errorf("Expected QID() to return 1, got %v", discard.QID())
	}

	// Test with nil metadata
	discard = NewDiscard(nil)
	if discard.Metadata() == nil {
		t.Error("Expected non-nil metadata map")
	}

	if discard.N() != nil {
		t.Errorf("Expected N() to return nil, got %v", discard.N())
	}
}

func TestPullMessage(t *testing.T) {
	metadata := map[string]interface{}{"n": 100}
	pull := NewPull(metadata)

	if pull.Signature() != PullSignature {
		t.Errorf("Expected signature %d, got %d", PullSignature, pull.Signature())
	}

	if !reflect.DeepEqual(pull.Metadata(), metadata) {
		t.Errorf("Expected metadata %v, got %v", metadata, pull.Metadata())
	}

	// Test with nil metadata
	pull = NewPull(nil)
	if pull.Metadata() == nil {
		t.Error("Expected non-nil metadata map")
	}
}

func TestRouteMessage(t *testing.T) {
	metadata := map[string]interface{}{"addresses": []string{"localhost:7687"}}
	route := NewRoute(metadata)

	if route.Signature() != RouteSignature {
		t.Errorf("Expected signature %d, got %d", RouteSignature, route.Signature())
	}

	if !reflect.DeepEqual(route.Metadata(), metadata) {
		t.Errorf("Expected metadata %v, got %v", metadata, route.Metadata())
	}

	// Test with nil metadata
	route = NewRoute(nil)
	if route.Metadata() == nil {
		t.Error("Expected non-nil metadata map")
	}
}

func TestGenericMessage(t *testing.T) {
	sig := byte(0x42) // Some undefined signature
	fields := []interface{}{"test", 123}

	generic := &GenericMessage{sig, fields}

	if generic.Signature() != sig {
		t.Errorf("Expected signature %d, got %d", sig, generic.Signature())
	}

	if !reflect.DeepEqual(generic.Fields(), fields) {
		t.Errorf("Expected fields %v, got %v", fields, generic.Fields())
	}
}

func TestMessageRegistry(t *testing.T) {
	// Test built-in registrations
	successFields := []interface{}{map[string]interface{}{"fields": []string{"name"}}}
	msg, err := CreateMessage(SuccessSignature, successFields)

	if err != nil {
		t.Fatalf("Unexpected error creating Success message: %v", err)
	}

	if msg.Signature() != SuccessSignature {
		t.Errorf("Expected signature %d, got %d", SuccessSignature, msg.Signature())
	}

	success, ok := msg.(*Success)
	if !ok {
		t.Fatalf("Expected *Success, got %T", msg)
	}

	if len(success.Metadata()) == 0 {
		t.Errorf("Expected non-empty metadata")
	}

	// Register a custom message
	customSig := byte(0xFF)
	RegisterMessage(customSig, func(fields []interface{}) Message {
		return &GenericMessage{customSig, fields}
	})

	customFields := []interface{}{"custom"}
	customMsg, err := CreateMessage(customSig, customFields)

	if err != nil {
		t.Fatalf("Unexpected error creating custom message: %v", err)
	}

	if customMsg.Signature() != customSig {
		t.Errorf("Expected signature %d, got %d", customSig, customMsg.Signature())
	}

	// Test unregistered message creates a GenericMessage
	unregisteredSig := byte(0xAA)
	unregFields := []interface{}{"unregistered"}
	unregMsg, err := CreateMessage(unregisteredSig, unregFields)

	if err != nil {
		t.Fatalf("Unexpected error creating unregistered message: %v", err)
	}

	if unregMsg.Signature() != unregisteredSig {
		t.Errorf("Expected signature %d, got %d", unregisteredSig, unregMsg.Signature())
	}

	_, ok = unregMsg.(*GenericMessage)
	if !ok {
		t.Fatalf("Expected *GenericMessage, got %T", unregMsg)
	}
}
