package driver

import (
	"crypto/tls"
	"net"
	"testing"
	"time"

	"github.com/seuros/gopher-cypher/src/connection_url_resolver"
	"github.com/seuros/gopher-cypher/src/internal/testutil"
)

func newDriverOrSkip(t *testing.T, url string) Driver {
	t.Helper()
	addr := connection_url_resolver.NewConnectionUrlResolver(url).Address()
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Skip("database not available")
	}
	conn.Close()
	dr, err := NewDriver(url)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return dr
}

func newDriverOrSkipTLS(t *testing.T, url string) Driver {
	t.Helper()
	resolver := connection_url_resolver.NewConnectionUrlResolver(url)
	addr := resolver.Address()
	cfg := resolver.ToHash()
	tlsCfg := &tls.Config{}
	if cfg.SSC {
		tlsCfg.InsecureSkipVerify = true
	}
	conn, err := tls.Dial("tcp", addr, tlsCfg)
	if err != nil {
		t.Skip("database not available or tls handshake failed")
	}
	conn.Close()
	dr, err := NewDriver(url)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return dr
}

func TestPingMemgraph(t *testing.T) {
	dr := newDriverOrSkip(t, testutil.MemgraphURL())
	defer dr.Close()

	if err := dr.Ping(); err != nil {
		t.Fatalf("%v", err)
	}
}

func TestPingNeo4j(t *testing.T) {
	dr := newDriverOrSkip(t, testutil.Neo4jURL())
	defer dr.Close()

	if err := dr.Ping(); err != nil {
		t.Fatalf("%v", err)
	}
}

func TestPingNeo4jSSL(t *testing.T) {
	dr := newDriverOrSkipTLS(t, testutil.Neo4jSSLURL())
	defer dr.Close()

	if err := dr.Ping(); err != nil {
		t.Fatalf("%v", err)
	}
}

func TestPingNeo4jSSC(t *testing.T) {
	dr := newDriverOrSkipTLS(t, testutil.Neo4jSSCURL())
	defer dr.Close()

	if err := dr.Ping(); err != nil {
		t.Fatalf("%v", err)
	}
}
