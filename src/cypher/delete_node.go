package cypher

// DeleteNode represents a DELETE or DETACH DELETE clause.
type DeleteNode struct {
	Expressions []interface{}
	Detach      bool
}

func (n *DeleteNode) Accept(v Visitor) error {
	if vv, ok := v.(interface{ VisitDeleteNode(*DeleteNode) error }); ok {
		return vv.VisitDeleteNode(n)
	}
	return nil
}
