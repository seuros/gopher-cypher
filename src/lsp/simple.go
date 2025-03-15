package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/seuros/gopher-cypher/src/parser"
)

type SimpleServer struct {
	parser *parser.Parser
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

func StartSimpleServer() error {
	log.SetOutput(os.Stderr)
	log.Println("Starting simple Cypher LSP server...")

	p, err := parser.New()
	if err != nil {
		return err
	}

	server := &SimpleServer{parser: p}
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
			_, err = os.Stdin.Read(content)
			if err != nil {
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