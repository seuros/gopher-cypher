package cypher

// LoadCSVNode represents a LOAD CSV clause.
type LoadCSVNode struct {
	WithHeaders bool
	From        interface{}
	As          string
}

func (n *LoadCSVNode) Accept(v Visitor) error {
	if vv, ok := v.(interface{ VisitLoadCSVNode(*LoadCSVNode) error }); ok {
		return vv.VisitLoadCSVNode(n)
	}
	return nil
}
