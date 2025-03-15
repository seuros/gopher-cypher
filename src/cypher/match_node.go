package cypher

// MatchNode represents a MATCH clause.
type MatchNode struct {
	Pattern interface{}
}

func (n *MatchNode) Accept(v Visitor) error {
	if vv, ok := v.(interface{ VisitMatchNode(*MatchNode) error }); ok {
		return vv.VisitMatchNode(n)
	}
	return nil
}

// Type returns the ClauseType for MatchNode.
func (n *MatchNode) Type() ClauseType {
	return MatchClause
}
