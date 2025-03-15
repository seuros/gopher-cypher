package driver

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"
)

// Config holds configuration options for the driver
type Config struct {
	// TLS holds TLS-specific configuration
	TLS *TLSConfig

	// ConnectionPool holds connection pool configuration
	ConnectionPool *PoolConfig

	// Observability holds telemetry configuration
	Observability *ObservabilityConfig

	// Logging holds logging configuration
	Logging *LoggingConfig
}

// TLSConfig provides advanced TLS configuration options
type TLSConfig struct {
	// Config allows passing a custom tls.Config directly
	// If provided, this takes precedence over other TLS settings
	Config *tls.Config

	// InsecureSkipVerify disables certificate verification (equivalent to +ssc)
	InsecureSkipVerify bool

	// ServerName specifies the expected server name for certificate validation
	// If empty, it's derived from the connection URL
	ServerName string

	// ClientCertificates holds client certificates for mutual TLS
	ClientCertificates []tls.Certificate

	// RootCAs specifies the root certificate authorities to trust
	// If nil, system root CAs are used
	RootCAs *x509.CertPool

	// ClientCAs specifies certificate authorities for client certificate validation
	ClientCAs *x509.CertPool

	// MinVersion specifies the minimum TLS version (default: TLS 1.2)
	MinVersion uint16

	// MaxVersion specifies the maximum TLS version (default: latest)
	MaxVersion uint16

	// CipherSuites specifies allowed cipher suites
	// If empty, Go's default secure cipher suites are used
	CipherSuites []uint16
}

// PoolConfig provides connection pool configuration options
type PoolConfig struct {
	// MaxConnections specifies the maximum number of connections in the pool
	// Default: 100 (matching Neo4j driver)
	MaxConnections int

	// MaxIdleTime specifies how long connections can be idle before being closed
	// Default: 30 minutes
	MaxIdleTime time.Duration

	// ConnectionLifetime specifies the maximum lifetime of a connection
	// Default: 1 hour (matching Neo4j driver)
	ConnectionLifetime time.Duration

	// AcquisitionTimeout specifies how long to wait for a connection from the pool
	// Default: 30 seconds
	AcquisitionTimeout time.Duration

	// EnableLivenessCheck enables periodic connection health checks
	// Default: true
	EnableLivenessCheck bool
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		TLS: &TLSConfig{
			MinVersion: tls.VersionTLS12,
			MaxVersion: 0, // Use Go's default (latest)
		},
		ConnectionPool: &PoolConfig{
			MaxConnections:      100,
			MaxIdleTime:         30 * time.Minute,
			ConnectionLifetime:  1 * time.Hour,
			AcquisitionTimeout:  30 * time.Second,
			EnableLivenessCheck: true,
		},
		Observability: DefaultObservabilityConfig(),
		Logging:       DefaultLoggingConfig(),
	}
}

// NewTLSConfigFromCertFiles creates a TLSConfig from certificate file paths
func NewTLSConfigFromCertFiles(certFile, keyFile, caFile string) (*TLSConfig, error) {
	tlsConfig := &TLSConfig{
		MinVersion: tls.VersionTLS12,
	}

	// Load client certificate if provided
	if certFile != "" && keyFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		tlsConfig.ClientCertificates = []tls.Certificate{cert}
	}

	// Load custom CA if provided
	if caFile != "" {
		caCertData, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file %s: %w", caFile, err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCertData) {
			return nil, fmt.Errorf("failed to parse CA certificate from %s", caFile)
		}
		tlsConfig.RootCAs = caCertPool
	}

	return tlsConfig, nil
}

// buildTLSConfig creates a *tls.Config from TLSConfig settings
func (tc *TLSConfig) buildTLSConfig(serverName string) *tls.Config {
	// If custom config provided, use it directly
	if tc.Config != nil {
		return tc.Config.Clone()
	}

	// Build config from individual settings
	config := &tls.Config{
		InsecureSkipVerify: tc.InsecureSkipVerify,
		ServerName:         tc.ServerName,
		Certificates:       tc.ClientCertificates,
		RootCAs:            tc.RootCAs,
		ClientCAs:          tc.ClientCAs,
		MinVersion:         tc.MinVersion,
		MaxVersion:         tc.MaxVersion,
		CipherSuites:       tc.CipherSuites,
	}

	// Use provided server name if none specified in config
	if config.ServerName == "" {
		config.ServerName = serverName
	}

	return config
}
