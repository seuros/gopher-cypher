package cypher

// WhereNode represents a WHERE clause with one or more conditions.
type WhereNode struct {
	Conditions []Expression
}

func (n *WhereNode) Accept(v Visitor) error {
	if vv, ok := v.(interface{ VisitWhereNode(*WhereNode) error }); ok {
		return vv.VisitWhereNode(n)
	}
	return nil
}

// Type returns the ClauseType for WhereNode.
func (n *WhereNode) Type() ClauseType {
	return WhereClause
}
