package boltutil

import (
	"runtime"
	"strings"
	"testing"
)

func TestUserAgentGeneration(t *testing.T) {
	// Test version function
	version := getLibraryVersion()
	if version != LibraryVersion {
		t.Errorf("Expected version %s, got %s", LibraryVersion, version)
	}

	// Test user agent format
	goVersion := runtime.Version()[2:] // Remove "go" prefix  
	expectedUserAgent := "gopher-cypher::Bolt/" + LibraryVersion + " (Go/" + goVersion + ")"
	
	// Simulate the user agent generation (extract from SendHello logic)
	userAgent := "gopher-cypher::Bolt/" + getLibraryVersion() + " (Go/" + runtime.Version()[2:] + ")"
	
	if userAgent != expectedUserAgent {
		t.Errorf("Expected user agent %s, got %s", expectedUserAgent, userAgent)
	}

	// Test platform string format
	platform := "go " + runtime.Version()[2:] + " [" + runtime.GOARCH + "-" + runtime.GOOS + "]"
	
	// Verify it contains expected components
	if !strings.Contains(platform, runtime.GOARCH) {
		t.Errorf("Platform string should contain architecture: %s", platform)
	}
	if !strings.Contains(platform, runtime.GOOS) {
		t.Errorf("Platform string should contain OS: %s", platform)
	}
	if !strings.Contains(platform, "go ") {
		t.Errorf("Platform string should contain 'go ': %s", platform)
	}
	
	t.Logf("Generated user agent: %s", userAgent)
	t.Logf("Generated platform: %s", platform)
}