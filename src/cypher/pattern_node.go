package cypher

// PatternNode is a base type for pattern-related nodes.
type PatternNode struct{}

func (n *PatternNode) Accept(v Visitor) error {
	if vv, ok := v.(interface{ VisitPatternNode(*PatternNode) error }); ok {
		return vv.VisitPatternNode(n)
	}
	return nil
}
