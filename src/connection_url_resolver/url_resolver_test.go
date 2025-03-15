package connection_url_resolver

import (
	"reflect"
	"testing"
)

func TestNewConnectionUrlResolver(t *testing.T) {
	resolver := NewConnectionUrlResolver("neo4j://localhost")
	if resolver == nil {
		t.Error("Expected resolver not to be nil")
	}
}

func TestToHashWithValidURL(t *testing.T) {
	resolver := NewConnectionUrlResolver("neo4j://user:pass@localhost:7687/testdb")
	config := resolver.ToHash()

	expected := &ConnectionConfig{
		Adapter:  "neo4j",
		Host:     "localhost",
		Port:     7687,
		Username: "user",
		Password: "pass",
		Database: "testdb",
		SSL:      false,
		SSC:      false,
		Options:  map[string]string{},
	}

	if !reflect.DeepEqual(config, expected) {
		t.Errorf("Expected %+v but got %+v", expected, config)
	}
}

func TestToHashWithInvalidURL(t *testing.T) {
	cases := []string{
		"",                          // Empty string
		"invalid",                   // No scheme separator
		"unknown://localhost",       // Unsupported adapter
		"neo4j+invalid://localhost", // Invalid modifier
	}

	for _, c := range cases {
		resolver := NewConnectionUrlResolver(c)
		config := resolver.ToHash()

		if config != nil {
			t.Errorf("Expected nil for invalid URL: %s, got %+v", c, config)
		}
	}
}

func TestSSLAndSSCModifiers(t *testing.T) {
	testCases := []struct {
		url       string
		expectSSL bool
		expectSSC bool
	}{
		{"neo4j://localhost", false, false},
		{"neo4j+ssl://localhost", true, false},
		{"neo4j+ssc://localhost", true, true},
		{"neo4j+s://localhost", true, true},
		{"neo4j+ssl+ssc://localhost", true, true},
	}

	for _, tc := range testCases {
		resolver := NewConnectionUrlResolver(tc.url)
		config := resolver.ToHash()

		if config == nil {
			t.Errorf("Expected config for URL: %s, got nil", tc.url)
			continue
		}

		if config.SSL != tc.expectSSL {
			t.Errorf("URL %s: expected SSL=%t, got %t", tc.url, tc.expectSSL, config.SSL)
		}

		if config.SSC != tc.expectSSC {
			t.Errorf("URL %s: expected SSC=%t, got %t", tc.url, tc.expectSSC, config.SSC)
		}
	}
}

func TestDefaultValues(t *testing.T) {
	// Test default host
	resolver1 := NewConnectionUrlResolver("neo4j://:7687")
	config1 := resolver1.ToHash()
	if config1.Host != "localhost" {
		t.Errorf("Expected default host to be 'localhost', got '%s'", config1.Host)
	}

	if config1.Database != "neo4j" {
		t.Errorf("Expected default database to be 'neo4j', got '%s'", config1.Database)
	}

	// Test default port
	resolver2 := NewConnectionUrlResolver("neo4j://localhost")
	config2 := resolver2.ToHash()
	if config2.Port != DefaultPort {
		t.Errorf("Expected default port to be %d, got %d", DefaultPort, config2.Port)
	}

	if config2.Database != "neo4j" {
		t.Errorf("Expected default database to be 'neo4j', got '%s'", config2.Database)
	}
}

func TestQueryParameters(t *testing.T) {
	url := "neo4j://localhost?timeout=30&poolSize=5"
	resolver := NewConnectionUrlResolver(url)
	config := resolver.ToHash()

	expected := map[string]string{
		"timeout":  "30",
		"poolSize": "5",
	}

	if !reflect.DeepEqual(config.Options, expected) {
		t.Errorf("Expected options %v but got %v", expected, config.Options)
	}
}

func TestURLWithEncodedCredentials(t *testing.T) {
	// URL with encoded credentials
	url := "neo4j://user%40example.com:p%40ssw%3Ard@localhost"
	resolver := NewConnectionUrlResolver(url)
	config := resolver.ToHash()

	if config.Username != "user@example.com" {
		t.Errorf("Expected decoded username 'user@example.com', got '%s'", config.Username)
	}

	if config.Password != "p@ssw:rd" {
		t.Errorf("Expected decoded password 'p@ssw:rd', got '%s'", config.Password)
	}
}

func TestMemgraphAdapter(t *testing.T) {
	url := "memgraph://localhost"
	resolver := NewConnectionUrlResolver(url)
	config := resolver.ToHash()

	if config.Adapter != "memgraph" {
		t.Errorf("Expected adapter 'memgraph', got '%s'", config.Adapter)
	}
}

func TestDatabaseFromPath(t *testing.T) {
	testCases := []struct {
		url          string
		expectDBName string
	}{
		{"neo4j://localhost", "neo4j"},  // No database specified
		{"neo4j://localhost/", "neo4j"}, // Empty path
		{"memgraph://localhost", ""},    // Memgraph default is empty
		{"neo4j://localhost/testdb", "testdb"},
		{"memgraph+ssl://localhost:7687/mycustomdb", "mycustomdb"},
		{"memgraph+ssc://localhost:7687/memgraph", "memgraph"},
	}

	for _, tc := range testCases {
		resolver := NewConnectionUrlResolver(tc.url)
		config := resolver.ToHash()
		if config == nil {
			t.Errorf("Expected config for URL: %s, got nil", tc.url)
			continue
		}
		if config.Database != tc.expectDBName {
			t.Errorf("URL %s: expected Database='%s', got '%s'",
				tc.url, tc.expectDBName, config.Database)
		}
	}
}

