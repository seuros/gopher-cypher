package driver

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/seuros/gopher-cypher/src/connection_url_resolver"
)

func dialOrSkip(t *testing.T, url string) {
	t.Helper()
	addr := connection_url_resolver.NewConnectionUrlResolver(url).Address()
	if _, err := net.DialTimeout("tcp", addr, time.Second); err != nil {
		t.Skip("database not available")
	}
}

func TestRunWithWrongAuth(t *testing.T) {
	dialOrSkip(t, "neo4j://memgraph:wrongpass@localhost:7687")
	_, err := NewDriver("neo4j://memgraph:wrongpass@localhost:7687")
	if err == nil {
		t.Fatal("expected authentication error, got nil")
	}
	t.Logf("got expected error: %v", err)
}
func TestRunQuery(t *testing.T) {
	dialOrSkip(t, "memgraph://memgraph:activecypher@localhost:7688")
	dr, err := NewDriver("memgraph://memgraph:activecypher@localhost:7688")
	if err != nil {
		t.Fatalf("%v", err)
	}
	defer dr.Close()

	ctx := context.Background()
	cols, rows, err := dr.Run(ctx, "RETURN 1 AS n", map[string]interface{}{}, map[string]interface{}{})
	if err != nil {
		t.Fatalf("%v", err)
	}
	if len(cols) != 1 || cols[0] != "n" {
		t.Fatalf("unexpected columns: %v", cols)
	}
	if len(rows) != 1 || rows[0]["n"] != int64(1) {
		t.Fatalf("unexpected result: %v", rows)
	}

	cols, rows, err = dr.Run(ctx, "UNWIND range(1, 10) AS i RETURN i AS id, i * 2 AS double, 'Row ' + toString(i) AS label", map[string]interface{}{}, map[string]interface{}{})
	if err != nil {
		t.Fatalf("%v", err)
	}
	if len(cols) != 3 || cols[0] != "id" || cols[1] != "double" || cols[2] != "label" {
		t.Fatalf("unexpected columns: %v", cols)
	}
	if len(rows) == 0 {
		t.Fatalf("unexpected result: %v", rows)
	}

}
