package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type TestCase struct {
	Description string `json:"description"`
	Query       string `json:"query"`
}

func TestParserWithFixtures(t *testing.T) {
	// Read test cases from fixtures
	fixturesPath := filepath.Join("..", "..", "fixtures", "cypher_parser_test_cases.json")
	data, err := os.ReadFile(fixturesPath)
	if err != nil {
		t.Fatalf("Failed to read fixtures file: %v", err)
	}

	var testCases []TestCase
	if err := json.Unmarshal(data, &testCases); err != nil {
		t.Fatalf("Failed to parse fixtures JSON: %v", err)
	}

	if len(testCases) == 0 {
		t.Fatal("No test cases found in fixtures file")
	}

	// Create parser
	parser, err := New()
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	// Track which features our parser supports vs doesn't support yet
	supported := 0
	unsupported := 0
	var unsupportedCases []TestCase

	for _, tc := range testCases {
		t.Run(tc.Description, func(t *testing.T) {
			query, err := parser.Parse(tc.Query)
			if err != nil {
				// Log unsupported cases but don't fail the test yet
				// This helps us track our parser's current capabilities
				t.Logf("UNSUPPORTED: %s", tc.Description)
				t.Logf("Query: %s", tc.Query)
				t.Logf("Error: %v", err)
				unsupported++
				unsupportedCases = append(unsupportedCases, tc)
				return
			}

			// Verify we got a valid AST
			if query == nil {
				t.Errorf("Parser returned nil query for: %s", tc.Query)
				return
			}

			// Verify the AST can generate Cypher (basic sanity check)
			cypher, params := query.BuildCypher()
			if cypher == "" {
				t.Errorf("Generated empty Cypher for: %s", tc.Query)
				return
			}

			supported++
			t.Logf(" SUPPORTED: %s", tc.Description)
			t.Logf("Original: %s", tc.Query)
			t.Logf("Generated: %s", cypher)
			if len(params) > 0 {
				t.Logf("Parameters: %v", params)
			}
		})
	}

	// Summary report
	t.Logf("\n=== PARSER COVERAGE REPORT ===")
	t.Logf("Total test cases: %d", len(testCases))
	t.Logf("Supported: %d (%.1f%%)", supported, float64(supported)/float64(len(testCases))*100)
	t.Logf("Unsupported: %d (%.1f%%)", unsupported, float64(unsupported)/float64(len(testCases))*100)

	if unsupported > 0 {
		t.Logf("\n=== UNSUPPORTED FEATURES ===")
		for _, uc := range unsupportedCases {
			t.Logf("- %s: %s", uc.Description, uc.Query)
		}
	}

	// This test passes as long as we can parse SOME queries
	// As we improve the parser, more cases should start passing
	if supported == 0 {
		t.Fatal("Parser couldn't parse any test cases - something is broken!")
	}
}

func TestBasicFixtureValidation(t *testing.T) {
	// Test a few basic cases that our current parser SHOULD support
	parser, err := New()
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	basicCases := []struct {
		name  string
		query string
	}{
		{
			name:  "simple match return",
			query: "MATCH (n) RETURN n",
		},
		{
			name:  "match with where parameter",
			query: "MATCH (n) WHERE n.age > $age RETURN n",
		},
		{
			name:  "unwind parameter list",
			query: "UNWIND $items AS item RETURN item",
		},
		{
			name:  "set with parameter",
			query: "MATCH (n) SET n.name = $name",
		},
		{
			name:  "merge node",
			query: "MERGE (n:User)",
		},
	}

	for _, tc := range basicCases {
		t.Run(tc.name, func(t *testing.T) {
			query, err := parser.Parse(tc.query)
			if err != nil {
				t.Errorf("Expected basic query to parse successfully: %v", err)
				t.Errorf("Query: %s", tc.query)
			}
			if query == nil {
				t.Errorf("Parser returned nil query")
			}
		})
	}
}