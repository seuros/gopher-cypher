package cypher

// WithNode represents a WITH clause.
type WithNode struct {
	Items           []interface{}
	Distinct        bool
	WhereConditions []interface{}
}

func (n *WithNode) Accept(v Visitor) error {
	if vv, ok := v.(interface{ VisitWithNode(*WithNode) error }); ok {
		return vv.VisitWithNode(n)
	}
	return nil
}
