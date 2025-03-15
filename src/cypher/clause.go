package cypher

// ClauseType defines the type of a Cypher clause.
type ClauseType int

// Enum for ClauseType
const (
	UnknownClauseType ClauseType = iota
	CallClause                   // Not in current grammar.go but for completeness
	DeleteClause                 // Not in current grammar.go but for completeness
	ForeachClause                // Not in current grammar.go but for completeness
	LoadCSVClause                // Not in current grammar.go but for completeness
	MatchClause
	MergeClause
	RemoveClause
	ReturnClause
	SetClause
	SkipClause
	LimitClause
	UnwindClause
	WhereClause
	WithClause // Not in current grammar.go but for completeness
	// Add other clause types as they are implemented
)

// Clause represents a single part of a Cypher query.
// Implementations generate the query snippet and update the provided Query.
type Clause interface {
	// BuildCypher returns the Cypher representation of this clause
	// while appending parameters to the Query.
	BuildCypher(q *Query) string
	// Type returns the specific type of the clause.
	Type() ClauseType
}

// ClauseOrder determines the sorting order of clauses.
// The order is based on common Cypher query structure.
func ClauseOrder(c Clause) int {
	switch c.Type() {
	// Order based on typical Cypher query structure:
	// MATCH, MERGE, UNWIND traditionally appear early.
	// WHERE filters these.
	// SET, REMOVE modify data.
	// RETURN projects results.
	// SKIP, LIMIT paginate results.
	// Other clauses like CALL, CREATE, DELETE, WITH, etc., would fit specific spots.
	case MatchClause:
		return 5
	case MergeClause:
		return 6
	case UnwindClause:
		return 7
	case WhereClause: // Often follows MATCH/MERGE/UNWIND
		return 11
	case SetClause:
		return 20
	case RemoveClause:
		return 21
	case ReturnClause:
		return 37
	case SkipClause:
		return 43
	case LimitClause:
		return 47
	// Placeholder for clauses not yet in grammar.go but part of general Cypher
	case CallClause:
		return 1
	case DeleteClause:
		return 25 // Example: after SET/REMOVE, before WITH/RETURN
	case WithClause:
		return 30 // Example: can be used to pipe results before RETURN
	// Ensure all types from grammar.go's Clause struct are covered here as they get implemented.
	default:
		return 99 // Unknown/other clauses go last
	}
}
