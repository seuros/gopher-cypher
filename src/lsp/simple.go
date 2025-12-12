package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

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
	TextDocumentSync   int                `json:"textDocumentSync"`
	HoverProvider      bool               `json:"hoverProvider"`
	CompletionProvider *CompletionOptions `json:"completionProvider"`
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
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Content-Length:") {
			lengthStr := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			length, err := strconv.Atoi(lengthStr)
			if err != nil {
				continue
			}

			// Skip empty line
			scanner.Scan()

			// Read message content
			content := make([]byte, length)
			if _, err = io.ReadFull(os.Stdin, content); err != nil {
				continue
			}

			var msg Message
			if err := json.Unmarshal(content, &msg); err != nil {
				continue
			}

			response := server.handleMessage(&msg)
			if response != nil {
				server.sendResponse(response)
			}
		}
	}

	return nil
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
					TextDocumentSync: 1,
					HoverProvider:    true,
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
		return &Message{
			JsonRPC: "2.0",
			ID:      msg.ID,
			Result: map[string]interface{}{
				"contents": map[string]interface{}{
					"kind":  "markdown",
					"value": "**Cypher Element**\n\nHover information for Cypher syntax.",
				},
			},
		}
	case "textDocument/completion":
		keywords := []string{"MATCH", "WHERE", "RETURN", "LIMIT", "SKIP", "CREATE", "MERGE", "SET", "REMOVE"}
		items := make([]map[string]interface{}, len(keywords))
		for i, keyword := range keywords {
			items[i] = map[string]interface{}{
				"label":      keyword,
				"kind":       14, // Keyword
				"insertText": keyword,
			}
		}
		return &Message{
			JsonRPC: "2.0",
			ID:      msg.ID,
			Result: map[string]interface{}{
				"isIncomplete": false,
				"items":        items,
			},
		}
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
		// We don't currently extract precise locations from parser errors.
		// Emit a file-level diagnostic.
		diags = append(diags, Diagnostic{
			Range: Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: 0, Character: 1},
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
