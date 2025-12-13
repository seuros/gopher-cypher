// Package driver implements a lightweight Bolt protocol client used to
// communicate with Neo4j and Memgraph databases.
package driver

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	"github.com/seuros/gopher-cypher/src/connection_url_resolver"
	"github.com/yudhasubki/netpool"
)

// Driver defines the minimal functionality required to communicate with a
// Cypher-compatible database. Implementations must manage their own
// connections and provide simple query execution utilities.
type Driver interface {
	// Close releases all resources associated with the driver.
	Close() error
	// Ping verifies the server is reachable and the Bolt protocol is
	// supported.
	Ping() error
	// Run executes a Cypher query with context, optional parameters and metadata.
	// It returns the column names and resulting rows.
	Run(ctx context.Context, query string, params map[string]interface{}, metaData map[string]interface{}) ([]string, []map[string]interface{}, error)
	// RunWithContext executes a Cypher query with context support and returns detailed summary.
	// This is the recommended method for production use with observability.
	RunWithContext(ctx context.Context, query string, params map[string]interface{}, metaData map[string]interface{}) ([]string, []map[string]interface{}, *ResultSummary, error)
}

// StreamingDriver extends Driver with streaming query capabilities for memory-efficient
// processing of large result sets.
type StreamingDriver interface {
	Driver
	// RunStream executes a Cypher query and returns a streaming Result for lazy record processing.
	// This is memory-efficient for large result sets as records are fetched on-demand.
	RunStream(ctx context.Context, query string, params map[string]interface{}, metaData map[string]interface{}) (Result, error)
}

// driver implements the Driver interface using a pool of TCP connections.
type driver struct {
	urlResolver   *connection_url_resolver.ConnectionUrlResolver
	netPool       *netpool.Netpool
	config        *Config
	observability *observabilityInstruments
	logger        Logger
}

// NewDriver initializes a new Driver based on the provided connection URL.
// It validates the URL, establishes a connection pool and performs an initial
// ping to ensure the server is reachable.
func NewDriver(urlString string) (Driver, error) {
	return NewDriverWithConfig(urlString, nil)
}

// NewDriverWithConfig creates a new Driver with custom configuration options.
// If config is nil, default configuration is used.
func NewDriverWithConfig(urlString string, config *Config) (Driver, error) {
	if config == nil {
		config = DefaultConfig()
	}
	d := driver{
		config: config,
	}

	// Initialize logger
	if config.Logging != nil && config.Logging.Logger != nil {
		d.logger = config.Logging.Logger
	} else {
		d.logger = &NoOpLogger{}
	}

	d.logger.Info("Initializing gopher-cypher driver", "url", urlString)

	// Initialize observability
	if config.Observability != nil && (config.Observability.EnableTracing || config.Observability.EnableMetrics) {
		d.observability = initObservability()
		d.logger.Debug("Observability enabled", "tracing", config.Observability.EnableTracing, "metrics", config.Observability.EnableMetrics)
	}

	d.urlResolver = connection_url_resolver.NewConnectionUrlResolver(urlString)
	if d.urlResolver.ToHash() == nil {
		d.logger.Error("Failed to resolve connection URL", "url", urlString)
		return nil, fmt.Errorf("unable to resolve connection url: %s", urlString)
	}

	urlCfg := d.urlResolver.ToHash()
	d.logger.Debug("Connection URL resolved", "host", urlCfg.Host, "port", urlCfg.Port, "ssl", urlCfg.SSL, "database", urlCfg.Database)

	var err error
	dialFn := func() (net.Conn, error) {
		urlCfg := d.urlResolver.ToHash()
		address := d.urlResolver.Address()

		if config.Logging != nil && config.Logging.LogConnectionPool {
			d.logger.Debug("Opening connection", "address", address, "ssl", urlCfg.SSL, "ssc", urlCfg.SSC)
		}

		if urlCfg.SSL || urlCfg.SSC {
			// Build TLS config from driver configuration
			tlsCfg := config.TLS.buildTLSConfig(urlCfg.Host)

			// Override with URL-specific settings if needed
			if urlCfg.SSC {
				tlsCfg.InsecureSkipVerify = true
				d.logger.Warn("TLS certificate verification disabled (SSC mode)", "address", address)
			}

			d.logger.Debug("Establishing TLS connection", "address", address, "server_name", tlsCfg.ServerName)
			return tls.Dial("tcp", address, tlsCfg)
		}

		d.logger.Debug("Establishing plain TCP connection", "address", address)
		return net.Dial("tcp", address)
	}

	d.netPool, err = netpool.New(dialFn)
	if err != nil {
		d.logger.Error("Failed to create connection pool", "error", err)
		return nil, err
	}

	d.logger.Debug("Connection pool created successfully")

	err = d.Ping()
	if err != nil {
		d.logger.Error("Initial ping failed", "error", err)
		return nil, err
	}

	d.logger.Info("Driver initialized successfully", "address", d.urlResolver.Address())
	return &d, nil
}

// Close shuts down the driver's connection pool.
func (d *driver) Close() error {
	d.logger.Info("Closing driver")
	d.netPool.Close()
	d.logger.Debug("Connection pool closed")
	return nil
}
