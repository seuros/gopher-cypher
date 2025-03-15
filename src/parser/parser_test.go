package parser

import (
	"testing"
)

func TestBasicParsing(t *testing.T) {
	parser, err := New()
	if err != nil {
		t.Fatalf("failed to create parser: %v", err)
	}

	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{
			name:  "simple match return",
			input: `MATCH (n:User) RETURN n.name`,
			valid: true,
		},
		{
			name:  "match with where",
			input: `MATCH (n:User) WHERE n.age > 30 RETURN n.name`,
			valid: true,
		},
		{
			name:  "with limit",
			input: `MATCH (n:User) RETURN n.name LIMIT 10`,
			valid: true,
		},
		{
			name:  "multiple statements blocked",
			input: `MATCH (n:User) RETURN n.name; DROP DATABASE`,
			valid: false,
		},
		{
			name:  "single quotes blocked",
			input: `MATCH (n:User) WHERE n.name = 'admin' RETURN n.name`,
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.Parse(tt.input)
			if tt.valid && err != nil {
				t.Errorf("expected valid query to parse, got error: %v", err)
			}
			if !tt.valid && err == nil {
				t.Errorf("expected invalid query to fail parsing")
			}
		})
	}
}