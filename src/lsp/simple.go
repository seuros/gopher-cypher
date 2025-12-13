package lsp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/seuros/gopher-cypher/src/parser"
)

type SimpleServer struct {
	parser    *parser.Parser
	documents map[string]string
}

type Message struct {
	JsonRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type InitializeParams struct {
	RootURI string `json:"rootUri"`
}

type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
}

type ServerCapabilities struct {
	TextDocumentSync           int                `json:"textDocumentSync"`
	HoverProvider              bool               `json:"hoverProvider"`
	CompletionProvider         *CompletionOptions `json:"completionProvider"`
	DocumentFormattingProvider bool               `json:"documentFormattingProvider,omitempty"`
}

type CompletionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters"`
}

// LSP diagnostic shapes (minimal subset)
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity,omitempty"` // 1=Error,2=Warning
	Source   string `json:"source,omitempty"`
	Message  string `json:"message"`
}

type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

func StartSimpleServer() error {
	log.SetOutput(os.Stderr)
	log.Println("Starting simple Cypher LSP server...")

	p, err := parser.New()
	if err != nil {
		return err
	}

	server := &SimpleServer{
		parser:    p,
		documents: make(map[string]string),
	}
	reader := bufio.NewReader(os.Stdin)

	for {
		msg, err := readMessage(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			// Keep going on malformed input.
			log.Printf("read error: %v", err)
			continue
		}

		response := server.handleMessage(msg)
		if response != nil {
			server.sendResponse(response)
		}
	}
}

func (s *SimpleServer) handleMessage(msg *Message) *Message {
	log.Printf("Handling message: %s", msg.Method)

	switch msg.Method {
	case "initialize":
		return &Message{
			JsonRPC: "2.0",
			ID:      msg.ID,
			Result: InitializeResult{
				Capabilities: ServerCapabilities{
					TextDocumentSync:           1,
					HoverProvider:              true,
					DocumentFormattingProvider: true,
					CompletionProvider: &CompletionOptions{
						TriggerCharacters: []string{":", ".", "(", " "},
					},
				},
			},
		}
	case "initialized":
		return nil
	case "shutdown":
		return &Message{
			JsonRPC: "2.0",
			ID:      msg.ID,
			Result:  nil,
		}
	case "exit":
		os.Exit(0)
		return nil
	case "textDocument/didOpen":
		s.handleDidOpen(msg.Params)
		return nil
	case "textDocument/didChange":
		s.handleDidChange(msg.Params)
		return nil
	case "textDocument/hover":
		return s.handleHover(msg.ID, msg.Params)
	case "textDocument/completion":
		return s.handleCompletion(msg.ID)
	case "textDocument/formatting":
		return s.handleFormatting(msg.ID, msg.Params)
	}

	return nil
}

func (s *SimpleServer) sendResponse(msg *Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	fmt.Printf("Content-Length: %d\r\n\r\n%s", len(data), data)
}

func (s *SimpleServer) sendNotification(method string, params interface{}) {
	s.sendResponse(&Message{
		JsonRPC: "2.0",
		Method:  method,
		Params:  params,
	})
}

func (s *SimpleServer) handleDidOpen(params interface{}) {
	m, ok := params.(map[string]interface{})
	if !ok {
		return
	}
	doc, ok := m["textDocument"].(map[string]interface{})
	if !ok {
		return
	}
	uri, _ := doc["uri"].(string)
	text, _ := doc["text"].(string)
	if uri == "" {
		return
	}
	s.documents[uri] = text
	s.publishDiagnostics(uri, text)
}

func (s *SimpleServer) handleDidChange(params interface{}) {
	m, ok := params.(map[string]interface{})
	if !ok {
		return
	}
	doc, ok := m["textDocument"].(map[string]interface{})
	if !ok {
		return
	}
	uri, _ := doc["uri"].(string)
	if uri == "" {
		return
	}
	changes, ok := m["contentChanges"].([]interface{})
	if !ok || len(changes) == 0 {
		return
	}
	// We advertise full sync (TextDocumentSync=1), so take full text.
	last, _ := changes[len(changes)-1].(map[string]interface{})
	text, _ := last["text"].(string)
	s.documents[uri] = text
	s.publishDiagnostics(uri, text)
}

func (s *SimpleServer) publishDiagnostics(uri, text string) {
	var diags []Diagnostic

	if _, err := s.parser.Parse(text); err != nil {
		start := Position{Line: 0, Character: 0}
		end := Position{Line: 0, Character: 1}

		var perr participle.Error
		if errors.As(err, &perr) {
			pos := perr.Position()
			if pos.Line > 0 {
				start.Line = pos.Line - 1
				end.Line = start.Line
			}
			if pos.Column > 0 {
				start.Character = pos.Column - 1
				end.Character = start.Character + 1
			}
		}

		diags = append(diags, Diagnostic{
			Range: Range{
				Start: start,
				End:   end,
			},
			Severity: 1,
			Source:   "gopher-cypher",
			Message:  err.Error(),
		})
	}

	s.sendNotification("textDocument/publishDiagnostics", map[string]interface{}{
		"uri":         uri,
		"diagnostics": diags,
	})
}

