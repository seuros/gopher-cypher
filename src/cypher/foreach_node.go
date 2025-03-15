package cypher

// ForeachNode represents a FOREACH clause.
type ForeachNode struct {
	Variable      string
	Expression    interface{}
	UpdateClauses []Node
}

func (n *ForeachNode) Accept(v Visitor) error {
	if vv, ok := v.(interface{ VisitForeachNode(*ForeachNode) error }); ok {
		return vv.VisitForeachNode(n)
	}
	return nil
}