func TestSSLConnectionParams(t *testing.T) {
	cases := []struct {
		url        string
		secure     bool
		verifyCert bool
	}{
		{"neo4j://localhost", false, false},
		{"neo4j+ssl://localhost", true, true},
		{"neo4j+s://localhost", true, false},
	}

	for _, c := range cases {
		resolver := NewConnectionUrlResolver(c.url)
		params := resolver.SSLConnectionParams()
		if params["secure"] != c.secure {
			t.Errorf("URL %s: expected secure=%t, got %t", c.url, c.secure, params["secure"])
		}
		if params["verify_cert"] != c.verifyCert {
			t.Errorf("URL %s: expected verify_cert=%t, got %t", c.url, c.verifyCert, params["verify_cert"])
		}
	}
}

func TestSSLConnectionParamsEmpty(t *testing.T) {
	resolver := NewConnectionUrlResolver("")
	params := resolver.SSLConnectionParams()
	if len(params) != 0 {
		t.Errorf("Expected empty map for empty URL, got %v", params)
	}
}

func TestSSLAndSSCBehaviour(t *testing.T) {
	cases := []struct {
		url          string
		expectSecure bool
		expectVerify bool
		expectDB     string
	}{
		{"neo4j://localhost/", false, false, "neo4j"},
		{"neo4j+ssl://localhost/", true, true, "neo4j"},
		{"neo4j+ssc://localhost/", true, false, "neo4j"},
		{"memgraph://localhost/", false, false, ""},
		{"memgraph+ssl://localhost/", true, true, ""},
		{"memgraph+ssc://localhost/", true, false, ""},
	}

	for _, tc := range cases {
		resolver := NewConnectionUrlResolver(tc.url)
		cfg := resolver.ToHash()
		params := resolver.SSLConnectionParams()

		if cfg.Database != tc.expectDB {
			t.Errorf("URL %s: expected database '%s', got '%s'", tc.url, tc.expectDB, cfg.Database)
		}
		if params["secure"] != tc.expectSecure {
			t.Errorf("URL %s: expected secure=%t, got %t", tc.url, tc.expectSecure, params["secure"])
		}
		if params["verify_cert"] != tc.expectVerify {
			t.Errorf("URL %s: expected verify_cert=%t, got %t", tc.url, tc.expectVerify, params["verify_cert"])
		}
	}
}

func TestSAliasEquality(t *testing.T) {
	neoS := NewConnectionUrlResolver("neo4j+s://localhost")
	neoSSC := NewConnectionUrlResolver("neo4j+ssc://localhost")
	if !reflect.DeepEqual(neoS.ToHash(), neoSSC.ToHash()) {
		t.Errorf("neo4j+s and neo4j+ssc should resolve identically")
	}

	mgS := NewConnectionUrlResolver("memgraph+s://localhost")
	mgSSC := NewConnectionUrlResolver("memgraph+ssc://localhost")
	if !reflect.DeepEqual(mgS.ToHash(), mgSSC.ToHash()) {
		t.Errorf("memgraph+s and memgraph+ssc should resolve identically")
	}
}

func TestCompleteConfiguration(t *testing.T) {
	url := "neo4j+ssl://user:pass@example.com:8765/mydb?timeout=60&maxPoolSize=20"
	resolver := NewConnectionUrlResolver(url)
	config := resolver.ToHash()

	expected := &ConnectionConfig{
		Adapter:  "neo4j",
		Host:     "example.com",
		Port:     8765,
		Username: "user",
		Password: "pass",
		Database: "mydb",
		SSL:      true,
		SSC:      false,
		Options: map[string]string{
			"timeout":     "60",
			"maxPoolSize": "20",
		},
	}

	if !reflect.DeepEqual(config, expected) {
		t.Errorf("Complete configuration:\nExpected: %+v\nGot: %+v", expected, config)
	}
}

func TestFullExample(t *testing.T) {
	url := "neo4j://username:password@localhost:7687/database"
	resolver := NewConnectionUrlResolver(url)
	config := resolver.ToHash()

	t.Log("Connection Configuration:")
	t.Logf("Adapter: %s", config.Adapter)
	t.Logf("Host: %s", config.Host)
	t.Logf("Port: %d", config.Port)
	t.Logf("Username: %s", config.Username)
	t.Logf("Password: %s", config.Password)
	t.Logf("Database: %s", config.Database)
	t.Logf("SSL Enabled: %t", config.SSL)
	t.Logf("SSC Enabled: %t", config.SSC)
	t.Log("Options:", config.Options)

	// URL with SSL and query parameters
	sslUrl := "neo4j+ssl://neo4j:s3cr3t@graph.example.com:7687/prod?timeout=30&poolSize=10"
	sslResolver := NewConnectionUrlResolver(sslUrl)
	sslConfig := sslResolver.ToHash()

	t.Log("\nSSL Connection Configuration:")
	t.Logf("Adapter: %s", sslConfig.Adapter)
	t.Logf("Host: %s", sslConfig.Host)
	t.Logf("Port: %d", sslConfig.Port)
	t.Logf("Username: %s", sslConfig.Username)
	t.Logf("Password: %s", sslConfig.Password)
	t.Logf("Database: %s", sslConfig.Database)
	t.Logf("SSL Enabled: %t", sslConfig.SSL)
	t.Logf("SSC Enabled: %t", sslConfig.SSC)
	t.Log("Options:", sslConfig.Options)
}
