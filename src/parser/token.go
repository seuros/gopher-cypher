package parser

import "regexp"

var (
	identifierPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
)

func IsValidIdentifier(s string) bool {
	return identifierPattern.MatchString(s)
}

func IsSafeString(s string) bool {
	return !containsUnsafePatterns(s)
}

func containsUnsafePatterns(s string) bool {
	unsafePatterns := []string{
		"1=1",
		"OR",
		"AND",
		"DROP",
		"DELETE",
		"CREATE",
		"MERGE",
		"SET",
		"REMOVE",
		"FOREACH",
		"CALL",
		"LOAD",
	}

	for _, pattern := range unsafePatterns {
		if regexp.MustCompile(`(?i)` + pattern).MatchString(s) {
			return true
		}
	}

	return false
}
