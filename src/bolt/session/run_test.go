package session

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/seuros/gopher-cypher/src/connection_url_resolver"
)

func newSessionOrSkip(t *testing.T, url string) Session {
	t.Helper()
	addr := connection_url_resolver.NewConnectionUrlResolver(url).Address()
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Skip("database not available")
	}
	conn.Close()
	s, err := NewSession(url)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return s
}

func TestRunQuery(t *testing.T) {
	s := newSessionOrSkip(t, "memgraph://memgraph:activecypher@localhost:7688")
	defer s.Close()
	ctx := context.Background()
	cols, rows, err := s.Run(ctx, "RETURN 1 AS n", map[string]interface{}{}, map[string]interface{}{})
	if err != nil {
		t.Fatalf("%v", err)
	}
	if len(cols) != 1 || cols[0] != "n" {
		t.Fatalf("unexpected columns: %v", cols)
	}
	if len(rows) != 1 || rows[0]["n"] != int64(1) {
		t.Fatalf("unexpected result: %v", rows)
	}

	cols, rows, err = s.Run(ctx, "UNWIND range(1, 10) AS i RETURN i AS id, i * 2 AS double, 'Row ' + toString(i) AS label", map[string]interface{}{}, map[string]interface{}{})
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
