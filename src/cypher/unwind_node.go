package cypher

// UnwindNode represents an UNWIND clause.
type UnwindNode struct {
	Expression interface{}
	AliasName  string
}

func (n *UnwindNode) Accept(v Visitor) error {
	if vv, ok := v.(interface{ VisitUnwindNode(*UnwindNode) error }); ok {
		return vv.VisitUnwindNode(n)
	}
	return nil
}

// Type returns the ClauseType for UnwindNode.
func (n *UnwindNode) Type() ClauseType {
	return UnwindClause
}
