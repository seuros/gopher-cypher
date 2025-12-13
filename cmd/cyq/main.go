package main

import (
	"errors"
	"fmt"
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

	var err error
	switch command {
	case "lint":
		err = lintCommand(args)
	case "fmt":
		err = fmtCommand(args)
	case "inspect":
		err = inspectCommand(args)
	case "run":
		err = runCommand(args)
	case "ping":
		err = pingCommand(args)
	case "lsp":
		err = lspCommand(args)
	case "version", "--version", "-v":
		err = versionCommand()
	case "help", "--help", "-h":
		printUsage()
		return
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		var exitErr *exitError
		if errors.As(err, &exitErr) {
			if exitErr.Error() != "" {
				fmt.Fprintln(os.Stderr, exitErr.Error())
			}
			os.Exit(exitErr.code)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("cyq - Cypher query tool")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  cyq lint <file>                - Validate Cypher syntax")
	fmt.Println("  cyq fmt <file>                 - Format Cypher query")
	fmt.Println("  cyq inspect <file>             - Inspect AST structure")
	fmt.Println("  cyq run [flags] [file|-]       - Execute a query against a database")
	fmt.Println("  cyq ping [flags]               - Test database connectivity")
	fmt.Println("  cyq lsp                        - Start Language Server")
	fmt.Println("  cyq version                    - Show version information")
	fmt.Println()
	fmt.Println("Run flags:")
	fmt.Println("  --url <url>                    - Connection URL (or set CYQ_URL)")
	fmt.Println("  --params <json>                - Params as JSON object (e.g. '{\"n\": 1}')")
	fmt.Println("  --params-file <path>           - Params from JSON file")
	fmt.Println("  --format table|json|jsonl      - Output format (default: table)")
	fmt.Println("  --timeout 10s                  - Optional context timeout (default: none)")
}

func versionCommand() error {
	fmt.Printf("cyq version %s\n", driver.Version())
	fmt.Printf("User agent: %s\n", driver.UserAgent())
	return nil
}

func lintCommand(args []string) error {
	if len(args) != 1 {
		return usageErrorf(2, "Usage: cyq lint <file>")
	}

	filename := args[0]
	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	p, err := parser.New()
	if err != nil {
		return err
	}

	_, err = p.Parse(string(content))
	if err != nil {
		return usageErrorf(1, "Syntax error in %s: %v", filename, err)
	}

	fmt.Printf("%s: OK\n", filename)
	return nil
}

func fmtCommand(args []string) error {
	if len(args) != 1 {
		return usageErrorf(2, "Usage: cyq fmt <file>")
	}

	filename := args[0]
	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	p, err := parser.New()
	if err != nil {
		return err
	}

	query, err := p.Parse(string(content))
	if err != nil {
		return err
	}

	formatted, _ := query.BuildCypher()
	fmt.Print(formatted)
	return nil
}

func inspectCommand(args []string) error {
	if len(args) != 1 {
		return usageErrorf(2, "Usage: cyq inspect <file>")
	}

	filename := args[0]
	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	p, err := parser.New()
	if err != nil {
		return err
	}

	query, err := p.Parse(string(content))
	if err != nil {
		return err
	}

	fmt.Printf("Query structure for %s:\n", filename)
	cypher, params := query.BuildCypher()
	fmt.Printf("Generated Cypher: %s\n", cypher)
	fmt.Printf("Parameters: %v\n", params)
	return nil
}

func lspCommand(args []string) error {
	if len(args) != 0 {
		return usageErrorf(2, "Usage: cyq lsp")
	}

	if err := lsp.StartServer(); err != nil {
		return err
	}

	return nil
}
