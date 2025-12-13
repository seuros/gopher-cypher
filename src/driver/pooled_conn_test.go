package driver

import (
	"net"
	"testing"
	"time"
)

// mockConn implements net.Conn for testing
type mockConn struct {
	closed      bool
	readErr     error
	writeErr    error
	readTimeout bool
	deadlineErr error
	localAddr   net.Addr
	remoteAddr  net.Addr
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	if m.readTimeout {
		return 0, &net.OpError{Op: "read", Err: &timeoutError{}}
	}
	return 0, m.readErr
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	return len(b), m.writeErr
}

func (m *mockConn) Close() error {
	m.closed = true
	return nil
}

func (m *mockConn) LocalAddr() net.Addr {
	if m.localAddr != nil {
		return m.localAddr
	}
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
}

func (m *mockConn) RemoteAddr() net.Addr {
	if m.remoteAddr != nil {
		return m.remoteAddr
	}
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7687}
}

func (m *mockConn) SetDeadline(t time.Time) error {
	return m.deadlineErr
}

func (m *mockConn) SetReadDeadline(t time.Time) error {
	return m.deadlineErr
}

func (m *mockConn) SetWriteDeadline(t time.Time) error {
	return m.deadlineErr
}

// timeoutError implements net.Error with Timeout() = true
type timeoutError struct{}

func (e *timeoutError) Error() string   { return "i/o timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }

func TestNewPooledConn(t *testing.T) {
	mock := &mockConn{}
	pc := newPooledConn(mock)

	if pc == nil {
		t.Fatal("newPooledConn returned nil")
	}

	if pc.Conn != mock {
		t.Error("underlying connection not set correctly")
	}

	if pc.authenticated {
		t.Error("new connection should not be authenticated")
	}

	if pc.createdAt.IsZero() {
		t.Error("createdAt should be set")
	}
}

func TestPooledConnIsAlive_Timeout(t *testing.T) {
	// Timeout means connection is alive (no data pending)
	mock := &mockConn{readTimeout: true}
	pc := newPooledConn(mock)

	if !pc.isAlive() {
		t.Error("connection with read timeout should be considered alive")
	}
}

func TestPooledConnIsAlive_EOF(t *testing.T) {
	// EOF means connection is dead
	mock := &mockConn{readErr: net.ErrClosed}
	pc := newPooledConn(mock)

	if pc.isAlive() {
		t.Error("connection with EOF should be considered dead")
	}
}

func TestPooledConnIsAlive_DeadlineError(t *testing.T) {
	// Can't set deadline = connection is broken
	mock := &mockConn{deadlineErr: net.ErrClosed}
	pc := newPooledConn(mock)

	if pc.isAlive() {
		t.Error("connection with deadline error should be considered dead")
	}
}

func TestPooledConnMarkAuthenticated(t *testing.T) {
	mock := &mockConn{}
	pc := newPooledConn(mock)

	pc.markAuthenticated(5, 8)

	if !pc.authenticated {
		t.Error("should be marked as authenticated")
	}

	if pc.boltVersion[0] != 5 || pc.boltVersion[1] != 8 {
		t.Errorf("bolt version should be 5.8, got %d.%d", pc.boltVersion[0], pc.boltVersion[1])
	}

	if pc.lastUsedAt.IsZero() {
		t.Error("lastUsedAt should be set after authentication")
	}
}

func TestPooledConnNeedsReauth_NotAuthenticated(t *testing.T) {
	mock := &mockConn{}
	pc := newPooledConn(mock)

	if !pc.needsReauth(30 * time.Minute) {
		t.Error("unauthenticated connection should need reauth")
	}
}

func TestPooledConnNeedsReauth_Authenticated(t *testing.T) {
	mock := &mockConn{}
	pc := newPooledConn(mock)
	pc.markAuthenticated(5, 8)

	if pc.needsReauth(30 * time.Minute) {
		t.Error("freshly authenticated connection should not need reauth")
	}
}

func TestPooledConnNeedsReauth_IdleTooLong(t *testing.T) {
	mock := &mockConn{}
	pc := newPooledConn(mock)
	pc.markAuthenticated(5, 8)

	// Manually set lastUsedAt to the past
	pc.mu.Lock()
	pc.lastUsedAt = time.Now().Add(-1 * time.Hour)
	pc.mu.Unlock()

	if !pc.needsReauth(30 * time.Minute) {
		t.Error("connection idle > maxIdleTime should need reauth")
	}
}

func TestPooledConnNeedsReauth_ZeroMaxIdle(t *testing.T) {
	mock := &mockConn{}
	pc := newPooledConn(mock)
	pc.markAuthenticated(5, 8)

	// Set lastUsedAt to the past
	pc.mu.Lock()
	pc.lastUsedAt = time.Now().Add(-1 * time.Hour)
	pc.mu.Unlock()

	// With zero maxIdleTime, should never consider idle
	if pc.needsReauth(0) {
		t.Error("with zero maxIdleTime, authenticated connection should not need reauth")
	}
}

func TestPooledConnTouch(t *testing.T) {
	mock := &mockConn{}
	pc := newPooledConn(mock)
	pc.markAuthenticated(5, 8)

	// Set lastUsedAt to the past
	pc.mu.Lock()
	oldTime := time.Now().Add(-1 * time.Hour)
	pc.lastUsedAt = oldTime
	pc.mu.Unlock()

	pc.touch()

	pc.mu.RLock()
	newTime := pc.lastUsedAt
	pc.mu.RUnlock()

	if !newTime.After(oldTime) {
		t.Error("touch should update lastUsedAt to current time")
	}
}

func TestPooledConnAge(t *testing.T) {
	mock := &mockConn{}
	pc := newPooledConn(mock)

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	age := pc.age()
	if age < 10*time.Millisecond {
		t.Errorf("age should be at least 10ms, got %v", age)
	}
}

func TestPooledConnIdleTime_NeverUsed(t *testing.T) {
	mock := &mockConn{}
	pc := newPooledConn(mock)

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	idle := pc.idleTime()
	if idle < 10*time.Millisecond {
		t.Errorf("idle time should be at least 10ms for never-used conn, got %v", idle)
	}
}

func TestPooledConnIdleTime_AfterUse(t *testing.T) {
	mock := &mockConn{}
	pc := newPooledConn(mock)

	// Simulate use
	pc.touch()
	time.Sleep(10 * time.Millisecond)

	idle := pc.idleTime()
	if idle < 10*time.Millisecond {
		t.Errorf("idle time should be at least 10ms after use, got %v", idle)
	}
}

func TestPooledConnBoltVersion(t *testing.T) {
	mock := &mockConn{}
	pc := newPooledConn(mock)
	pc.markAuthenticated(5, 2)

	if pc.boltMajor() != 5 {
		t.Errorf("expected major version 5, got %d", pc.boltMajor())
	}

	if pc.boltMinor() != 2 {
		t.Errorf("expected minor version 2, got %d", pc.boltMinor())
	}
}

func TestPooledConnIsAuthenticated(t *testing.T) {
	mock := &mockConn{}
	pc := newPooledConn(mock)

	if pc.isAuthenticated() {
		t.Error("new connection should not be authenticated")
	}

	pc.markAuthenticated(5, 8)

	if !pc.isAuthenticated() {
		t.Error("connection should be authenticated after markAuthenticated")
	}
}
