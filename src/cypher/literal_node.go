package cypher

// LiteralNode represents a literal value in the AST.
type LiteralNode struct {
	Value interface{}
}

// Accept satisfies the Node interface.
func (n *LiteralNode) Accept(v Visitor) error {
	if vv, ok := v.(interface{ VisitLiteralNode(*LiteralNode) error }); ok {
		return vv.VisitLiteralNode(n)
	}
	return nil
}

// LiteralData is a lightweight variant used in optimized paths.
type LiteralData struct {
	Value interface{}
}

// Accept satisfies the Node interface for LiteralData.
func (n *LiteralData) Accept(v Visitor) error {
	if vv, ok := v.(interface{ VisitLiteralData(*LiteralData) error }); ok {
		return vv.VisitLiteralData(n)
	}
	return nil
}
