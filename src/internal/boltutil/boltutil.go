package boltutil

import (
	"errors"
	"fmt"
	"io"
	"net"
	"runtime"
	"time"

	"github.com/seuros/gopher-cypher/src/bolt/messaging"
	"github.com/seuros/gopher-cypher/src/connection_url_resolver"
)

// DefaultTimeout is the default timeout for Bolt protocol operations
const DefaultTimeout = 30 * time.Second

// LibraryVersion is injected at build time via -ldflags
var LibraryVersion = "dev"

// getLibraryVersion returns the current library version
func getLibraryVersion() string {
	return LibraryVersion
}

// CheckVersion negotiates the Bolt protocol version with the server and
// validates the returned version. Returns the negotiated major and minor
// version numbers on success.
func CheckVersion(conn net.Conn) (major, minor byte, err error) {
	magic := []byte{
		0x60, 0x60, 0xB0, 0x17,
		0, 0, 8, 5,
		0, 0, 2, 5,
		0, 0, 0, 0,
		0, 0, 0, 0,
	}

	// Set deadline for handshake
	if err = conn.SetDeadline(time.Now().Add(DefaultTimeout)); err != nil {
		err = fmt.Errorf("failed to set deadline: %w", err)
		return
	}
	defer conn.SetDeadline(time.Time{})

	if _, err = conn.Write(magic); err != nil {
		return
	}

	buf := make([]byte, 4)
	if _, err = io.ReadFull(conn, buf); err != nil {
		return
	}

	major = buf[3]
	minor = buf[2]
	if major == 80 && minor == 84 {
		err = errors.New("The server responded with an HTTP response. Please ensure you're not trying to connect to the HTTP endpoint. Note that HTTP typically uses port 7474, while the BOLT protocol uses port 7687.")
		return
	}

	if major != 5 {
		err = fmt.Errorf("Unsupported protocol version %d,%d", major, minor)
		return
	}
	if minor != 8 && minor != 2 {
		err = fmt.Errorf("Unsupported protocol version %d,%d", major, minor)
		return
	}

	return major, minor, nil
}

// SendHello performs the HELLO handshake with the server.
func SendHello(conn net.Conn) error {
	version := getLibraryVersion()
	userAgent := fmt.Sprintf("gopher-cypher::Bolt/%s (Go/%s)", version, runtime.Version()[2:]) // Remove "go" prefix
	platform := fmt.Sprintf("go %s [%s-%s]", runtime.Version()[2:], runtime.GOARCH, runtime.GOOS)

	message := messaging.NewHello(map[string]interface{}{
		"user_agent":                     userAgent,
		"notifications_minimum_severity": "WARNING",
		"bolt_agent": map[string]interface{}{
			"product":          userAgent,
			"platform":         platform,
			"language":         fmt.Sprintf("%s/%s", runtime.GOOS, runtime.Version()),
			"language_details": fmt.Sprintf("%s %s", runtime.Compiler, runtime.Version()),
		},
	})

	_, err := message.Send(conn)
	return err
}

// Authenticate sends logon credentials to the server and checks for failure.
func Authenticate(conn net.Conn, urlResolver *connection_url_resolver.ConnectionUrlResolver) error {
	messageLogon := messaging.NewLogon(map[string]interface{}{
		"scheme":      "basic",
		"principal":   urlResolver.ToHash().Username,
		"credentials": urlResolver.ToHash().Password,
	})

	response, err := messageLogon.Send(conn)
	if err != nil {
		return err
	}

	if messageFail, isFail := response.(*messaging.Failure); isFail {
		return errors.New(messageFail.Message())
	}

	return nil
}
