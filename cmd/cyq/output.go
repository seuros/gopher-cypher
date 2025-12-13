package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/seuros/gopher-cypher/src/driver"
)

func writeTable(ctx context.Context, w io.Writer, keys []string, result driver.Result) (int64, error) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	defer func() { _ = tw.Flush() }()

	if len(keys) > 0 {
		_, _ = fmt.Fprintln(tw, strings.Join(keys, "\t"))
	}

	var rows int64
	for result.Next(ctx) {
		rec := result.Record()
		if rec == nil {
			continue
		}
		rows++

		line := make([]string, 0, len(keys))
		for _, key := range keys {
			line = append(line, stringifyValue((*rec)[key]))
		}
		_, _ = fmt.Fprintln(tw, strings.Join(line, "\t"))
	}

	if err := result.Err(); err != nil {
		return rows, err
	}
	return rows, nil
}

func writeJSONLines(ctx context.Context, w io.Writer, result driver.Result) (int64, error) {
	enc := json.NewEncoder(w)
	var rows int64
	for result.Next(ctx) {
		rec := result.Record()
		if rec == nil {
			continue
		}
		rows++
		if err := enc.Encode(*rec); err != nil {
			return rows, err
		}
	}
	if err := result.Err(); err != nil {
		return rows, err
	}
	return rows, nil
}

func writeJSONArray(ctx context.Context, w io.Writer, result driver.Result) (int64, error) {
	var rows int64
	first := true

	if _, err := io.WriteString(w, "["); err != nil {
		return 0, err
	}

	for result.Next(ctx) {
		rec := result.Record()
		if rec == nil {
			continue
		}
		rows++

		if !first {
			if _, err := io.WriteString(w, ","); err != nil {
				return rows, err
			}
		}
		first = false

		b, err := json.Marshal(*rec)
		if err != nil {
			return rows, err
		}
		if _, err := w.Write(b); err != nil {
			return rows, err
		}
	}

	if _, err := io.WriteString(w, "]\n"); err != nil {
		return rows, err
	}

	if err := result.Err(); err != nil {
		return rows, err
	}
	return rows, nil
}

func stringifyValue(v interface{}) string {
	if v == nil {
		return "null"
	}

	switch x := v.(type) {
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	default:
		b, err := json.Marshal(v)
		if err == nil {
			return string(b)
		}
		return fmt.Sprint(v)
	}
}
