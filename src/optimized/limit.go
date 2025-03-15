package optimized

// LimitData is an optimized representation of a LIMIT clause amount.
type LimitData struct {
	Expression any
}

// Accept satisfies the Node interface for LimitData.
func (n *LimitData) Accept(v Visitor) error {
	if vv, ok := v.(interface{ VisitLimitData(*LimitData) error }); ok {
		return vv.VisitLimitData(n)
	}
	return nil
}
