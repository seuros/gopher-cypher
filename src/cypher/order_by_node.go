package cypher

// OrderByItem represents a single ORDER BY specification.
type OrderByItem struct {
	Expression interface{}
	Direction  string
}

// OrderByNode represents an ORDER BY clause.
type OrderByNode struct {
	Items []OrderByItem
}

func (n *OrderByNode) Accept(v Visitor) error {
	if vv, ok := v.(interface{ VisitOrderByNode(*OrderByNode) error }); ok {
		return vv.VisitOrderByNode(n)
	}
	return nil
}
