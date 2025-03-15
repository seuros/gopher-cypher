package session

import (
	"net"

	"github.com/seuros/gopher-cypher/src/connection_url_resolver"
	"github.com/seuros/gopher-cypher/src/internal/boltutil"
)

func checkVersion(conn net.Conn) error {
	return boltutil.CheckVersion(conn)
}

func sendHello(conn net.Conn) error {
	return boltutil.SendHello(conn)
}

func authenticate(conn net.Conn, urlResolver *connection_url_resolver.ConnectionUrlResolver) error {
	return boltutil.Authenticate(conn, urlResolver)
}
