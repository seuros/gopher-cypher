package cypher

// ReturnNode represents a RETURN clause.
type ReturnNode struct {
	Items    []interface{}
	Distinct bool
}

func (n *ReturnNode) Accept(v Visitor) error {
	if vv, ok := v.(interface{ VisitReturnNode(*ReturnNode) error }); ok {
		return vv.VisitReturnNode(n)
	}
	return nil
}

// Type returns the ClauseType for ReturnNode.
func (n *ReturnNode) Type() ClauseType {
	return ReturnClause
}
