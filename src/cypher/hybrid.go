package cypher

// HybridApproach contains helper constructors mirroring the Ruby version.
type HybridApproach struct{}

// CreateLiteral constructs an optimized LiteralData node.
func (HybridApproach) CreateLiteral(v interface{}) LiteralData {
	return LiteralData{Value: v}
}
