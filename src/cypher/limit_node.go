package cypher

// LimitNode represents a LIMIT clause.
type LimitNode struct {
	Expression interface{}
}

func (n *LimitNode) Accept(v Visitor) error {
	if vv, ok := v.(interface{ VisitLimitNode(*LimitNode) error }); ok {
		return vv.VisitLimitNode(n)
	}
	return nil
}

// Type returns the ClauseType for LimitNode.
func (n *LimitNode) Type() ClauseType {
	return LimitClause
}
