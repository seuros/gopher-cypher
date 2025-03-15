package messaging

import (
	"net"
)

// Message signature constants
const (
	HelloSignature    = 0x01
	GoodbyeSignature  = 0x02
	ResetSignature    = 0x0F
	RunSignature      = 0x10
	BeginSignature    = 0x11
	CommitSignature   = 0x12
	RollbackSignature = 0x13
	DiscardSignature  = 0x2F
	PullSignature     = 0x3F
	RouteSignature    = 0x66
	LogonSignature    = 0x6A
	SuccessSignature  = 0x70
	RecordSignature   = 0x71
	IgnoredSignature  = 0x7E
	FailureSignature  = 0x7F
)

// Message is the base interface for all Bolt protocol messages
type Message interface {
	Signature() byte
	Fields() []interface{}
}

// Registry of message constructors by signature
var registry = make(map[byte]func([]interface{}) Message)

// RegisterMessage adds a message constructor to the registry
func RegisterMessage(signature byte, constructor func([]interface{}) Message) {
	registry[signature] = constructor
}

// CreateMessage creates a message instance from its signature and fields
func CreateMessage(signature byte, fields []interface{}) (Message, error) {
	constructor, exists := registry[signature]
	if !exists {
		return &GenericMessage{signature, fields}, nil
	}
	return constructor(fields), nil
}

// GenericMessage represents a Bolt protocol message not specifically handled
type GenericMessage struct {
	sig    byte
	fields []interface{}
}

func (m *GenericMessage) Signature() byte {
	return m.sig
}

func (m *GenericMessage) Fields() []interface{} {
	return m.fields
}

// Hello represents the HELLO message
type Hello struct {
	metadata map[string]interface{}
}

func NewHello(metadata map[string]interface{}) *Hello {
	return &Hello{metadata: metadata}
}

func (m *Hello) Signature() byte {
	return HelloSignature
}

func (m *Hello) Fields() []interface{} {
	return []interface{}{m.metadata}
}

func (m *Hello) Metadata() map[string]interface{} {
	return m.metadata
}

func (m *Hello) Send(conn net.Conn) (Message, error) {
	return sendRequest(m.Signature(), m.Fields(), conn)
}

// Logon represents the Login message
type Logon struct {
	metadata map[string]interface{}
}

func NewLogon(metadata map[string]interface{}) *Logon {
	return &Logon{metadata: metadata}
}

func (m *Logon) Signature() byte {
	return LogonSignature
}

func (m *Logon) Fields() []interface{} {
	return []interface{}{m.metadata}
}

func (m *Logon) Metadata() map[string]interface{} {
	return m.metadata
}

func (m *Logon) Send(conn net.Conn) (Message, error) {
	return sendRequest(m.Signature(), m.Fields(), conn)
}

// Goodbye represents the GOODBYE message
type Goodbye struct{}

func NewGoodbye() *Goodbye {
	return &Goodbye{}
}

func (m *Goodbye) Signature() byte {
	return GoodbyeSignature
}

func (m *Goodbye) Fields() []interface{} {
	return []interface{}{}
}

// Reset represents the RESET message
type Reset struct{}

func NewReset() *Reset {
	return &Reset{}
}

func (m *Reset) Signature() byte {
	return ResetSignature
}

func (m *Reset) Fields() []interface{} {
	return []interface{}{}
}

// Run represents the RUN message
type Run struct {
	query      string
	parameters map[string]interface{}
	metadata   map[string]interface{}
}

func NewRun(query string, parameters map[string]interface{}, metadata map[string]interface{}) *Run {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	if parameters == nil {
		parameters = make(map[string]interface{})
	}

	// Normalize mode if it exists and is a string
	if mode, ok := metadata["mode"].(string); ok && len(mode) > 1 {
		metadata["mode"] = mode[:1]
	}

	return &Run{
		query:      query,
		parameters: parameters,
		metadata:   metadata,
	}
}

func (m *Run) Signature() byte {
	return RunSignature
}

func (m *Run) Fields() []interface{} {
	return []interface{}{m.query, m.parameters, m.metadata}
}

func (m *Run) Query() string {
	return m.query
}

func (m *Run) Parameters() map[string]interface{} {
	return m.parameters
}

func (m *Run) Metadata() map[string]interface{} {
	return m.metadata
}

func (m *Run) Send(conn net.Conn) ([]string, []map[string]interface{}, error) {
	return sendRequestData(m.Signature(), m.Fields(), conn)
}

// Begin represents the BEGIN message
type Begin struct {
	metadata map[string]interface{}
}

