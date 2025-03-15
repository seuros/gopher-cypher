package cypher

// SkipNode represents a SKIP clause.
type SkipNode struct {
	Amount interface{}
}

func (n *SkipNode) Accept(v Visitor) error {
	if vv, ok := v.(interface{ VisitSkipNode(*SkipNode) error }); ok {
		return vv.VisitSkipNode(n)
	}
	return nil
}

// Type returns the ClauseType for SkipNode.
func (n *SkipNode) Type() ClauseType {
	return SkipClause
}
