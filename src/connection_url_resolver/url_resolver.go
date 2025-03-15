package connection_url_resolver

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Constants for supported adapters and default port
var (
	SupportedAdapters = []string{"neo4j", "memgraph"}
	DefaultPort       = 7687
)

// ConnectionConfig represents the normalized configuration for Cypher-based database connections
type ConnectionConfig struct {
	Adapter  string
	Username string
	Password string
	Host     string
	Port     int
	Database string
	SSL      bool
	SSC      bool
	Options  map[string]string
}

// ConnectionUrlResolver accepts Cypher-based database URLs and converts them
// into a normalized configuration for adapter resolution.
//
// Supported URL prefixes:
// - neo4j://
// - neo4j+ssl://
// - neo4j+ssc://
// - memgraph://
// - memgraph+ssl://
// - memgraph+ssc://
type ConnectionUrlResolver struct {
	urlString string
	parsed    *ConnectionConfig
}

// NewConnectionUrlResolver initializes a new resolver with a URL string
func NewConnectionUrlResolver(urlString string) *ConnectionUrlResolver {
	resolver := &ConnectionUrlResolver{
		urlString: urlString,
	}
	resolver.parsed = resolver.parseURL(urlString)
	return resolver
}

// ToHash converts the URL to a normalized configuration
func (r *ConnectionUrlResolver) ToHash() *ConnectionConfig {
	return r.parsed
}

func (r *ConnectionUrlResolver) Address() string {
	return fmt.Sprintf("%s:%d", r.parsed.Host, r.parsed.Port)
}

// SSLConnectionParams mirrors Ruby's ssl_connection_params and returns
// connection security options.
// The returned map contains:
//   - secure:      true if SSL/TLS should be used
//   - verify_cert: true if the server certificate should be verified
func (r *ConnectionUrlResolver) SSLConnectionParams() map[string]bool {
	if r.parsed == nil {
		return map[string]bool{}
	}

	secure := r.parsed.SSL || r.parsed.SSC
	verify := secure && !r.parsed.SSC

	return map[string]bool{
		"secure":      secure,
		"verify_cert": verify,
	}
}

// parseURL parses the URL string into a ConnectionConfig structure
func (r *ConnectionUrlResolver) parseURL(urlString string) *ConnectionConfig {
	if urlString == "" {
		return nil
	}

	// Extract scheme and potential modifiers (ssl, ssc)
	schemeParts := strings.SplitN(urlString, "://", 2)
	if len(schemeParts) != 2 {
		return nil
	}

	scheme := schemeParts[0]
	rest := schemeParts[1]

	adapter, modifiers, valid := r.extractAdapterAndModifiers(scheme)
	if !valid {
		return nil
	}

	// Parse the remaining part as a standard URI
	uriString := fmt.Sprintf("%s://%s", adapter, rest)
	uri, err := url.Parse(uriString)
	if err != nil {
		return nil
	}

	// Extract query parameters
	options := make(map[string]string)
	if uri.RawQuery != "" {
		for key, values := range uri.Query() {
			if len(values) > 0 && key != "" && values[0] != "" {
				options[key] = values[0]
			}
		}
	}

	// Extract database from path. By default the database name matches the
	// adapter except for Memgraph which doesn't use a default database.
	var database string
	if uri.Path != "" && uri.Path != "/" {
		database = strings.TrimPrefix(uri.Path, "/")
		if database == "" {
			if adapter != "memgraph" {
				database = adapter
			}
		}
	} else if adapter != "memgraph" {
		database = adapter
	}

	// Extract username and password
	var username, password string
	if uri.User != nil {
		username = uri.User.Username()
		if username == "" {
			username = ""
		}
		pass, hasPass := uri.User.Password()
		if hasPass {
			password = pass
		}
	}

	// Set up host and port
	host := uri.Hostname()
	if host == "" {
		host = "localhost"
	}

	port := DefaultPort
	if uri.Port() != "" {
		portNum, err := strconv.Atoi(uri.Port())
		if err == nil {
			port = portNum
		}
	}

	// When using SSC (self-signed certificates), SSL must also be enabled
	useSSL := contains(modifiers, "ssl")
	useSSC := contains(modifiers, "ssc")

	// Self-signed certificates imply SSL is also enabled
	if useSSC {
		useSSL = true
	}

	return &ConnectionConfig{
		Adapter:  adapter,
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		Database: database,
		SSL:      useSSL,
		SSC:      useSSC,
		Options:  options,
	}
}

// extractAdapterAndModifiers extracts the adapter name and modifiers from the scheme
func (r *ConnectionUrlResolver) extractAdapterAndModifiers(scheme string) (string, []string, bool) {
	parts := strings.Split(scheme, "+")
	if len(parts) == 0 {
		return "", nil, false
	}

	adapter := parts[0]
	if !contains(SupportedAdapters, adapter) {
		return "", nil, false
	}

	var modifiers []string
	for i := 1; i < len(parts); i++ {
		m := parts[i]
		switch m {
		case "ssl":
			modifiers = append(modifiers, "ssl")
		case "ssc", "s":
			modifiers = append(modifiers, "ssc")
		default:
			// If there are parts that are neither the adapter nor valid modifiers, the URL is invalid
			return "", nil, false
		}
	}

	return adapter, modifiers, true
}

// contains checks if a string slice contains a specific element
func contains(slice []string, element string) bool {
	for _, e := range slice {
		if e == element {
			return true
		}
	}
	return false
}
