package driver

import (
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	version := Version()
	if version == "" {
		t.Error("Version should not be empty")
	}
	
	// Version should be in semantic version format (roughly), except for dev builds
	if version != "dev" && !strings.Contains(version, ".") {
		t.Errorf("Version should contain dots for semantic versioning: %s", version)
	}
	
	t.Logf("Driver version: %s", version)
}

func TestUserAgent(t *testing.T) {
	userAgent := UserAgent()
	if userAgent == "" {
		t.Error("UserAgent should not be empty")
	}
	
	// Should contain the product name
	if !strings.Contains(userAgent, "gopher-cypher") {
		t.Errorf("UserAgent should contain product name: %s", userAgent)
	}
	
	// Should contain the version
	if !strings.Contains(userAgent, Version()) {
		t.Errorf("UserAgent should contain version: %s", userAgent)
	}
	
	t.Logf("User agent: %s", userAgent)
}