// readMessage reads a single JSON-RPC message according to LSP framing.
func readMessage(r *bufio.Reader) (*Message, error) {
	headers := make(map[string]string)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key != "" {
			headers[key] = value
		}
	}

	lengthStr, ok := headers["Content-Length"]
	if !ok {
		// Some clients may send lowercase headers.
		lengthStr = headers["content-length"]
	}
	if lengthStr == "" {
		return nil, fmt.Errorf("missing Content-Length")
	}
	length, err := strconv.Atoi(lengthStr)
	if err != nil || length < 0 {
		return nil, fmt.Errorf("invalid Content-Length: %q", lengthStr)
	}

	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}

	var msg Message
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func (s *SimpleServer) handleFormatting(id interface{}, params interface{}) *Message {
	uri, text := s.getURIAndText(params)
	if uri == "" {
		return errorResponse(id, -32602, "missing textDocument.uri")
	}

	parsed, err := s.parser.Parse(text)
	if err != nil {
		return errorResponse(id, -32603, err.Error())
	}

	formatted, _ := parsed.BuildCypher()
	edit := TextEdit{
		Range:   fullDocumentRange(text),
		NewText: formatted + "\n",
	}

	return &Message{
		JsonRPC: "2.0",
		ID:      id,
		Result:  []TextEdit{edit},
	}
}

func (s *SimpleServer) handleHover(id interface{}, params interface{}) *Message {
	uri, text, line, character := s.getHoverContext(params)
	if uri == "" || text == "" {
		return &Message{JsonRPC: "2.0", ID: id, Result: nil}
	}

	word := wordAtPosition(text, line, character)
	upper := strings.ToUpper(word)

	docs := map[string]string{
		"MATCH":    "Matches a simple node pattern.",
		"OPTIONAL": "Optional match; returns nulls when no match.",
		"MERGE":    "Matches or creates a node pattern.",
		"UNWIND":   "Expands a list into rows.",
		"WHERE":    "Filters results using a comparison.",
		"RETURN":   "Projects values from the match.",
		"SET":      "Updates properties.",
		"REMOVE":   "Removes properties.",
		"SKIP":     "Skips the first N rows.",
		"LIMIT":    "Limits results to N rows.",
		"AS":       "Aliases a return item.",
	}

	value := "**Cypher**\n\n"
	if d, ok := docs[upper]; ok {
		value += d
	} else if strings.HasPrefix(word, "$") {
		value += "Parameter reference."
	} else if word != "" {
		value += "Identifier."
	} else {
		value += "Cypher element."
	}

	return &Message{
		JsonRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"contents": map[string]interface{}{
				"kind":  "markdown",
				"value": value,
			},
		},
	}
}

func (s *SimpleServer) handleCompletion(id interface{}) *Message {
	keywords := []string{"MATCH", "OPTIONAL", "MERGE", "UNWIND", "WHERE", "RETURN", "SET", "REMOVE", "SKIP", "LIMIT", "AS"}
	functions := []string{"count", "collect", "coalesce", "sum", "avg", "min", "max"}

	items := make([]map[string]interface{}, 0, len(keywords)+len(functions))
	for _, keyword := range keywords {
		items = append(items, map[string]interface{}{
			"label":      keyword,
			"kind":       14, // Keyword
			"insertText": keyword,
		})
	}
	for _, fn := range functions {
		items = append(items, map[string]interface{}{
			"label":      fn,
			"kind":       3, // Function
			"insertText": fn,
		})
	}

	return &Message{
		JsonRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"isIncomplete": false,
			"items":        items,
		},
	}
}

func (s *SimpleServer) getURIAndText(params interface{}) (string, string) {
	m, ok := params.(map[string]interface{})
	if !ok {
		return "", ""
	}
	doc, ok := m["textDocument"].(map[string]interface{})
	if !ok {
		return "", ""
	}
	uri, _ := doc["uri"].(string)
	text := s.documents[uri]
	if text == "" {
		if t, ok := doc["text"].(string); ok {
			text = t
		}
	}
	return uri, text
}

func (s *SimpleServer) getHoverContext(params interface{}) (string, string, int, int) {
	m, ok := params.(map[string]interface{})
	if !ok {
		return "", "", 0, 0
	}
	doc, ok := m["textDocument"].(map[string]interface{})
	if !ok {
		return "", "", 0, 0
	}
	pos, ok := m["position"].(map[string]interface{})
	if !ok {
		return "", "", 0, 0
	}
	uri, _ := doc["uri"].(string)
	line, _ := pos["line"].(float64)
	character, _ := pos["character"].(float64)
	return uri, s.documents[uri], int(line), int(character)
}

func fullDocumentRange(text string) Range {
	lines := strings.Split(text, "\n")
	lastLine := len(lines) - 1
	if lastLine < 0 {
		lastLine = 0
		lines = []string{""}
	}
	lastChar := len(lines[lastLine])
	return Range{
		Start: Position{Line: 0, Character: 0},
		End:   Position{Line: lastLine, Character: lastChar},
	}
}

func wordAtPosition(text string, line, character int) string {
	lines := strings.Split(text, "\n")
	if line < 0 || line >= len(lines) {
		return ""
	}
	l := lines[line]
	if character < 0 {
		character = 0
	}
	if character > len(l) {
		character = len(l)
	}

	start := character
	for start > 0 && isWordRune(rune(l[start-1])) {
		start--
	}
	end := character
	for end < len(l) && isWordRune(rune(l[end])) {
		end++
	}
	return l[start:end]
}

func isWordRune(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') ||
		r == '_' || r == '$'
}

func errorResponse(id interface{}, code int, message string) *Message {
	return &Message{
		JsonRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
		},
	}
}
