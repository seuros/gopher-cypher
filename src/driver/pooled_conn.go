package driver

import (
	"net"
	"sync"
	"time"
)

// pooledConn wraps a net.Conn with connection state tracking for efficient
// pool management. It tracks authentication status to avoid redundant
// handshakes and provides liveness checking to detect dead connections.
type pooledConn struct {
	net.Conn
	mu            sync.RWMutex
	authenticated bool
	boltVersion   [2]byte // [major, minor]
	createdAt     time.Time
	lastUsedAt    time.Time
}

// newPooledConn wraps a raw connection with state tracking.
func newPooledConn(conn net.Conn) *pooledConn {
	now := time.Now()
	return &pooledConn{
		Conn:      conn,
		createdAt: now,
	}
}

// isAlive checks if the connection is still responsive by attempting
// a non-blocking read with a very short deadline. A timeout indicates
// the connection is alive (no data pending), while EOF or other errors
// indicate a dead connection.
func (pc *pooledConn) isAlive() bool {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	// Set a very short read deadline to check for connection death
	if err := pc.SetReadDeadline(time.Now().Add(1 * time.Millisecond)); err != nil {
		return false
	}
	defer func() { _ = pc.SetReadDeadline(time.Time{}) }()

	// Try to read one byte - timeout means alive, EOF/error means dead
	one := make([]byte, 1)
	_, err := pc.Read(one)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return true // Timeout means connection is alive, just no data pending
		}
		return false // EOF, broken pipe, connection reset, etc.
	}
	// Got data unexpectedly - this shouldn't happen in normal operation
	// but the connection is technically alive
	return true
}

// markAuthenticated records successful Bolt authentication and version.
func (pc *pooledConn) markAuthenticated(major, minor byte) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.authenticated = true
	pc.boltVersion = [2]byte{major, minor}
	pc.lastUsedAt = time.Now()
}

// touch updates the last used timestamp.
func (pc *pooledConn) touch() {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.lastUsedAt = time.Now()
}

// needsReauth checks if the connection needs re-authentication.
// Returns true if:
// - Connection was never authenticated
// - Connection has been idle longer than maxIdleTime
func (pc *pooledConn) needsReauth(maxIdleTime time.Duration) bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	if !pc.authenticated {
		return true
	}
	if maxIdleTime > 0 && time.Since(pc.lastUsedAt) > maxIdleTime {
		return true
	}
	return false
}

// isAuthenticated returns whether this connection has been authenticated.
func (pc *pooledConn) isAuthenticated() bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	return pc.authenticated
}

// boltMajor returns the negotiated Bolt major version.
func (pc *pooledConn) boltMajor() byte {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	return pc.boltVersion[0]
}

// boltMinor returns the negotiated Bolt minor version.
func (pc *pooledConn) boltMinor() byte {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	return pc.boltVersion[1]
}

// age returns how long since the connection was created.
func (pc *pooledConn) age() time.Duration {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	return time.Since(pc.createdAt)
}

// idleTime returns how long since the connection was last used.
func (pc *pooledConn) idleTime() time.Duration {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	if pc.lastUsedAt.IsZero() {
		return time.Since(pc.createdAt)
	}
	return time.Since(pc.lastUsedAt)
}

// markDirty marks the connection as needing reset/re-auth after a failure.
// This ensures the connection won't be reused in a failed state.
func (pc *pooledConn) markDirty() {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.authenticated = false
}
