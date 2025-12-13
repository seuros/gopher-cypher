package driver

import (
	"crypto/tls"
	"crypto/x509"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	// Test TLS defaults
	if config.TLS == nil {
		t.Fatal("TLS config should not be nil")
	}
	if config.TLS.MinVersion != tls.VersionTLS12 {
		t.Errorf("Expected TLS 1.2 minimum, got %d", config.TLS.MinVersion)
	}

	// Test pool defaults
	if config.ConnectionPool == nil {
		t.Fatal("ConnectionPool config should not be nil")
	}
	if config.ConnectionPool.MaxConnections != 100 {
		t.Errorf("Expected 100 max connections, got %d", config.ConnectionPool.MaxConnections)
	}
	if config.ConnectionPool.MaxIdleTime != 30*time.Minute {
		t.Errorf("Expected 30 minute idle time, got %v", config.ConnectionPool.MaxIdleTime)
	}
	if config.ConnectionPool.ConnectionLifetime != 1*time.Hour {
		t.Errorf("Expected 1 hour lifetime, got %v", config.ConnectionPool.ConnectionLifetime)
	}
}

func TestTLSConfigBuild(t *testing.T) {
	tests := []struct {
		name       string
		tlsConfig  *TLSConfig
		serverName string
		validate   func(*testing.T, *tls.Config)
	}{
		{
			name: "basic TLS config",
			tlsConfig: &TLSConfig{
				MinVersion: tls.VersionTLS12,
				MaxVersion: tls.VersionTLS13,
			},
			serverName: "localhost",
			validate: func(t *testing.T, cfg *tls.Config) {
				if cfg.MinVersion != tls.VersionTLS12 {
					t.Errorf("Expected TLS 1.2, got %d", cfg.MinVersion)
				}
				if cfg.MaxVersion != tls.VersionTLS13 {
					t.Errorf("Expected TLS 1.3, got %d", cfg.MaxVersion)
				}
				if cfg.ServerName != "localhost" {
					t.Errorf("Expected localhost, got %s", cfg.ServerName)
				}
			},
		},
		{
			name: "insecure skip verify",
			tlsConfig: &TLSConfig{
				InsecureSkipVerify: true,
			},
			serverName: "test.example.com",
			validate: func(t *testing.T, cfg *tls.Config) {
				if !cfg.InsecureSkipVerify {
					t.Error("Expected InsecureSkipVerify to be true")
				}
			},
		},
		{
			name: "custom server name",
			tlsConfig: &TLSConfig{
				ServerName: "custom.example.com",
			},
			serverName: "localhost",
			validate: func(t *testing.T, cfg *tls.Config) {
				if cfg.ServerName != "custom.example.com" {
					t.Errorf("Expected custom.example.com, got %s", cfg.ServerName)
				}
			},
		},
		{
			name: "custom tls.Config takes precedence",
			tlsConfig: &TLSConfig{
				Config: &tls.Config{
					MinVersion: tls.VersionTLS13,
					ServerName: "override.example.com",
				},
				MinVersion: tls.VersionTLS12, // This should be ignored
			},
			serverName: "localhost",
			validate: func(t *testing.T, cfg *tls.Config) {
				if cfg.MinVersion != tls.VersionTLS13 {
					t.Errorf("Expected custom config TLS 1.3, got %d", cfg.MinVersion)
				}
				if cfg.ServerName != "override.example.com" {
					t.Errorf("Expected custom server name, got %s", cfg.ServerName)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.tlsConfig.buildTLSConfig(tt.serverName)
			tt.validate(t, cfg)
		})
	}
}

func TestTLSConfigWithCertificates(t *testing.T) {
	// Create a basic certificate for testing
	cert := tls.Certificate{
		// Empty cert for testing - in real usage this would be loaded from files
	}

	tlsConfig := &TLSConfig{
		ClientCertificates: []tls.Certificate{cert},
	}

	builtConfig := tlsConfig.buildTLSConfig("localhost")

	if len(builtConfig.Certificates) != 1 {
		t.Errorf("Expected 1 certificate, got %d", len(builtConfig.Certificates))
	}
}

func TestTLSConfigWithCustomRootCAs(t *testing.T) {
	// Create a custom certificate pool
	pool := x509.NewCertPool()

	tlsConfig := &TLSConfig{
		RootCAs: pool,
	}

	builtConfig := tlsConfig.buildTLSConfig("localhost")

	if builtConfig.RootCAs != pool {
		t.Error("Expected custom root CA pool to be used")
	}
}

func TestNewDriverWithConfig(t *testing.T) {
	// Test with nil config (should use defaults)
	_, err := NewDriverWithConfig("memgraph://test:test@localhost:7688", nil)
	// We expect this to fail since there's no server, but we're testing config handling
	if err == nil {
		t.Log("Driver creation succeeded (server must be running)")
	} else {
		t.Logf("Expected connection error: %v", err)
	}

	// Test with custom config
	config := &Config{
		TLS: &TLSConfig{
			MinVersion:         tls.VersionTLS13,
			InsecureSkipVerify: true,
		},
		ConnectionPool: &PoolConfig{
			MaxConnections: 50,
			MaxIdleTime:    15 * time.Minute,
		},
	}

	_, err = NewDriverWithConfig("memgraph://test:test@localhost:7688", config)
	if err == nil {
		t.Log("Driver creation with custom config succeeded")
	} else {
		t.Logf("Expected connection error with custom config: %v", err)
	}
}
