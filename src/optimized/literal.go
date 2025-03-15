package optimized

// LiteralData is a lightweight representation of a literal value.
type LiteralData struct {
	Value any
}

// Accept satisfies the Node interface for LiteralData.
func (n *LiteralData) Accept(v Visitor) error {
	if vv, ok := v.(interface{ VisitLiteralData(*LiteralData) error }); ok {
		return vv.VisitLiteralData(n)
	}
	return nil
}