func NewBegin(metadata map[string]interface{}) *Begin {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	// Set default mode if not present
	if _, exists := metadata["mode"]; !exists {
		metadata["mode"] = "write"
	}

	// Handle database name based on adapter
	if adapter, ok := metadata["adapter"].(string); ok && adapter == "memgraph" {
		// For Memgraph, remove db key entirely
		delete(metadata, "db")
	} else if modeStr, ok := metadata["mode"].(string); ok && len(modeStr) == 1 {
		// Only set db for Neo4j if not already set
		if _, exists := metadata["db"]; !exists {
			metadata["db"] = "neo4j"
		}
	}

	return &Begin{metadata: metadata}
}

func (m *Begin) Signature() byte {
	return BeginSignature
}

func (m *Begin) Fields() []interface{} {
	return []interface{}{m.metadata}
}

func (m *Begin) Metadata() map[string]interface{} {
	return m.metadata
}

// Commit represents the COMMIT message
type Commit struct{}

func NewCommit() *Commit {
	return &Commit{}
}

func (m *Commit) Signature() byte {
	return CommitSignature
}

func (m *Commit) Fields() []interface{} {
	return []interface{}{}
}

// Rollback represents the ROLLBACK message
type Rollback struct{}

func NewRollback() *Rollback {
	return &Rollback{}
}

func (m *Rollback) Signature() byte {
	return RollbackSignature
}

func (m *Rollback) Fields() []interface{} {
	return []interface{}{}
}

// Discard represents the DISCARD message
type Discard struct {
	metadata map[string]interface{}
}

func NewDiscard(metadata map[string]interface{}) *Discard {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	return &Discard{metadata: metadata}
}

func (m *Discard) Signature() byte {
	return DiscardSignature
}

func (m *Discard) Fields() []interface{} {
	return []interface{}{m.metadata}
}

func (m *Discard) Metadata() map[string]interface{} {
	return m.metadata
}

func (m *Discard) N() interface{} {
	if n, exists := m.metadata["n"]; exists {
		return n
	}
	return nil
}

func (m *Discard) QID() interface{} {
	if qid, exists := m.metadata["qid"]; exists {
		return qid
	}
	return nil
}

// Pull represents the PULL message
type Pull struct {
	metadata map[string]interface{}
}

func NewPull(metadata map[string]interface{}) *Pull {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	return &Pull{metadata: metadata}
}

func (m *Pull) Signature() byte {
	return PullSignature
}

func (m *Pull) Fields() []interface{} {
	return []interface{}{m.metadata}
}

func (m *Pull) Metadata() map[string]interface{} {
	return m.metadata
}

// Route represents the ROUTE message
type Route struct {
	metadata map[string]interface{}
}

func NewRoute(metadata map[string]interface{}) *Route {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	return &Route{metadata: metadata}
}

func (m *Route) Signature() byte {
	return RouteSignature
}

func (m *Route) Fields() []interface{} {
	return []interface{}{m.metadata}
}

func (m *Route) Metadata() map[string]interface{} {
	return m.metadata
}

// Success represents the SUCCESS message
type Success struct {
	metadata map[string]interface{}
}

func NewSuccess(fields []interface{}) Message {
	var metadata map[string]interface{}
	if len(fields) > 0 {
		if meta, ok := fields[0].(map[string]interface{}); ok {
			metadata = meta
		} else {
			metadata = make(map[string]interface{})
		}
	} else {
		metadata = make(map[string]interface{})
	}
	return &Success{metadata: metadata}
}

func (m *Success) Signature() byte {
	return SuccessSignature
}

func (m *Success) Fields() []interface{} {
	return []interface{}{m.metadata}
}

func (m *Success) Metadata() map[string]interface{} {
	return m.metadata
}

// Failure represents the FAILURE message
type Failure struct {
	metadata map[string]interface{}
}

func NewFailure(fields []interface{}) Message {
	var metadata map[string]interface{}
	if len(fields) > 0 {
		if meta, ok := fields[0].(map[string]interface{}); ok {
			metadata = meta
		} else {
			metadata = make(map[string]interface{})
		}
	} else {
		metadata = make(map[string]interface{})
	}
	return &Failure{metadata: metadata}
}

func (m *Failure) Signature() byte {
	return FailureSignature
}

func (m *Failure) Fields() []interface{} {
	return []interface{}{m.metadata}
}

func (m *Failure) Metadata() map[string]interface{} {
	return m.metadata
}

func (m *Failure) Code() string {
	if code, ok := m.metadata["code"].(string); ok {
		return code
	}
	return ""
}

func (m *Failure) Message() string {
	if msg, ok := m.metadata["message"].(string); ok {
		return msg
	}
	return ""
}

func init() {
	// Register message constructors
	RegisterMessage(SuccessSignature, func(fields []interface{}) Message {
		return NewSuccess(fields)
	})
	RegisterMessage(FailureSignature, func(fields []interface{}) Message {
		return NewFailure(fields)
	})
}
