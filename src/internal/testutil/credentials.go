// Package testutil provides shared utilities for integration tests
package testutil

import "os"

// Default test credentials - override via environment variables
var (
	// Neo4j credentials
	Neo4jUser     = getEnvOrDefault("NEO4J_USER", "neo4j")
	Neo4jPassword = getEnvOrDefault("NEO4J_PASSWORD", "activecypher")
	Neo4jHost     = getEnvOrDefault("NEO4J_HOST", "localhost")
	Neo4jPort     = getEnvOrDefault("NEO4J_PORT", "7687")

	// Memgraph credentials
	MemgraphUser     = getEnvOrDefault("MEMGRAPH_USER", "memgraph")
	MemgraphPassword = getEnvOrDefault("MEMGRAPH_PASSWORD", "activecypher")
	MemgraphHost     = getEnvOrDefault("MEMGRAPH_HOST", "localhost")
	MemgraphPort     = getEnvOrDefault("MEMGRAPH_PORT", "7688")
)

func getEnvOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// Neo4jURL returns the Neo4j connection URL for tests
func Neo4jURL() string {
	return "neo4j://" + Neo4jUser + ":" + Neo4jPassword + "@" + Neo4jHost + ":" + Neo4jPort
}

// Neo4jSSLURL returns the Neo4j SSL connection URL for tests
func Neo4jSSLURL() string {
	return "neo4j+ssl://" + Neo4jUser + ":" + Neo4jPassword + "@" + Neo4jHost + ":" + Neo4jPort
}

// Neo4jSSCURL returns the Neo4j self-signed certificate connection URL for tests
func Neo4jSSCURL() string {
	return "neo4j+ssc://" + Neo4jUser + ":" + Neo4jPassword + "@" + Neo4jHost + ":" + Neo4jPort
}

// MemgraphURL returns the Memgraph connection URL for tests
func MemgraphURL() string {
	return "memgraph://" + MemgraphUser + ":" + MemgraphPassword + "@" + MemgraphHost + ":" + MemgraphPort
}

// InvalidCredentialsURL returns a URL with wrong credentials for negative tests
func InvalidCredentialsURL() string {
	return "neo4j://memgraph:wrongpass@" + MemgraphHost + ":" + MemgraphPort
}
