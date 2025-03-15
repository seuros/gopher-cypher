package optimized

// SkipData is an optimized representation of a SKIP clause amount.
type SkipData struct {
	Expression any
}

// Accept satisfies the Node interface for SkipData.
func (n *SkipData) Accept(v Visitor) error {
	if vv, ok := v.(interface{ VisitSkipData(*SkipData) error }); ok {
		return vv.VisitSkipData(n)
	}
	return nil
}
