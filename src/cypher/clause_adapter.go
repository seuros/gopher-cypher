package cypher

import (
	"fmt"
	"reflect"
)

// ClauseAdapter bridges AST nodes with the Clause interface used by Query.
type ClauseAdapter struct {
	Node Node
	key  uintptr
}

// NewClauseAdapter constructs a ClauseAdapter for a given node.
func NewClauseAdapter(n Node) *ClauseAdapter {
	return &ClauseAdapter{Node: n, key: reflect.ValueOf(n).Pointer()}
}

// BuildCypher compiles the AST node, using a cache to reuse results.
func (c *ClauseAdapter) BuildCypher(q *Query) string {
	cacheKey := fmt.Sprintf("%d:%T", c.key, c.Node)
	return simpleCache.Fetch(cacheKey, func() string {
		compiler := NewQueryIntegratedCompiler(q)
		compiler.Compile(c.Node)
		return compiler.Output()
	})
}

// Type returns the ClauseType of the underlying Node.
func (c *ClauseAdapter) Type() ClauseType {
	// Type switch on the actual node type
	switch c.Node.(type) {
	case *MatchNode:
		return MatchClause
	case *MergeNode:
		return MergeClause
	case *UnwindNode:
		return UnwindClause
	case *WhereNode:
		return WhereClause
	case *SetNode:
		return SetClause
	case *RemoveNode:
		return RemoveClause
	case *ReturnNode:
		return ReturnClause
	case *SkipNode:
		return SkipClause
	case *LimitNode:
		return LimitClause
	// Add cases for other node types as they are defined
	// e.g., CallNode, DeleteNode, WithNode etc.
	default:
		// Unknown node type - return UnknownClauseType silently
		// Callers should handle this case appropriately
		return UnknownClauseType
	}
}
