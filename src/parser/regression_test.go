package parser

import (
	"testing"
	"github.com/stretchr/testify/require"
)

// TestRegressionProtection ensures that parser coverage doesn't regress
// by testing all currently working fixtures that comprise our 40% coverage.
func TestRegressionProtection(t *testing.T) {
	parser, err := New()
	require.NoError(t, err)

	// These are the 6 fixtures that MUST continue to work (our 40% coverage)
	workingFixtures := []struct {
		name  string
		query string
	}{
		{
			name:  "Simple variable return",
			query: "RETURN n",
		},
		{
			name:  "Function call with no arguments", 
			query: "RETURN count()",
		},
		{
			name:  "Property access return",
			query: "RETURN n.name",
		},
		{
			name:  "Math expression with parameters",
			query: "RETURN $a + $b",
		},
		{
			name:  "Basic match with label",
			query: "MATCH (n:Person) RETURN n",
		},
		{
			name:  "Match with property filter",
			query: "MATCH (n:Person) WHERE n.age = 25 RETURN n",
		},
	}

	for _, fixture := range workingFixtures {
		t.Run(fixture.name, func(t *testing.T) {
			// This test MUST pass - if it fails, we have a regression
			query, err := parser.Parse(fixture.query)
			require.NoError(t, err, "Regression detected: %s should parse successfully", fixture.name)
			require.NotNil(t, query, "Parsed query should not be nil")
			
			// Verify the query can be built back to Cypher
			cypherOutput, _ := query.BuildCypher()
			require.NotEmpty(t, cypherOutput, "Query should generate non-empty Cypher output")
		})
	}
}

// TestKnownFailures documents the 9 fixtures that should fail (our 60% missing coverage)
// These tests should pass when they fail - if any start passing, we've improved coverage!
func TestKnownFailures(t *testing.T) {
	parser, err := New()
	require.NoError(t, err)

	knownFailures := []struct {
		name  string
		query string
		reason string
	}{
		{
			name:   "Basic return of parameters",
			query:  "RETURN $p1, $p2", 
			reason: "Multiple parameter returns not supported yet",
		},
		{
			name:   "Function with parameters",
			query:  "RETURN substring($text, 0, 5)",
			reason: "Functions with parameters not supported yet",
		},
		{
			name:   "Multiple expressions",
			query:  "RETURN n.name, n.age",
			reason: "Multiple return expressions not supported yet",
		},
		{
			name:   "Complex match pattern",
			query:  "MATCH (a)-[:KNOWS]->(b) RETURN a, b",
			reason: "Relationship patterns not supported yet",
		},
		{
			name:   "Optional match",
			query:  "OPTIONAL MATCH (n:Person) RETURN n",
			reason: "Optional match not fully implemented",
		},
		{
			name:   "Set multiple properties",
			query:  "MATCH (n) SET n.name = $name, n.age = $age",
			reason: "Multiple SET assignments not supported yet",
		},
		{
			name:   "Unwind complex list",
			query:  "UNWIND [$a, $b, $c] AS item RETURN item",
			reason: "Complex list unwinding not supported yet",
		},
		{
			name:   "Limit with parameter",
			query:  "MATCH (n) RETURN n LIMIT $limit",
			reason: "Parameterized LIMIT not implemented",
		},
		{
			name:   "Order by clause",
			query:  "MATCH (n) RETURN n ORDER BY n.name",
			reason: "ORDER BY clause not implemented yet",
		},
	}

	for _, fixture := range knownFailures {
		t.Run(fixture.name, func(t *testing.T) {
			// These SHOULD fail - if they pass, we've improved coverage!
			_, err := parser.Parse(fixture.query)
			if err == nil {
				t.Logf("COVERAGE IMPROVEMENT: %s now parses successfully! Update regression tests.", fixture.name)
				// Note: We don't fail the test here - this is good news!
			} else {
				t.Logf("Expected failure: %s - %s", fixture.reason, err.Error())
			}
		})
	}
}

// TestSecurityValidation ensures our security measures remain intact
func TestSecurityValidation(t *testing.T) {
	parser, err := New()
	require.NoError(t, err)

	securityTests := []struct {
		name  string
		query string
		expectedError string
	}{
		{
			name:          "Multiple statements blocked",
			query:         "RETURN 1; DROP DATABASE;",
			expectedError: "multiple statements not allowed",
		},
		{
			name:          "Single quotes blocked", 
			query:         "RETURN 'malicious'",
			expectedError: "single quotes not allowed",
		},
	}

	for _, test := range securityTests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parser.Parse(test.query)
			require.Error(t, err, "Security violation should be caught")
			require.Contains(t, err.Error(), test.expectedError)
		})
	}
}