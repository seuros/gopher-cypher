package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/seuros/gopher-cypher/src/driver"
)

func runCommand(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	urlFlag := fs.String("url", os.Getenv("CYQ_URL"), "Connection URL (or set CYQ_URL)")
	queryFlag := fs.String("query", "", "Query string (if no file is provided)")
	paramsFlag := fs.String("params", "", "Params as JSON object (e.g. '{\"n\": 1}')")
	paramsFileFlag := fs.String("params-file", "", "Path to JSON file containing params")
	formatFlag := fs.String("format", "table", "Output format: table|json|jsonl")
	timeoutFlag := fs.Duration("timeout", 0, "Optional context timeout (e.g. 10s, 1m). 0 disables.")
	noSummaryFlag := fs.Bool("no-summary", false, "Do not print summary to stderr")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return &exitError{code: 0}
		}
		return usageErrorf(2, "%v", err)
	}

	if *urlFlag == "" {
		return usageErrorf(2, "Missing --url (or set CYQ_URL)")
	}

	query, err := resolveQuery(*queryFlag, fs.Args())
	if err != nil {
		return err
	}

	params, err := resolveParams(*paramsFlag, *paramsFileFlag)
	if err != nil {
		return err
	}

	ctx := context.Background()
	if *timeoutFlag > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeoutFlag)
		defer cancel()
	}

	dr, err := driver.NewDriver(*urlFlag)
	if err != nil {
		return err
	}
	defer func() { _ = dr.Close() }()

	streaming, ok := dr.(driver.StreamingDriver)
	if !ok {
		return fmt.Errorf("driver does not support streaming")
	}

	result, err := streaming.RunStream(ctx, query, params, nil)
	if err != nil {
		return err
	}

	keys, err := result.Keys()
	if err != nil {
		return err
	}

	var rows int64
	switch strings.ToLower(*formatFlag) {
	case "table":
		rows, err = writeTable(ctx, os.Stdout, keys, result)
	case "json":
		rows, err = writeJSONArray(ctx, os.Stdout, result)
	case "jsonl":
		rows, err = writeJSONLines(ctx, os.Stdout, result)
	default:
		return usageErrorf(2, "Unknown --format %q (expected table|json|jsonl)", *formatFlag)
	}
	if err != nil {
		_, _ = result.Consume(ctx)
		return err
	}

	summary, consumeErr := result.Consume(ctx)
	if consumeErr != nil {
		return consumeErr
	}

	if !*noSummaryFlag && summary != nil {
		fmt.Fprintf(os.Stderr, "rows=%d time=%s\n", rows, summary.ExecutionTime.Truncate(time.Microsecond))
	}

	return nil
}

func resolveQuery(queryFlag string, remainingArgs []string) (string, error) {
	if queryFlag != "" {
		if len(remainingArgs) != 0 {
			return "", usageErrorf(2, "Provide either --query or a file path, not both")
		}
		return normalizeQuery(queryFlag), nil
	}

	if len(remainingArgs) > 1 {
		return "", usageErrorf(2, "Usage: cyq run [flags] [file|-]")
	}

	filename := "-"
	if len(remainingArgs) == 1 {
		filename = remainingArgs[0]
	}

	var content []byte
	var err error
	if filename == "-" {
		content, err = io.ReadAll(os.Stdin)
	} else {
		content, err = os.ReadFile(filename)
	}
	if err != nil {
		return "", err
	}

	query := normalizeQuery(string(content))
	if query == "" {
		return "", usageErrorf(2, "Query is empty")
	}
	return query, nil
}

func normalizeQuery(query string) string {
	q := strings.TrimSpace(query)
	q = strings.TrimSuffix(q, ";")
	return strings.TrimSpace(q)
}

func resolveParams(paramsFlag string, paramsFile string) (map[string]interface{}, error) {
	if paramsFlag != "" && paramsFile != "" {
		return nil, usageErrorf(2, "Provide either --params or --params-file, not both")
	}

	if paramsFlag == "" && paramsFile == "" {
		return map[string]interface{}{}, nil
	}

	var data []byte
	if paramsFile != "" {
		b, err := os.ReadFile(paramsFile)
		if err != nil {
			return nil, err
		}
		data = b
	} else {
		data = []byte(paramsFlag)
	}

	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.UseNumber()
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return nil, usageErrorf(2, "Invalid params JSON: %v", err)
	}

	params, ok := normalizeJSONNumbers(v).(map[string]interface{})
	if !ok {
		return nil, usageErrorf(2, "Params must be a JSON object")
	}
	return params, nil
}

func normalizeJSONNumbers(v interface{}) interface{} {
	switch x := v.(type) {
	case map[string]interface{}:
		for k, vv := range x {
			x[k] = normalizeJSONNumbers(vv)
		}
		return x
	case []interface{}:
		for i, vv := range x {
			x[i] = normalizeJSONNumbers(vv)
		}
		return x
	case json.Number:
		s := x.String()
		if !strings.ContainsAny(s, ".eE") {
			if i, err := x.Int64(); err == nil {
				return i
			}
		}
		if f, err := x.Float64(); err == nil {
			return f
		}
		return s
	default:
		return v
	}
}
