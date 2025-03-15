package cypher

// RemoveItem represents a REMOVE target.
type RemoveItem interface{}

type PropertyRemoval struct {
	Property string
}

type LabelRemoval struct {
	Variable string
	Label    string
}

// RemoveNode represents a REMOVE clause.
type RemoveNode struct {
	Items []RemoveItem
}

func (n *RemoveNode) Accept(v Visitor) error {
	if vv, ok := v.(interface{ VisitRemoveNode(*RemoveNode) error }); ok {
		return vv.VisitRemoveNode(n)
	}
	return nil
}

// Type returns the ClauseType for RemoveNode.
func (n *RemoveNode) Type() ClauseType {
	return RemoveClause
}
