package main

import (
	"fmt"
	"log"
	"os"

	"github.com/seuros/gopher-cypher/src/driver"
	"github.com/seuros/gopher-cypher/src/lsp"
	"github.com/seuros/gopher-cypher/src/parser"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "lint":
		lintCommand(args)
	case "fmt":
		fmtCommand(args)
	case "inspect":
		inspectCommand(args)
	case "lsp":
		lspCommand(args)
	case "version", "--version", "-v":
		versionCommand()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("cyq - Cypher query tool")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  cyq lint <file>     - Validate Cypher syntax")
	fmt.Println("  cyq fmt <file>      - Format Cypher query")
	fmt.Println("  cyq inspect <file>  - Inspect AST structure")
	fmt.Println("  cyq lsp             - Start Language Server")
	fmt.Println("  cyq version         - Show version information")
}

func versionCommand() {
	fmt.Printf("cyq version %s\n", driver.Version())
	fmt.Printf("User agent: %s\n", driver.UserAgent())
}

func lintCommand(args []string) {
	if len(args) != 1 {
		fmt.Println("Usage: cyq lint <file>")
		os.Exit(1)
	}

	filename := args[0]
	content, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}

	p, err := parser.New()
	if err != nil {
		log.Fatalf("Failed to create parser: %v", err)
	}

	_, err = p.Parse(string(content))
	if err != nil {
		fmt.Printf("Syntax error in %s: %v\n", filename, err)
		os.Exit(1)
	}

	fmt.Printf("%s: OK\n", filename)
}

func fmtCommand(args []string) {
	if len(args) != 1 {
		fmt.Println("Usage: cyq fmt <file>")
		os.Exit(1)
	}

	filename := args[0]
	content, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}

	p, err := parser.New()
	if err != nil {
		log.Fatalf("Failed to create parser: %v", err)
	}

	query, err := p.Parse(string(content))
	if err != nil {
		log.Fatalf("Parse error: %v", err)
	}

	formatted, _ := query.BuildCypher()
	fmt.Print(formatted)
}

func inspectCommand(args []string) {
	if len(args) != 1 {
		fmt.Println("Usage: cyq inspect <file>")
		os.Exit(1)
	}

	filename := args[0]
	content, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}

	p, err := parser.New()
	if err != nil {
		log.Fatalf("Failed to create parser: %v", err)
	}

	query, err := p.Parse(string(content))
	if err != nil {
		log.Fatalf("Parse error: %v", err)
	}

	fmt.Printf("Query structure for %s:\n", filename)
	cypher, params := query.BuildCypher()
	fmt.Printf("Generated Cypher: %s\n", cypher)
	fmt.Printf("Parameters: %v\n", params)
}

func lspCommand(args []string) {
	if len(args) != 0 {
		fmt.Println("Usage: cyq lsp")
		os.Exit(1)
	}

	if err := lsp.StartServer(); err != nil {
		log.Fatalf("LSP server error: %v", err)
	}
}
